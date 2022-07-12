package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/version"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/cli"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/server"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Executor struct {
	v       *viper.Viper
	rootCmd *cobra.Command
	env     *cli.Environment
	loader  *plugins.Loader
	log     logger.Logger

	srv *server.Server

	cfg                 *config.Project
	secrets             map[string]interface{}
	lastUpdateCheckFile string

	opts struct {
		env       string
		valueOpts *values.Options
	}
}

func NewExecutor() *Executor {
	v := viper.New()

	e := &Executor{
		v:       v,
		env:     cli.NewEnvironment(v),
		log:     logger.NewLogger(),
		secrets: make(map[string]interface{}),
	}

	e.opts.valueOpts = &values.Options{}

	setupEnvVars(e.env)

	e.rootCmd = e.newRoot()

	return e
}

func setupEnvVars(env *cli.Environment) {
	env.AddVarWithDefault("plugins_cache_dir", "plugins cache directory", clipath.DataDir("plugin-cache"))
	env.AddVar("no_color", "disable color output")
	env.AddVarWithDefault("log_level", "set logging level (options: debug, notice, info, warn, error)", "info")
}

func (e *Executor) commandPreRun(ctx context.Context) error {
	var (
		loadProjectOptions LoadProjectOptions
		loadAppsMode       config.LoadMode
	)

	e.env.SetPFlags()

	// Parse critical root flags.
	e.rootCmd.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true

	err := e.rootCmd.PersistentFlags().Parse(os.Args[1:])
	if err != nil {
		return err
	}

	helpFlag := e.rootCmd.PersistentFlags().Lookup("help")
	isHelp := helpFlag.Changed || (len(os.Args) > 1 && strings.EqualFold(os.Args[1], "help"))

	// Load values.
	pwd, err := os.Getwd()
	if err != nil {
		return merry.Errorf("cannot find current directory: %w", err)
	}

	pwd, err = filepath.EvalSymlinks(pwd)
	if err != nil {
		return merry.Errorf("cannot evaluate current directory: %w", err)
	}

	cfgPath := fileutil.FindYAMLGoingUp(pwd, config.ProjectYAMLName)

	for i, v := range e.opts.valueOpts.ValueFiles {
		e.opts.valueOpts.ValueFiles[i] = strings.ReplaceAll(v, "<env>", e.opts.env)
	}

	v, valuesLoadErr := e.opts.valueOpts.MergeValues(ctx, filepath.Dir(cfgPath), getter.All())

	vals := map[string]interface{}{
		"var":     v,
		"env":     e.opts.env,
		"secrets": e.secrets,
	}

	// Load essential config file first.
	configPreloadErr := e.loadProject(ctx, cfgPath, e.srv.Addr().String(), vals, LoadProjectOptions{
		Mode: config.LoadModeEssential,
	}, config.LoadModeSkip)

	// Augment/load new commands.
	err = e.addPluginsCommands()
	if err != nil {
		return err
	}

	cmd, _, err := e.rootCmd.Find(os.Args[1:])
	if err != nil {
		return err
	}

	loadProjectOptions.Mode = loadModeFromAnnotation(cmd.Annotations[cmdProjectLoadModeAnnotation])
	loadProjectOptions.SkipCheck = cmd.Annotations[cmdProjectSkipCheckAnnotation] == "1"
	loadProjectOptions.SkipLoadPlugins = cmd.Annotations[cmdProjectSkipLoadPluginsAnnotation] == "1"

	switch {
	case loadProjectOptions.Mode == config.LoadModeSkip:
		return nil
	case valuesLoadErr != nil:
		return valuesLoadErr
	case configPreloadErr != nil:
		return configPreloadErr
	case isHelp:
		return nil
	}

	loadAppsMode = loadModeFromAnnotation(cmd.Annotations[cmdAppsLoadModeAnnotation])

	if loadProjectOptions.Mode == config.LoadModeSkip {
		return nil
	}

	// Load secrets if needed.
	if cmd.Annotations[cmdSecretsLoadAnnotation] == "1" && e.cfg.Secrets.Plugin() != nil {
		e.log.Debugf("Loading secrets from plugin: %s\n", e.cfg.Secrets.Plugin().Name)

		secrets, err := e.cfg.Secrets.Plugin().Client().GetSecrets(ctx, e.cfg.Secrets.Type, e.cfg.Secrets.Other)
		if err != nil {
			return err
		}

		for k, v := range secrets {
			e.secrets[k] = v
		}
	}

	// Load config file properly now.
	loadProjectOptions.SkipLoadPlugins = true

	if err := e.loadProject(ctx, cfgPath, e.srv.Addr().String(), vals, loadProjectOptions, loadAppsMode); err != nil {
		return err
	}

	return nil
}

func (e *Executor) addPluginsCommands() error {
	if e.cfg == nil {
		return nil
	}

	for _, plug := range e.cfg.Plugins {
		plug := plug

		for cmdName, cmdt := range plug.Loaded().Commands {
			cmdName := cmdName
			cmdt := cmdt

			cmdName = strings.ToLower(cmdName)

			cmd, _, err := e.rootCmd.Find([]string{cmdName})
			if err != nil {
				cmd = &cobra.Command{
					Use:          fmt.Sprintf("%s-%s", plug.Loaded().ShortName(), cmdName),
					Short:        cmdt.Short,
					Long:         cmdt.Long,
					SilenceUsage: true,
					Annotations: map[string]string{
						cmdGroupAnnotation:           cmdGroupPlugin,
						cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
						cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
						cmdSecretsLoadAnnotation:     "1",
					},
					RunE: func(cmd *cobra.Command, args []string) error {
						return actions.NewCommand(e.log, e.cfg, &actions.CommandOptions{
							Name:       cmdName,
							InputTypes: cmdt.InputTypes(),
							Plugin:     plug.Loaded(),
							Args:       cmdt.Proto(args),
						}).Run(cmd.Context())
					},
				}

				e.rootCmd.AddCommand(cmd)
			}

			flags := cmd.Flags()

			for _, f := range cmdt.Flags {
				f.Name = strings.ToLower(f.Name)

				if flags.Lookup(f.Name) != nil {
					return merry.Errorf("plugin tried to add already existing argument '%s' to command '%s'", f, cmdName)
				}

				switch f.ValueType() {
				case plugins.CommandValueTypeBool:
					def, _ := f.Default.(bool)
					f.Value = flags.BoolP(f.Name, f.Short, def, f.Usage)
				case plugins.CommandValueTypeInt:
					def, _ := f.Default.(int)
					f.Value = flags.IntP(f.Name, f.Short, def, f.Usage)
				case plugins.CommandValueTypeString:
					def, _ := f.Default.(string)
					f.Value = flags.StringP(f.Name, f.Short, def, f.Usage)
				case plugins.CommandValueTypeStringArray:
					def, _ := f.Default.([]interface{})
					defStr := make([]string, len(def))

					for i, v := range def {
						defStr[i] = v.(string)
					}

					f.Value = flags.StringArrayP(f.Name, f.Short, defStr, f.Usage)
				}

				if f.Required {
					_ = cmd.MarkFlagRequired(f.Name)
				}
			}
		}
	}

	return nil
}

func (e *Executor) startHostServer() error {
	e.srv = server.NewServer(e.log, e.secrets)

	return e.srv.Serve()
}

func (e *Executor) Execute(ctx context.Context) error {
	if err := e.initConfig(); err != nil {
		return err
	}

	if err := e.setupLogging(); err != nil {
		return err
	}

	if err := e.startHostServer(); err != nil {
		return err
	}

	if err := e.commandPreRun(ctx); err != nil {
		return err
	}

	err := e.rootCmd.ExecuteContext(ctx)
	if err != nil {
		_ = e.cleanupProject()
		return err
	}

	return e.cleanupProject()
}

func (e *Executor) setupLogging() error {
	l := e.LogLevel()

	if err := e.log.SetLevelString(l); err != nil {
		return err
	}

	color := !e.NoColor()
	if !color {
		pterm.DisableColor()
	} else {
		pterm.EnableColor()
	}

	yaml.SetDefaultIncludeSource(true)
	yaml.SetDefaultColorize(color)

	return nil
}

func (e *Executor) initConfig() error {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	err = fileutil.MkdirAll(clipath.ConfigDir(), 0o755)
	if err != nil {
		return err
	}

	e.v.AddConfigPath(dir)
	e.v.AddConfigPath(clipath.ConfigDir())
	e.v.SetConfigType("yaml")
	e.v.SetConfigName(".outblocks")

	e.v.SetEnvPrefix(cli.EnvPrefix)
	e.v.AutomaticEnv()

	// If a config file is found, read it in.
	if err := e.v.ReadInConfig(); err == nil {
		e.log.Infoln("Using config file:", e.v.ConfigFileUsed())
	}

	e.lastUpdateCheckFile = clipath.ConfigDir(version.LastUpdateCheckFileName)

	return nil
}

func (e *Executor) Log() logger.Logger {
	return e.log
}

func (e *Executor) PluginsCacheDir() string {
	return e.v.GetString("plugins_cache_dir")
}

func (e *Executor) LogLevel() string {
	return e.v.GetString("log_level")
}

func (e *Executor) NoColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return true
	}

	return e.v.GetBool("no_color")
}
