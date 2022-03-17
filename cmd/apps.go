package cmd

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apps",
		Aliases: []string{"app", "application", "applications"},
		Short:   "App management",
		Long:    `Applications management - list, add or delete.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:          cmdGroupMain,
			cmdSkipLoadConfigAnnotation: "1",
		},
	}

	listOpts := &actions.AppListOptions{}

	list := &cobra.Command{
		Use:   "list",
		Short: "List apps",
		Long:  `List configured applications.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			// TODO: list apps from state as well
			return actions.NewAppList(e.Log(), e.cfg, listOpts).Run(cmd.Context())
		},
	}

	del := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete an app",
		Long:    `Delete an existing application config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			// TODO: app deletion
			return nil
		},
	}

	addOpts := &actions.AppAddOptions{}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a new app",
		Long:  `Add a new application (generates new config).`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewAppAdd(e.Log(), e.cfg, addOpts).Run(cmd.Context())
		},
	}

	f := add.Flags()
	f.BoolVar(&addOpts.Overwrite, "overwrite", false, "do not ask if application definition already exists")
	f.StringVarP(&addOpts.Name, "name", "n", "", "application name")
	f.StringVarP(&addOpts.Dir, "dir", "d", "", "application dir, defaults to: <app_type>/<app_name>")
	f.StringVar(&addOpts.URL, "url", "", "application URL")
	f.StringVar(&addOpts.Type, "type", "", "application type (options: static, function, service)")
	f.StringVar(&addOpts.StaticBuildCommand, "static-build-command", "", "static app build command")
	f.StringVar(&addOpts.StaticBuildDir, "static-build-dir", "", "static app build dir")
	f.StringVar(&addOpts.RunCommand, "run-command", "", "app dev run command")
	f.StringVar(&addOpts.StaticRouting, "static-routing", config.StaticAppRoutingReact, fmt.Sprintf("static app routing (options: %s)", strings.Join(config.StaticAppRoutings, ", ")))
	f.StringVarP(&addOpts.OutputDir, "output-dir", "o", "", "output dir, defaults to application dir")
	f.StringVar(&addOpts.DeployPlugin, "deploy-plugin", "", "deploy plugin, defaults to first available")
	f.StringVar(&addOpts.RunPlugin, "run-plugin", "direct", "deploy plugin, defaults to direct run")

	cmd.AddCommand(
		list,
		del,
		add,
	)

	return cmd
}
