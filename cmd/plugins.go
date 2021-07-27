package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugins",
		Aliases: []string{"plugin"},
		Short:   "Plugin management",
		Long:    `Plugin management - list, update, add or delete.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List plugins",
		Long:  `List installed plugins.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewPluginList(e.Log(), e.loader).Run(cmd.Context(), e.cfg)
		},
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update plugins",
		Long:  `Update installed plugins to matching versions from config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdSkipCheckConfigAnnotation: "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewPluginUpdate(e.Log(), e.loader).Run(cmd.Context(), e.cfg)
		},
	}

	cmd.AddCommand(
		list,
		update,
	)

	// TODO: add, remove plugins

	return cmd
}
