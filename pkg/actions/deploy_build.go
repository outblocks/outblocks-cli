package actions

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/ansel1/merry/v2"
	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/command"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
)

var (
	dockerServerMinimumVersion = semver.MustParse("18.09")
	dockerClientMinimumVersion = semver.MustParse("1.39")
	commandCleanupTimeout      = 5 * time.Second
)

func (d *Deploy) dockerClient(ctx context.Context) (*dockerclient.Client, error) {
	var err error

	d.once.dockerCli.Do(func() {
		d.dockerCli, err = dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
		if err != nil {
			return
		}

		var dockerVer dockertypes.Version

		// Server version.
		dockerVer, err = d.dockerCli.ServerVersion(ctx)
		if err != nil {
			return
		}

		ver := semver.MustParse(dockerVer.Version)

		if ver.LessThan(dockerServerMinimumVersion) {
			err = merry.Errorf("minimum docker server version required: %s", dockerServerMinimumVersion)
			return
		}

		// Client version.
		ver = semver.MustParse(d.dockerCli.ClientVersion())

		if ver.LessThan(dockerClientMinimumVersion) {
			err = merry.Errorf("minimum docker client version required: %s", dockerClientMinimumVersion)
			return
		}
	})

	if err != nil {
		return nil, merry.Errorf("error creating docker client: %w", err)
	}

	return d.dockerCli, err
}

func (d *Deploy) printAppOutput(app config.App, msg string, isErr bool) {
	prefix := fmt.Sprintf("APP:%s:%s:", app.Type(), app.Name())

	if isErr {
		d.log.Printf("%s %s\n", pterm.FgYellow.Sprint(prefix), plugin_util.StripAnsiControl(msg))
	} else {
		d.log.Printf("%s %s\n", pterm.FgGreen.Sprint(prefix), plugin_util.StripAnsiControl(msg))
	}
}

func (d *Deploy) runAppCommand(ctx context.Context, cmd *command.Cmd, app config.App) error {
	// Process stdout/stderr.
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		s := bufio.NewScanner(cmd.Stdout())

		for s.Scan() {
			d.printAppOutput(app, s.Text(), false)
		}

		wg.Done()
	}()

	go func() {
		s := bufio.NewScanner(cmd.Stderr())

		for s.Scan() {
			d.printAppOutput(app, s.Text(), true)
		}

		wg.Done()
	}()

	err := cmd.Run()
	if err != nil {
		return merry.Errorf("error running build for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	select {
	case <-ctx.Done():
		_ = cmd.Stop(commandCleanupTimeout)
	case <-cmd.WaitChannel():
	}

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		return merry.Errorf("error running build for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	return nil
}

func (d *Deploy) buildStaticApp(ctx context.Context, app *config.StaticApp, eval *util.VarEvaluator) error {
	var err error

	env := plugin_util.MergeStringMaps(app.Env(), app.Build.Env)

	env, err = eval.ExpandStringMap(env)
	if err != nil {
		return err
	}

	cmd, err := command.New(app.Build.Command.ExecCmdAsUser(), command.WithDir(app.Dir()), command.WithEnv(util.FlattenEnvMap(env)))
	if err != nil {
		return merry.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	return d.runAppCommand(ctx, cmd, app)
}

func (d *Deploy) buildServiceApp(ctx context.Context, app *config.ServiceApp, eval *util.VarEvaluator) error {
	dockercontext := filepath.Join(app.Dir(), app.Build.DockerContext)
	dockercontext, ok := plugin_util.CheckDir(dockercontext)

	if !ok {
		return merry.Errorf("%s app '%s' docker context '%s' does not exist", app.Type(), app.Name(), dockercontext)
	}

	dockerfile := filepath.Join(dockercontext, app.Build.Dockerfile)

	if !plugin_util.FileExists(dockerfile) {
		return merry.Errorf("%s app '%s' dockerfile '%s' does not exist", app.Type(), app.Name(), dockerfile)
	}

	cli, err := d.dockerClient(ctx)
	if err != nil {
		return err
	}

	buildArgsMap, err := eval.ExpandStringMap(app.Build.DockerBuildArgs)
	if err != nil {
		return err
	}

	buildArgs := util.FlattenEnvMap(buildArgsMap)

	secretsMap, err := eval.ExpandStringMap(app.Build.DockerSecrets)
	if err != nil {
		return err
	}

	explicitBuildx := false

	cmdArgs := []string{
		"--platform=linux/amd64",
		"--tag", app.AppBuild.LocalDockerImage,
		"--file", app.Build.Dockerfile,
		"--progress=plain",
	}

	if !d.opts.SkipPull && !app.Build.SkipPull {
		cmdArgs = append(cmdArgs, "--pull")
	}

	// Add cache if needed.
	dockerBuildCacheDir := d.opts.DockerBuildCacheDir
	dockerBuildCacheDirOutput := d.opts.DockerBuildCacheDirOutput

	if dockerBuildCacheDirOutput == "" {
		dockerBuildCacheDirOutput = dockerBuildCacheDir
	}

	if dockerBuildCacheDir != "" {
		dockerBuildCacheDir = filepath.Join(dockerBuildCacheDir, fmt.Sprintf("%s.%s", app.AppType, app.Name()))
		dockerBuildCacheDirOutput := filepath.Join(dockerBuildCacheDirOutput, fmt.Sprintf("%s.%s", app.AppType, app.Name()))

		_ = os.MkdirAll(dockerBuildCacheDir, 0o755)
		_ = os.MkdirAll(dockerBuildCacheDirOutput, 0o755)

		cacheFrom := fmt.Sprintf("type=local,modes=max,src=%s", dockerBuildCacheDir)
		cacheTo := fmt.Sprintf("type=local,dest=%s", dockerBuildCacheDirOutput)

		cmdArgs = append(cmdArgs,
			"--cache-from", cacheFrom,
			"--cache-to", cacheTo,
			"--load", // Load the image into docker after build.
		)

		explicitBuildx = true
	}

	// Add build args if needed.
	if len(buildArgs) > 0 {
		for _, a := range buildArgs {
			cmdArgs = append(cmdArgs, "--build-arg", a)
		}
	}

	// Add secrets if needed.
	if len(secretsMap) > 0 {
		for _, secret := range secretsMap {
			cmdArgs = append(cmdArgs, "--secret", secret)
		}
	}

	cmdArgs = append(cmdArgs, ".")

	if explicitBuildx {
		// Use buildx for explicit cache usage.
		cmdArgs = append([]string{"buildx", "build"}, cmdArgs...)
	} else {
		// Use normal build.
		cmdArgs = append([]string{"build"}, cmdArgs...)
	}

	cmd, err := command.New(
		exec.Command("docker", cmdArgs...),
		command.WithDir(dockercontext),
		command.WithEnv([]string{"DOCKER_BUILDKIT=1"}),
	)
	if err != nil {
		return merry.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	d.printAppOutput(app, fmt.Sprintf("Building image '%s'...", app.AppBuild.LocalDockerImage), false)

	err = d.runAppCommand(ctx, cmd, app)
	if err != nil {
		return err
	}

	insp, err := cli.ImageInspect(ctx, app.AppBuild.LocalDockerImage)
	if err != nil {
		return merry.Errorf("error inspecting created image for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	app.AppBuild.LocalDockerHash = insp.ID

	return nil
}

func (d *Deploy) buildFunctionApp(ctx context.Context, app *config.FunctionApp, eval *util.VarEvaluator) error {
	var err error

	if app.Build != nil && !app.Build.Command.IsEmpty() {
		env := plugin_util.MergeStringMaps(app.Env(), app.Build.Env)

		env, err = eval.ExpandStringMap(env)
		if err != nil {
			return err
		}

		cmd, err := command.New(app.Build.Command.ExecCmdAsUser(), command.WithDir(app.Dir()), command.WithEnv(util.FlattenEnvMap(env)))
		if err != nil {
			return merry.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
		}

		err = d.runAppCommand(ctx, cmd, app)
		if err != nil {
			return err
		}
	}

	// Archive if necessary.
	archive := app.DeployPlugin().AppOverrides.Function.Archive
	if archive != nil && !*archive {
		return nil
	}

	out := filepath.Join(d.opts.BuildTempDir, fmt.Sprintf("%s.zip", app.ID()))

	patterns := []string{
		"{*.outblocks.yaml,*.outblocks.yml,outblocks.yaml,outblocks.yml,.git,.gitignore,.DS_Store,node_modules,npm-debug.log}",
	}

	patterns = append(patterns, app.Package.Patterns...)

	appDir := app.Dir()
	if app.Build != nil {
		appDir = filepath.Join(appDir, app.Build.Dir)
	}

	err = fileutil.ArchiveDir(appDir, out, patterns)
	if err != nil {
		return merry.Errorf("error creating archive for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	hash, err := util.HashFile(out)
	if err != nil {
		return merry.Errorf("error hashing created archive for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	app.AppBuild.LocalArchivePath = out
	app.AppBuild.LocalArchiveHash = hex.EncodeToString(hash)

	return nil
}

type appPrepare struct {
	app     config.App
	prepare func() error
}

func (d *Deploy) prepareApps(ctx context.Context) error {
	var prepare []*appPrepare

	cli, err := d.dockerClient(ctx)
	if err != nil {
		return err
	}

	for _, app := range d.cfg.Apps {
		if app.Type() != config.AppTypeService {
			continue
		}

		a := app.(*config.ServiceApp)
		isCustom := a.ServiceAppProperties.Build.DockerImage != ""

		if d.opts.Destroy || (!d.opts.SkipBuild && !a.ServiceAppProperties.Build.SkipBuild) {
			continue
		}

		insp, err := cli.ImageInspect(ctx, a.AppBuild.LocalDockerImage)
		if err == nil {
			a.AppBuild.LocalDockerHash = insp.ID

			continue
		} else if !isCustom {
			return merry.Errorf("image '%s' for %s app %s does not seem to exist: %w", a.AppBuild.LocalDockerImage, app.Type(), app.Name(), err)
		}

		prepare = append(prepare, &appPrepare{
			app: app,
			prepare: func() error {
				cmd, err := command.New(
					exec.Command("docker", "pull", a.AppBuild.LocalDockerImage),
				)
				if err != nil {
					return merry.Errorf("error preparing pull command for %s app: %s: %w", app.Type(), app.Name(), err)
				}

				d.printAppOutput(app, fmt.Sprintf("Pulling image '%s'...", a.AppBuild.LocalDockerImage), false)

				err = d.runAppCommand(ctx, cmd, app)
				if err != nil {
					d.printAppOutput(app, fmt.Sprintf("error pulling custom image: %s", err), true)
				}

				insp, err := cli.ImageInspect(ctx, a.AppBuild.LocalDockerImage)
				if err != nil {
					return merry.Errorf("error inspecting image %s for %s app: %s: %w", a.AppBuild.LocalDockerImage, app.Type(), app.Name(), err)
				}

				a.AppBuild.LocalDockerHash = insp.ID

				return nil
			},
		})
	}

	if len(prepare) == 0 {
		return nil
	}

	d.log.Printf("Preparing %d app(s)...\n", len(prepare))
	prog, _ := d.log.ProgressBar().WithTotal(len(prepare)).WithTitle("Preparing apps...").Start()
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	for _, b := range prepare {
		b := b

		g.Go(func() error {
			err := b.prepare()
			if err != nil {
				return err
			}

			pterm.Success.Printf("%s app '%s' done\n", util.Title(b.app.Type()), b.app.Name())
			prog.Increment()

			return nil
		})
	}

	err = g.Wait()

	prog.Stop()

	return err
}

type appBuilder struct {
	app   config.App
	build func() error
}

func (d *Deploy) buildApps(ctx context.Context, stateApps map[string]*apiv1.AppState) error {
	appMap := make(map[string]*apiv1.AppState)

	// Prepare AppVars from state.
	for _, appState := range stateApps {
		if appState.App == nil {
			continue
		}

		appMap[appState.App.Id] = appState
	}

	appStates := make([]*apiv1.AppState, 0, len(appMap))
	for _, app := range appMap {
		appStates = append(appStates, app)
	}

	appStateVars := types.AppVarsFromAppStates(appStates)

	var builders []*appBuilder

	apps := d.cfg.Apps
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	var (
		appsTemp []config.App
		appTypes []*apiv1.App
	)

	for _, app := range apps {
		if !d.opts.Targets.IsEmpty() && !d.opts.Targets.Matches(app.ID()) {
			continue
		}

		if d.opts.Skips.Matches(app.ID()) {
			continue
		}

		appsTemp = append(appsTemp, app)
		appTypes = append(appTypes, app.Proto())
	}

	apps = appsTemp

	appVars := types.AppVarsFromApps(appTypes)
	appVars = types.MergeAppVars(appStateVars, appVars)

	for i, app := range apps {
		eval := util.NewVarEvaluator(types.VarsForApp(appVars, appTypes[i], nil))

		switch app.Type() {
		case config.AppTypeStatic:
			a := app.(*config.StaticApp)

			if a.Build.Command.IsEmpty() {
				continue
			}

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildStaticApp(ctx, a, eval) },
			})

		case config.AppTypeService:
			a := app.(*config.ServiceApp)
			if a.ServiceAppProperties.Build.SkipBuild {
				continue
			}

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildServiceApp(ctx, a, eval) },
			})

		case config.AppTypeFunction:
			a := app.(*config.FunctionApp)

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildFunctionApp(ctx, a, eval) },
			})
		}
	}

	if len(builders) == 0 {
		return nil
	}

	d.log.Printf("Building %d app(s)...\n", len(builders))
	prog, _ := d.log.ProgressBar().WithTotal(len(builders)).WithTitle("Building apps...").Start()

	for _, b := range builders {
		b := b

		g.Go(func() error {
			err := b.build()
			if err != nil {
				return err
			}

			pterm.Success.Printf("%s app '%s' built\n", util.Title(b.app.Type()), b.app.Name())
			prog.Increment()

			return nil
		})
	}

	err := g.Wait()

	prog.Stop()

	return err
}
