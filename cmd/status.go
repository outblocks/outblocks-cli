package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Deploy stack status",
		Long:  `Shows Outblocks stack status.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			return actions.NewDeploy(e.Log(), e.cfg, &actions.DeployOptions{
				SkipAllApps:     true,
				SkipBuild:       true,
				SkipDNS:         true,
				SkipApply:       true,
				SkipDiff:        true,
				SkipStateCreate: true,
			}).Run(cmd.Context())
		},
	}

	return cmd
}
