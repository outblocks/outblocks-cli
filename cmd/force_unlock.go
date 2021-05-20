package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newForceUnlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "force-unlock",
		Short: "Force unlock",
		Long:  `Release a stuck lock.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewForceUnlock(e.Log()).Run(cmd.Context(), e.cfg, args[0])
		},
	}

	return cmd
}
