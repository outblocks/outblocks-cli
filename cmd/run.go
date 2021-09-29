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

	f := cmd.Flags()
	f.BoolVar(&opts.Direct, "direct", false, "force run all apps in local mode, directly running their commands")
	f.StringVarP(&opts.ListenIP, "listen-ip", "l", "127.0.0.1", "local server ip to listen on")
	f.IntVarP(&opts.ListenPort, "port", "p", 8000, "local server port")
	f.StringVar(&opts.HostsSuffix, "hosts-suffix", ".local.test", "local hosts suffix to use for url matching")
	f.BoolVar(&opts.HostsRouting, "hosts-routing", true, "adds local hosts and routes based on it, requires sudo/admin privilege")

	return cmd
}
