package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/spf13/cobra"
)

func (e *Executor) newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "Secrets management",
		Long:    `Secrets management - get, set, delete or edit.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeSkip,
		},
	}

	get := &cobra.Command{
		Use:   "get [flags] <key>",
		Short: "Get secret value",
		Long:  `Get secret value through plugin provider.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).Get(cmd.Context(), args[0])
		},
	}

	set := &cobra.Command{
		Use:   "set [flags] <key> <value>",
		Short: "Set secret value",
		Long:  `Set secret value through plugin provider for specified key.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).Set(cmd.Context(), args[0], args[1])
		},
	}

	del := &cobra.Command{
		Use:   "delete [flags] <key>",
		Short: "Delete secret value",
		Long:  `Delete secret value through plugin provider of specified key.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).Delete(cmd.Context(), args[0])
		},
	}

	edit := &cobra.Command{
		Use:   "edit",
		Short: "Edit secrets in editor",
		Long:  `Edit secrets through $EDITOR (defaults to vim/nano/vi).`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).Edit(cmd.Context())
		},
	}

	var importFile string

	imp := &cobra.Command{
		Use:   "import",
		Short: "Import secrets from file",
		Long:  `Import secrets from YAML file.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).Import(cmd.Context(), importFile)
		},
	}

	view := &cobra.Command{
		Use:     "view",
		Aliases: []string{"list"},
		Short:   "View all secrets",
		Long:    `View all secrets from plugin provider.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewSecretsManager(e.Log(), e.cfg).View(cmd.Context())
		},
	}

	imp.Flags().StringVarP(&importFile, "file", "i", "", "secrets file to import")
	_ = imp.MarkFlagRequired("file")

	cmd.AddCommand(
		get,
		set,
		del,
		edit,
		imp,
		view,
	)

	return cmd
}
