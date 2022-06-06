package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/spf13/cobra"
)

func (e *Executor) newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugins",
		Aliases: []string{"plugin"},
		Short:   "Plugin management",
		Long:    `Plugin management - list, update, add or delete.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeSkip,
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List plugins",
		Long:  `List installed plugins.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:            cmdGroupMain,
			cmdProjectLoadModeAnnotation:  cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:     cmdLoadModeSkip,
			cmdProjectSkipCheckAnnotation: "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader).List(cmd.Context())
		},
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update plugins",
		Long:  `Update installed plugins to matching versions from config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:            cmdGroupMain,
			cmdProjectLoadModeAnnotation:  cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:     cmdLoadModeSkip,
			cmdProjectSkipCheckAnnotation: "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader).Update(cmd.Context())
		},
	}

	cmd.AddCommand(
		list,
		update,
	)

	// TODO: add, remove plugins

	return cmd
}
