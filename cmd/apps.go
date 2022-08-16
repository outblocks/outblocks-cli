package cmd

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apps",
		Aliases: []string{"app", "application", "applications"},
		Short:   "App management",
		Long:    `Applications management - list, add or delete.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeSkip,
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List apps",
		Long:  `List configured applications.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeEssential,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewAppManager(e.Log(), e.cfg).List(cmd.Context())
		},
	}

	delOpts := &actions.DeployOptions{
		Destroy: true,
	}

	del := &cobra.Command{
		Use:     "delete [flags] <app name(s)>",
		Aliases: []string{"del", "remove"},
		Short:   "Delete an app",
		Long:    `Delete an existing application name in a form of <app type>.<name>, e.g.: static.website.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeEssential,
		},
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var apps []config.App

			delOpts.Targets = util.NewTargetMatcher()

			for _, arg := range args {
				err := delOpts.Targets.AddApp(arg)
				if err != nil {
					return err
				}

				for _, ap := range e.cfg.Apps {
					if delOpts.Targets.Matches(ap.ID()) {
						apps = append(apps, ap)
					}
				}
			}

			err := actions.NewDeploy(e.Log(), e.cfg, delOpts).Run(cmd.Context())
			if err != nil {
				return err
			}

			if len(apps) > 0 {
				e.log.Println("All apps deleted.")
				e.log.Println()
				e.log.Println("You still need to remove these apps locally:")

				for _, app := range apps {
					e.log.Printf("  * name: '%s' of type: '%s', path: %s\n", app.Name(), app.Type(), app.Dir())
				}
			}

			return nil
		},
	}

	addOpts := &actions.AppAddOptions{}

	add := &cobra.Command{
		Use:   "add",
		Short: "Add a new app",
		Long:  `Add a new application (generates new config).`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.NewAppManager(e.Log(), e.cfg).Add(cmd.Context(), addOpts)
		},
	}

	f := del.Flags()
	f.BoolVar(&delOpts.AutoApprove, "yes", false, "auto approve changes")
	f.BoolVar(&delOpts.ForceApprove, "force", false, "force approve even with critical changes")

	f = add.Flags()
	f.BoolVar(&addOpts.Overwrite, "overwrite", false, "do not ask if application definition already exists")
	f.StringVarP(&addOpts.Name, "name", "n", "", "application name")
	f.StringVarP(&addOpts.Dir, "dir", "d", "", "application dir, defaults to: <app_type>/<app_name>")
	f.StringVar(&addOpts.URL, "url", "", "application URL")
	f.StringVar(&addOpts.Type, "type", "", "application type (options: static, function, service)")
	f.StringVar(&addOpts.RunCommand, "run-command", "", "app dev run command")

	f.StringVar(&addOpts.StaticBuildCommand, "static-build-command", "", "(static only) static app build command")
	f.StringVar(&addOpts.StaticBuildDir, "static-build-dir", "", "(static only) static app build dir")
	f.StringVar(&addOpts.StaticRouting, "static-routing", "", fmt.Sprintf("(static only) static app routing (options: %s)", strings.Join(config.StaticAppRoutings, ", ")))

	f.StringVar(&addOpts.FunctionRuntime, "function-runtime", "", "(function only) the runtime in which the function is going to run")
	f.StringVar(&addOpts.FunctionEntrypoint, "function-entrypoint", "", "(function only) name of the function that will be executed when the function is triggered")

	f.StringVarP(&addOpts.OutputDir, "output-dir", "o", "", "output dir, defaults to application dir")
	f.StringVar(&addOpts.DeployPlugin, "deploy-plugin", "", "deploy plugin, defaults to first available")
	f.StringVar(&addOpts.RunPlugin, "run-plugin", "direct", "deploy plugin, defaults to direct run")

	cmd.AddCommand(
		list,
		del,
		add,
	)

	return cmd
}
