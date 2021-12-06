package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/spf13/cobra"
)

func (e *Executor) newInitCmd() *cobra.Command {
	opts := &actions.InitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize new config",
		Long:  `Initialize new Outblocks project config with opinionated defaults.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:          cmdGroupMain,
			cmdSkipLoadConfigAnnotation: "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewInit(e.Log(), e.PluginsCacheDir(), e.srv.Addr().String(), opts).Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.Overwrite, "overwrite", false, "do not ask if project file already exists")
	f.StringVar(&opts.Name, "name", "", "project name")
	f.StringVar(&opts.DeployPlugin, "deploy-plugin", "", "deploy plugin to use (e.g. gcp)")
	f.StringVar(&opts.RunPlugin, "run-plugin", "", "run plugin to use (e.g. docker)")
	f.StringVar(&opts.DNSDomain, "dns-domain", "", "main DNS domain to use with deployments")

	f.StringVar(&opts.GCP.Project, "gcp-project", "", "GCP project to use with GCP deploy plugin")
	f.StringVar(&opts.GCP.Region, "gcp-region", "", "GCP region to use with GCP deploy plugin")

	return cmd
}
