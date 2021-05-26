package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/cli"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
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
	env.AddVarWithDefault("plugins_cache_dir", "plugins cache directory", clipath.DataPath("plugin-cache"))
	env.AddVar("no_color", "disable color output")
	env.AddVarWithDefault("log_level", "set logging level: debug | warn | error", "warn")
}

func (e *Executor) Execute(ctx context.Context) error {
	if err := e.initConfig(); err != nil {
		return err
	}

	if err := e.setupLogging(); err != nil {
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
	l := e.v.GetString("log_level")

	if err := e.log.SetLevel(l); err != nil {
		return err
	}

	color := !e.v.GetBool("no_color")
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
	e.v.AddConfigPath(clipath.ConfigPath())
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
