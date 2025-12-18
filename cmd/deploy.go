package cmd

import (
	"fmt"
	"os"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/spf13/cobra"
)

func (e *Executor) newDeployCmd() *cobra.Command {
	opts := &actions.DeployOptions{}

	var targets, skips []string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy stack",
		Long:  `Deploys Outblocks stack and dependencies.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeFull,
			cmdAppsLoadModeAnnotation:    cmdLoadModeFull,
			cmdSecretsLoadAnnotation:     "1",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BuildTempDir = clipath.CacheDir(fmt.Sprintf("builds_%s_%s", e.cfg.Name, e.cfg.Env()))
			opts.Targets = util.NewTargetMatcher()
			opts.Skips = util.NewTargetMatcher()

			targets = append(targets, args...)

			_ = os.RemoveAll(opts.BuildTempDir)

			err := fileutil.MkdirAll(opts.BuildTempDir, 0o755)
			if err != nil {
				return merry.Errorf("cannot create build cache dir %s: %w", opts.BuildTempDir, err)
			}

			defer os.RemoveAll(opts.BuildTempDir)

			if opts.MergeMode {
				if len(targets) > 0 || len(skips) > 0 || opts.SkipAllApps {
					return merry.New("merge-mode already implies which apps are to be targeted/skipped")
				}

				for _, app := range e.cfg.Apps {
					_ = opts.Targets.AddApp(app.ID())
				}
			}

			if len(targets) > 0 && len(skips) > 0 {
				return merry.New("target-apps and skip-apps arguments are mutually exclusive modes")
			}

			for _, t := range targets {
				err := opts.Targets.Add(t)
				if err != nil {
					return err
				}
			}

			if !opts.SkipAllApps {
				for _, t := range skips {
					err := opts.Skips.Add(t)
					if err != nil {
						return err
					}
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
	e.env.BindCLIFlag("verify", f.Lookup("verify"))
	f.BoolVar(&opts.Destroy, "destroy", false, "destroy all existing resources")
	e.env.BindCLIFlag("destroy", f.Lookup("destroy"))
	f.BoolVar(&opts.SkipBuild, "skip-build", false, "skip build command before deploy")
	e.env.BindCLIFlag("skip_build", f.Lookup("skip-build"))
	f.BoolVar(&opts.SkipPull, "skip-pull", false, "skip docker images pull phase before deploy")
	e.env.BindCLIFlag("skip_pull", f.Lookup("skip-pull"))
	f.StringVar(&opts.DockerBuildCacheDir, "docker-build-cache-dir", "", "directory to use for docker build cache")
	e.env.BindCLIFlag("docker_build_cache_dir", f.Lookup("docker-build-cache-dir"))
	f.BoolVar(&opts.Lock, "lock", true, "acquire locks during deploy")
	e.env.BindCLIFlag("lock", f.Lookup("lock"))
	f.DurationVar(&opts.LockWait, "lock-wait", 0, "wait for lock if it is already acquired")
	e.env.BindCLIFlag("lock_wait", f.Lookup("lock-wait"))
	f.BoolVar(&opts.AutoApprove, "yes", false, "auto approve changes")
	e.env.BindCLIFlag("auto_approve", f.Lookup("yes"))
	f.BoolVar(&opts.ForceApprove, "force", false, "force approve even with critical changes")
	e.env.BindCLIFlag("force_approve", f.Lookup("force"))
	f.BoolVar(&opts.MergeMode, "merge-mode", false, "merge mode targets all apps that can be found (this is the same behavior as if all apps with visible app config were targeted manually)")
	e.env.BindCLIFlag("merge_mode", f.Lookup("merge-mode"))
	f.StringSliceVarP(&targets, "target-apps", "t", nil, "target only specified apps, can specify multiple or separate values with comma in a form of <app type>.<name>, e.g.: static.website,service.api")
	e.env.BindCLIFlag("target_apps", f.Lookup("target-apps"))
	f.StringSliceVarP(&skips, "skip-apps", "s", nil, "skip specified apps (if they exist), can specify multiple or separate values with comma in a form of <app type>.<name>, e.g.: static.website,service.api")
	e.env.BindCLIFlag("skip_apps", f.Lookup("skip-apps"))
	f.BoolVar(&opts.SkipAllApps, "skip-all-apps", false, "skip all apps (if they exist)")
	e.env.BindCLIFlag("skip_all_apps", f.Lookup("skip-all-apps"))
	f.BoolVar(&opts.SkipDNS, "skip-dns", false, "skip DNS setup (including self managed certificates)")
	e.env.BindCLIFlag("skip_dns", f.Lookup("skip-dns"))
	f.BoolVar(&opts.SkipMonitoring, "skip-monitoring", false, "skip monitoring setup")
	e.env.BindCLIFlag("skip_monitoring", f.Lookup("skip-monitoring"))

	return cmd
}
