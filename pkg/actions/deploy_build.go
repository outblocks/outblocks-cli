package actions

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/ansel1/merry/v2"
	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
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

func (d *Deploy) runAppBuildCommand(ctx context.Context, cmd *command.Cmd, app *config.BasicApp) error {
	prefix := fmt.Sprintf("APP:%s:%s:", app.Type(), app.Name())

	err := cmd.Run()
	if err != nil {
		return merry.Errorf("error running build for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	// Process stdout/stderr.
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		s := bufio.NewScanner(cmd.Stdout())

		for s.Scan() {
			d.log.Printf("%s %s\n", pterm.FgGreen.Sprint(prefix), plugin_util.StripAnsiControl(s.Text()))
		}

		wg.Done()
	}()

	go func() {
		s := bufio.NewScanner(cmd.Stderr())

		for s.Scan() {
			d.log.Printf("%s %s\n", pterm.FgYellow.Sprint(prefix), plugin_util.StripAnsiControl(s.Text()))
		}

		wg.Done()
	}()

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

	cmd, err := command.New(app.Build.Command, command.WithDir(app.Dir()), command.WithEnv(util.FlattenEnvMap(env)))
	if err != nil {
		return merry.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	return d.runAppBuildCommand(ctx, cmd, &app.BasicApp)
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
	for i, arg := range buildArgs {
		buildArgs[i] = strings.ReplaceAll(arg, "\"", "\\\"")
	}

	cmdStr := fmt.Sprintf("docker build --tag %s --pull --file %s --progress=plain", app.LocalDockerImage, app.Build.Dockerfile)

	// Add build args if needed.
	if len(buildArgs) > 0 {
		buildArgsStr := strings.Join(buildArgs, "\" --build-arg=\"")
		cmdStr += fmt.Sprintf("%s\"", buildArgsStr[1:])
	}

	cmdStr += " ."

	cmd, err := command.New(cmdStr, command.WithDir(dockercontext), command.WithEnv([]string{"DOCKER_BUILDKIT=1"}))
	if err != nil {
		return merry.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	err = d.runAppBuildCommand(ctx, cmd, &app.BasicApp)
	if err != nil {
		return err
	}

	insp, _, err := cli.ImageInspectWithRaw(ctx, app.LocalDockerImage)
	if err != nil {
		return merry.Errorf("error inspecting created image for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	app.LocalDockerHash = insp.ID

	return nil
}

type appBuilder struct {
	app   config.App
	build func() error
}

func (d *Deploy) buildApps(ctx context.Context) error {
	appTypeMap := make(map[string]*apiv1.App)

	if len(d.opts.TargetApps) != 0 || len(d.opts.SkipApps) != 0 {
		// Get state apps as well.
		state, _, err := getState(ctx, d.cfg, false, 0)
		if err != nil {
			return err
		}

		for _, appState := range state.Apps {
			appTypeMap[appState.App.Id] = appState.App
		}
	}

	var builders []*appBuilder

	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	apps := d.cfg.Apps
	targetAppIDsMap := util.StringArrayToSet(d.opts.TargetApps)
	skipAppIDsMap := util.StringArrayToSet(d.opts.SkipApps)

	var appsTemp []config.App

	for _, app := range apps {
		appTypeMap[app.ID()] = app.Proto()

		if len(targetAppIDsMap) > 0 && !targetAppIDsMap[app.ID()] {
			continue
		}

		if skipAppIDsMap[app.ID()] {
			continue
		}

		appsTemp = append(appsTemp, app)
	}

	apps = appsTemp

	// Flatten appTypeMap.
	appTypes := make([]*apiv1.App, 0, len(appTypeMap))
	for _, app := range appTypeMap {
		appTypes = append(appTypes, app)
	}

	appVars := types.AppVarsFromApps(appTypes)

	for _, app := range apps {
		eval := util.NewVarEvaluator(types.VarsForApp(appVars, app.Proto(), nil))

		// TODO: add build app function
		switch app.Type() {
		case config.AppTypeStatic:
			a := app.(*config.StaticApp)

			if a.Build.Command == "" {
				continue
			}

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildStaticApp(ctx, a, eval) },
			})

		case config.AppTypeService:
			a := app.(*config.ServiceApp)

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildServiceApp(ctx, a, eval) },
			})
		}
	}

	if len(builders) == 0 {
		return nil
	}

	d.log.Printf("Building %d apps...\n", len(builders))
	prog, _ := d.log.ProgressBar().WithTotal(len(builders)).WithTitle("Building apps...").Start()

	for _, b := range builders {
		b := b

		g.Go(func() error {
			err := b.build()
			if err != nil {
				return err
			}

			pterm.Success.Printf("Service app '%s' built\n", b.app.Name())
			prog.Increment()

			return nil
		})
	}

	err := g.Wait()

	prog.Stop()

	return err
}
