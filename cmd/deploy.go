package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy stack",
		Long:  `Deploys Outblocks stack and dependencies.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewDeploy(e.Log(), actions.DeployOptions{
				Verify: false,
			}).Run(cmd.Context(), e.cfg)
		},
	}

	return cmd
}
