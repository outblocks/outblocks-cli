package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/cli"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
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

	cfg *config.Project

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
	cmd, _, _ := e.rootCmd.Find(os.Args[1:])

	if cmd != nil {
		skipLoadConfig = cmd.Annotations[cmdSkipLoadConfigAnnotation] == "1"
		skipLoadApps = cmd.Annotations[cmdSkipLoadAppsAnnotation] == "1"
		skipCheckConfig = cmd.Annotations[cmdSkipCheckConfigAnnotation] == "1"

		if skipLoadConfig {
			return nil
		}
	}

	// Load values.
	for i, v := range e.opts.valueOpts.ValueFiles {
		e.opts.valueOpts.ValueFiles[i] = strings.ReplaceAll(v, "<env>", e.opts.env)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot find current directory: %w", err)
	}

	pwd, err = filepath.EvalSymlinks(pwd)
	if err != nil {
		return fmt.Errorf("cannot evaluate current directory: %w", err)
	}

	cfgPath := fileutil.FindYAMLGoingUp(pwd, config.ProjectYAMLName)

	v, err := e.opts.valueOpts.MergeValues(ctx, filepath.Dir(cfgPath), getter.All())
	if err != nil {
		if helpFlag.Changed {
			return nil
		}

		return fmt.Errorf("cannot load values files: %w", err)
	}

	vals := map[string]interface{}{"var": v}

	// Load config file.
	if err := e.loadProjectConfig(ctx, cfgPath, vals, skipLoadApps, skipLoadPlugins, skipCheckConfig); err != nil && !errors.Is(err, config.ErrProjectConfigNotFound) {
		return err
	}

	// Augment/load new commands.
	return e.addPluginsCommands(cmd)
}

func (e *Executor) addPluginsCommands(cmd *cobra.Command) error {
	skipLoadPlugins := cmd.Annotations[cmdSkipLoadPluginsAnnotation] == "1"

	if skipLoadPlugins || e.cfg == nil {
		return nil
	}

	for _, plug := range e.cfg.Plugins {
		for cmdName, cmdt := range plug.Loaded().Commands {
			cmdName = strings.ToLower(cmdName)

			// TODO: add possibility to add new commands
			if !strings.EqualFold(cmdName, cmd.Use) {
				continue
			}

			flags := cmd.Flags()

			for _, arg := range cmdt.Args {
				arg.Name = strings.ToLower(arg.Name)

				if flags.Lookup(arg.Name) != nil {
					return fmt.Errorf("plugin tried to add already existing argument '%s' to command '%s'", arg, cmdName)
				}

				switch arg.Type {
				case plugins.CommandTypeBool:
					def, _ := arg.Default.(bool)
					arg.Value = flags.Bool(arg.Name, def, arg.Usage)
				case plugins.CommandTypeInt:
					def, _ := arg.Default.(int)
					arg.Value = flags.Int(arg.Name, def, arg.Usage)
				case plugins.CommandTypeString:
					def, _ := arg.Default.(string)
					arg.Value = flags.String(arg.Name, def, arg.Usage)
				}
			}
		}
	}

	return nil
}

func (e *Executor) Execute(ctx context.Context) error {
	if err := e.initConfig(); err != nil {
		return err
	}

	if err := e.setupLogging(); err != nil {
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
