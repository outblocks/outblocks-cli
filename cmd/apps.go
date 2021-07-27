package cmd

import (
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
			cmdGroupAnnotation: cmdGroupMain,
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

			return actions.NewAppList(e.Log(), listOpts).Run(cmd.Context(), e.cfg)
		},
	}

	del := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete an app",
		Long:    `Delete an existing application config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdSkipLoadPluginsAnnotation: "1",
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

			return actions.NewAppAdd(e.Log(), addOpts).Run(cmd.Context(), e.cfg)
		},
	}

	f := add.Flags()
	f.BoolVar(&addOpts.Overwrite, "overwrite", false, "do not ask if application definition already exists")
	f.StringVar(&addOpts.Name, "name", "", "application name")
	f.StringVar(&addOpts.URL, "url", "", "application URL")
	f.StringVar(&addOpts.Type, "type", "", "application type (options: static, function, service)")
	f.StringVar(&addOpts.Static.BuildCommand, "static-build-command", "", "static app build command")
	f.StringVar(&addOpts.Static.BuildDir, "static-build-dir", "", "static app build dir")
	f.StringVar(&addOpts.Static.Routing, "static-routing", "", "static app routing (options: react, disabled)")
	f.StringVarP(&addOpts.OutputPath, "output-path", "o", "", "output path, defaults to: <app_type>/<app_name>")

	cmd.AddCommand(
		list,
		del,
		add,
	)

	return cmd
}