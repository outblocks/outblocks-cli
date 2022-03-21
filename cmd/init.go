package cmd

import (
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/spf13/cobra"
)

func (e *Executor) newInitCmd() *cobra.Command {
	opts := &actions.InitOptions{
		TemplateValueOpts: &values.Options{},
	}

	cmd := &cobra.Command{
		Use:   "init [flags] <path>",
		Short: "Initialize new config",
		Long:  `Initialize new Outblocks project config with opinionated defaults.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeSkip,
		},
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Path = args[0]
			}

			return actions.NewInit(e.Log(), e.PluginsCacheDir(), e.srv.Addr().String(), opts).Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.Overwrite, "overwrite", false, "do not ask if project file already exists")
	f.StringVarP(&opts.Template, "template", "t", "", "path/url of template to use")
	f.StringSliceVarP(&opts.TemplateValueOpts.ValueFiles, "template-values", "v", []string{}, "specify template values in a YAML/JSON file or a URL (can specify multiple or separate values with commas)")
	f.StringVar(&opts.Name, "name", "", "project name")
	f.StringVar(&opts.DeployPlugin, "deploy-plugin", "", "deploy plugin to use (e.g. gcp)")
	f.StringVar(&opts.RunPlugin, "run-plugin", "", "run plugin to use (e.g. docker)")
	f.StringVar(&opts.DNSDomain, "dns-domain", "", "main DNS domain to use with deployments")

	f.StringVar(&opts.GCP.Project, "gcp-project", "", "GCP project to use with GCP deploy plugin")
	f.StringVar(&opts.GCP.Region, "gcp-region", "", "GCP region to use with GCP deploy plugin")

	return cmd
}
