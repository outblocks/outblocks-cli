package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newRunCmd() *cobra.Command {
	opts := &actions.RunOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs stack locally",
		Long:  `Runs Outblocks stack and dependencies locally.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewRun(e.log, e.cfg, opts).Run(cmd.Context())
		},
	}

	return cmd
}
