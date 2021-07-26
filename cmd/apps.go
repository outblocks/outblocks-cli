package cmd

import (
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

	list := &cobra.Command{
		Use:   "list",
		Short: "List apps",
		Long:  `List configured applications.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: app list
			return nil
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

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a new app",
		Long:  `Add a new application (generates new config).`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: app add
			return nil
		},
	}

	cmd.AddCommand(
		list,
		del,
		add,
	)

	return cmd
}
