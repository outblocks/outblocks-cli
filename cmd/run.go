package cmd

import (
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/spf13/cobra"
)

func (e *Executor) newRunCmd() *cobra.Command {
	opts := &actions.RunOptions{}

	var targets, skips []string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs stack locally",
		Long:  `Runs Outblocks stack and dependencies locally.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeFull,
			cmdAppsLoadModeAnnotation:    cmdLoadModeFull,
			cmdSecretsLoadAnnotation:     "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Targets = util.NewTargetMatcher()
			opts.Skips = util.NewTargetMatcher()

			targets = append(targets, args...)

			for _, t := range targets {
				err := opts.Targets.Add(t)
				if err != nil {
					return err
				}
			}

			for _, t := range skips {
				err := opts.Skips.Add(t)
				if err != nil {
					return err
				}
			}

			return actions.NewRun(e.log, e.cfg, opts).Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.Direct, "direct", false, "force run all apps in local mode, directly running their commands")
	f.StringSliceVarP(&targets, "target", "t", nil, "target only specified apps/dependencies, can specify multiple or separate values with comma in a form of <type>.<name>, e.g.: static.website,service.api,dep.database")
	f.StringSliceVarP(&skips, "skip", "s", nil, "skip specified apps/dependencies (if they exist), can specify multiple or separate values with comma in a form of <type>.<name>, e.g.: static.website,service.api,dep.database")
	f.StringVarP(&opts.ListenIP, "listen-ip", "l", "127.0.0.1", "local server ip to listen on")
	f.IntVarP(&opts.ListenPort, "port", "p", 8000, "local server port")
	f.StringVar(&opts.HostsSuffix, "hosts-suffix", ".local.test", "local hosts suffix to use for url matching")
	f.BoolVar(&opts.HostsRouting, "hosts-routing", true, "adds local hosts and routes based on it, requires sudo/admin privilege")

	return cmd
}
