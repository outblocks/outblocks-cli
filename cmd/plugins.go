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
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader, e.srv.Addr().String()).List(cmd.Context())
		},
	}

	var updateForce bool

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
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader, e.srv.Addr().String()).Update(cmd.Context(), updateForce)
		},
	}

	addOpts := &actions.PluginManagerAddOptions{}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add plugin",
		Long:  `Add plugins by name from config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:            cmdGroupMain,
			cmdProjectLoadModeAnnotation:  cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:     cmdLoadModeSkip,
			cmdProjectSkipCheckAnnotation: "1",
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader, e.srv.Addr().String()).Add(cmd.Context(), args[0], addOpts)
		},
	}

	del := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete plugin",
		Long:    `Delete installed plugins by name from config.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:            cmdGroupMain,
			cmdProjectLoadModeAnnotation:  cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:     cmdLoadModeSkip,
			cmdProjectSkipCheckAnnotation: "1",
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewPluginManager(e.Log(), e.cfg, e.loader, e.srv.Addr().String()).Delete(cmd.Context(), args[0])
		},
	}

	add.Flags().StringVarP(&addOpts.Source, "source", "s", "", "specify plugin source, only needed for plugins not created by Outblocks Team")
	add.Flags().StringVarP(&addOpts.Version, "version", "v", "", "specify plugin version, defaults to latest available version")
	update.Flags().BoolVar(&updateForce, "force", false, "force update, ignoring existing version constraints and update project YAML if needed")

	cmd.AddCommand(
		add,
		list,
		update,
		del,
	)

	return cmd
}
