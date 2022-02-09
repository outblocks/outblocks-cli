package cmd

import (
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newDeployCmd() *cobra.Command {
	opts := &actions.DeployOptions{}

	var targetApps, skipApps []string

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

			if len(opts.TargetApps) > 0 && len(opts.SkipApps) > 0 {
				return merry.New("target-apps and skip-apps arguments are mutually exclusive modes")
			}

			if opts.SkipAllApps && opts.Destroy {
				return merry.New("skip-all-apps and destroy are mutually exclusive modes")
			}

			for _, t := range targetApps {
				tsplit := strings.SplitN(t, ".", 2)
				if len(tsplit) != 2 {
					return merry.Errorf("wrong format for target '%s': specify in a form of <app type>.<name>, e.g.: static.website", t)
				}

				opts.TargetApps = append(opts.TargetApps, config.ComputeAppID(tsplit[0], tsplit[1]))
			}

			if !opts.SkipAllApps {
				for _, t := range skipApps {
					tsplit := strings.SplitN(t, ".", 2)
					if len(tsplit) != 2 {
						return merry.Errorf("wrong format for skip '%s': specify in a form of <app type>.<name>, e.g.: static.website", t)
					}

					opts.SkipApps = append(opts.SkipApps, config.ComputeAppID(tsplit[0], tsplit[1]))
				}
			}

			if opts.Destroy || opts.SkipAllApps {
				opts.SkipBuild = true
			}

			return actions.NewDeploy(e.Log(), e.cfg, opts).Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.Verify, "verify", false, "verify existing resources")
	f.BoolVar(&opts.Destroy, "destroy", false, "destroy all existing resources")
	f.BoolVar(&opts.SkipBuild, "skip-build", false, "skip build command before deploy")
	f.BoolVar(&opts.Lock, "lock", true, "lock statefile during deploy")
	f.DurationVar(&opts.LockWait, "lock-wait", 0, "wait for lock if it is already acquired")
	f.BoolVar(&opts.AutoApprove, "yes", false, "auto approve changes")
	f.BoolVar(&opts.ForceApprove, "force", false, "force approve even with critical changes")
	f.StringSliceVarP(&targetApps, "target-apps", "t", nil, "target only specified apps, can specify multiple or separate values with comma in a form of <app type>.<name>, e.g.: static.website,service.api")
	f.StringSliceVarP(&skipApps, "skip-apps", "s", nil, "skip specified apps (if they exist), can specify multiple or separate values with comma in a form of <app type>.<name>, e.g.: static.website,service.api")
	f.BoolVar(&opts.SkipAllApps, "skip-all-apps", false, "skip all apps (if they exist)")
	f.BoolVar(&opts.SkipDNS, "skip-dns", false, "skip DNS setup (including self managed certificates)")

	return cmd
}
