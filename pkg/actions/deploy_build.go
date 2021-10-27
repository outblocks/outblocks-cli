package actions

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Masterminds/semver"
	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
)

var (
	dockerServerMinimumVersion = semver.MustParse("18.09")
	dockerClientMinimumVersion = semver.MustParse("1.39")
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
			err = fmt.Errorf("minimum docker server version required: %s", dockerServerMinimumVersion)
			return
		}

		// Client version.
		ver = semver.MustParse(d.dockerCli.ClientVersion())

		if ver.LessThan(dockerClientMinimumVersion) {
			err = fmt.Errorf("minimum docker client version required: %s", dockerClientMinimumVersion)
			return
		}
	})

	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %w", err)
	}

	return d.dockerCli, err
}

func (d *Deploy) runAppBuildCommand(ctx context.Context, cmd *util.CmdInfo, app *config.BasicApp) error {
	prefix := fmt.Sprintf("APP:%s:%s:", app.Type(), app.Name())

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running build for %s app: %s: %w", app.Type(), app.Name(), err)
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
		_ = cmd.Stop()
	case <-cmd.WaitChannel():
	}

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("error running build for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	return nil
}

func (d *Deploy) buildStaticApp(ctx context.Context, app *config.StaticApp) error {
	cmd, err := util.NewCmdInfo(app.Build.Command, app.Dir(), nil)
	if err != nil {
		return fmt.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	return d.runAppBuildCommand(ctx, cmd, &app.BasicApp)
}

func (d *Deploy) buildServiceApp(ctx context.Context, app *config.ServiceApp) error {
	dockercontext := filepath.Join(app.Dir(), app.Build.DockerContext)
	dockercontext, ok := plugin_util.CheckDir(dockercontext)

	if !ok {
		return fmt.Errorf("%s app '%s' docker context '%s' does not exist", app.Type(), app.Name(), dockercontext)
	}

	dockerfile := filepath.Join(dockercontext, app.Build.Dockerfile)

	if !plugin_util.FileExists(dockerfile) {
		return fmt.Errorf("%s app '%s' dockerfile '%s' does not exist", app.Type(), app.Name(), dockerfile)
	}

	cli, err := d.dockerClient(ctx)
	if err != nil {
		return err
	}

	cmdStr := fmt.Sprintf("docker build --tag %s --pull --file %s --progress=plain .", app.LocalDockerImage, app.Build.Dockerfile)

	cmd, err := util.NewCmdInfo(cmdStr, dockercontext, []string{"DOCKER_BUILDKIT=1"})
	if err != nil {
		return fmt.Errorf("error preparing build command for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	err = d.runAppBuildCommand(ctx, cmd, &app.BasicApp)
	if err != nil {
		return err
	}

	insp, _, err := cli.ImageInspectWithRaw(ctx, app.LocalDockerImage)
	if err != nil {
		return fmt.Errorf("error inspecting created image for %s app: %s: %w", app.Type(), app.Name(), err)
	}

	app.LocalDockerHash = insp.ID

	return nil
}

type appBuilder struct {
	app   config.App
	build func() error
}

func (d *Deploy) buildApps(ctx context.Context) error {
	var builders []*appBuilder

	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	for _, app := range d.cfg.Apps {
		// TODO: add build app function
		switch app.Type() {
		case config.AppTypeStatic:
			a := app.(*config.StaticApp)

			if a.Build.Command == "" {
				continue
			}

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildStaticApp(ctx, a) },
			})

		case config.AppTypeService:
			a := app.(*config.ServiceApp)

			builders = append(builders, &appBuilder{
				app:   a,
				build: func() error { return d.buildServiceApp(ctx, a) },
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

	_, _ = prog.Stop()

	return err
}
