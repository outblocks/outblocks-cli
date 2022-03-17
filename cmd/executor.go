package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/goccy/go-yaml"
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
	lastUpdateCheckFile string

	opts struct {
		env       string
		valueOpts *values.Options
	}
}

func NewExecutor() *Executor {
	v := viper.New()

	e := &Executor{
		v:   v,
		env: cli.NewEnvironment(v),
		log: logger.NewLogger(),
	}

	e.opts.valueOpts = &values.Options{}

	setupEnvVars(e.env)

	e.rootCmd = e.newRoot()

	return e
}

func setupEnvVars(env *cli.Environment) {
	env.AddVarWithDefault("plugins_cache_dir", "plugins cache directory", clipath.DataDir("plugin-cache"))
	env.AddVar("no_color", "disable color output")
	env.AddVarWithDefault("log_level", "set logging level: debug | warn | error", "warn")
}

func (e *Executor) commandPreRun(ctx context.Context) error {
	var skipLoadConfig, skipLoadApps, skipLoadPlugins, skipCheckConfig bool

	// Parse critical root flags.
	e.rootCmd.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true

	err := e.rootCmd.PersistentFlags().Parse(os.Args[1:])
	if err != nil {
		return err
	}

	helpFlag := e.rootCmd.PersistentFlags().Lookup("help")
	e.opts.env = e.v.GetString("env")
	cmd, _, err := e.rootCmd.Find(os.Args[1:])

	isHelp := helpFlag.Changed || e.rootCmd == cmd || (len(os.Args) > 1 && strings.EqualFold(os.Args[1], "help"))

	if err == nil {
		skipLoadConfig = cmd.Annotations[cmdSkipLoadConfigAnnotation] == "1"
		skipLoadApps = cmd.Annotations[cmdSkipLoadAppsAnnotation] == "1"
		skipCheckConfig = cmd.Annotations[cmdSkipCheckConfigAnnotation] == "1"

		if skipLoadConfig {
			return nil
		}
	} else {
		skipLoadApps = true
	}

	// Load values.
	for i, v := range e.opts.valueOpts.ValueFiles {
		e.opts.valueOpts.ValueFiles[i] = strings.ReplaceAll(v, "<env>", e.opts.env)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return merry.Errorf("cannot find current directory: %w", err)
	}

	pwd, err = filepath.EvalSymlinks(pwd)
	if err != nil {
		return merry.Errorf("cannot evaluate current directory: %w", err)
	}

	cfgPath := fileutil.FindYAMLGoingUp(pwd, config.ProjectYAMLName)

	v, err := e.opts.valueOpts.MergeValues(ctx, filepath.Dir(cfgPath), getter.All())
	if err != nil {
		if isHelp {
			return nil
		}

		return merry.Errorf("cannot load values files: %w", err)
	}

	vals := map[string]interface{}{
		"var": v,
		"env": e.opts.env,
	}

	// Load config file.
	if err := e.loadProjectConfig(ctx, cfgPath, e.srv.Addr().String(), vals, skipLoadApps, skipLoadPlugins, skipCheckConfig); err != nil && !errors.Is(err, config.ErrProjectConfigNotFound) {
		return err
	}

	if skipLoadPlugins {
		return nil
	}

	// Augment/load new commands.
	return e.addPluginsCommands()
}

func (e *Executor) addPluginsCommands() error {
	if e.cfg == nil {
		return nil
	}

	for _, plug := range e.cfg.Plugins {
		for cmdName, cmdt := range plug.Loaded().Commands {
			cmdName := cmdName
			cmdt := cmdt

			cmdName = strings.ToLower(cmdName)

			cmd, _, err := e.rootCmd.Find([]string{cmdName})
			if err != nil {
				cmd = &cobra.Command{
					Use:   fmt.Sprintf("%s-%s", plug.Name, cmdName),
					Short: cmdt.Short,
					Long:  cmdt.Long,
					Annotations: map[string]string{
						cmdGroupAnnotation: cmdGroupPlugin,
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
	e.srv = server.NewServer(e.log)

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
	return e.v.GetBool("no_color")
}
