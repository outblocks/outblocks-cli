package actions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/outblocks/outblocks-cli/internal/urlutil"
	"github.com/outblocks/outblocks-cli/pkg/actions/run"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
	"github.com/txn2/txeh"
)

const runCommand = "run"

type Run struct {
	log  logger.Logger
	cfg  *config.Project
	opts *RunOptions

	hosts      *txeh.Hosts
	addedHosts []string
}

type RunOptions struct {
	Direct       bool
	ListenIP     string
	ListenPort   int
	HostsSuffix  string
	HostsRouting bool
}

type runInfo struct {
	apps []*types.AppRun
	deps []*types.DependencyRun

	localApps     []*run.LocalApp
	localDeps     []*run.LocalDependency
	pluginAppsMap map[*plugins.Plugin]*plugin_go.RunRequest
	pluginDepsMap map[*plugins.Plugin]*plugin_go.RunRequest
}

const (
	loopbackHost       = "outblocks.host"
	loopbackIP         = "127.0.0.1"
	cleanupTimeout     = 10 * time.Second
	healthcheckSleep   = 1 * time.Second
	healthcheckTimeout = 3 * time.Second
)

func NewRun(log logger.Logger, cfg *config.Project, opts *RunOptions) *Run {
	return &Run{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *Run) cleanup() error {
	if len(d.addedHosts) > 0 {
		d.hosts.RemoveHosts(d.addedHosts)
		return d.hosts.Save()
	}

	return nil
}

func (d *Run) AddHosts(hosts ...string) error {
	d.hosts.AddHosts(loopbackIP, hosts)

	err := d.hosts.Save()
	if err != nil {
		return err
	}

	d.addedHosts = append(d.addedHosts, hosts...)

	return nil
}

func (d *Run) init() error {
	var err error

	d.hosts, err = txeh.NewHostsDefault()
	if err != nil {
		return err
	}

	backupHosts := clipath.DataDir("hosts.original")
	if _, err := os.Stat(backupHosts); os.IsNotExist(err) {
		if err = plugin_util.CopyFile(d.hosts.WriteFilePath, backupHosts, 0755); err != nil {
			return fmt.Errorf("cannot backup hosts file: %w", err)
		}
	}

	return err
}

func (d *Run) localURL(u *url.URL, port int, pathRedirect string) string {
	if !d.opts.HostsRouting {
		host := d.opts.ListenIP
		if d.opts.ListenIP == "127.0.0.1" {
			host = "localhost"
		}

		return fmt.Sprintf("http://%s:%d%s", host, port, pathRedirect)
	}

	return fmt.Sprintf("http://%s%s:%d%s", u.Hostname(), d.opts.HostsSuffix, d.opts.ListenPort, u.Path)
}

func (d *Run) loopbackHost() string {
	return loopbackHost + d.opts.HostsSuffix
}

func (d *Run) prepareRun(cfg *config.Project) (*runInfo, error) {
	info := &runInfo{
		pluginAppsMap: make(map[*plugins.Plugin]*plugin_go.RunRequest),
		pluginDepsMap: make(map[*plugins.Plugin]*plugin_go.RunRequest),
	}

	loopbackHost := d.loopbackHost()
	hosts := map[string]string{
		loopbackHost: loopbackIP,
	}

	// Apps.
	port := d.opts.ListenPort + 1

	for _, app := range cfg.Apps {
		if app.RunInfo().Command == "" {
			return nil, app.YAMLError("$.run.command", "App.Run.Command is required to run app")
		}

		runInfo := app.RunInfo()

		appPort := runInfo.Port
		if appPort == 0 {
			appPort = port
			port++
		}

		appType := app.PluginType()
		appType.Env = plugin_util.MergeStringMaps(appType.Env, runInfo.Env)
		appType.Properties = plugin_util.MergeMaps(cfg.Defaults.Run.Other, appType.Properties, runInfo.Other)

		appRun := &types.AppRun{
			App:     appType,
			URL:     d.localURL(app.URL(), appPort, app.PathRedirect()),
			IP:      loopbackIP,
			Port:    appPort,
			Command: runInfo.Command,
		}

		info.apps = append(info.apps, appRun)

		if d.opts.Direct && app.SupportsLocal() {
			info.localApps = append(info.localApps, &run.LocalApp{
				AppRun: appRun,
			})

			continue
		}

		runPlugin := app.RunPlugin()

		if _, ok := info.pluginAppsMap[runPlugin]; !ok {
			info.pluginAppsMap[runPlugin] = &plugin_go.RunRequest{
				Args:  runPlugin.CommandArgs(runCommand),
				Hosts: hosts,
			}
		}

		info.pluginAppsMap[runPlugin].Apps = append(info.pluginAppsMap[runPlugin].Apps, appRun)
	}

	// Dependencies.
	for _, dep := range cfg.Dependencies {
		depType := dep.PluginType()
		depPort := dep.Run.Port

		if depPort == 0 {
			depPort = port
			port++
		}

		depType.Properties = plugin_util.MergeMaps(cfg.Defaults.Run.Other, depType.Properties)

		depRun := &types.DependencyRun{
			Dependency: depType,
			IP:         loopbackIP,
			Port:       depPort,
		}

		info.deps = append(info.deps, depRun)

		if d.opts.Direct && dep.SupportsLocal() {
			info.localDeps = append(info.localDeps, &run.LocalDependency{
				DependencyRun: depRun,
			})

			continue
		}

		runPlugin := dep.RunPlugin()

		if _, ok := info.pluginDepsMap[runPlugin]; !ok {
			info.pluginDepsMap[runPlugin] = &plugin_go.RunRequest{
				Args:  runPlugin.CommandArgs(runCommand),
				Hosts: hosts,
			}
		}

		info.pluginDepsMap[runPlugin].Dependencies = append(info.pluginDepsMap[runPlugin].Dependencies, depRun)
	}

	// Gather envs.
	// TODO!: process envs similar as when deployment rather then have same for all, dont add anything automatically
	env := make(map[string]string)

	for _, app := range info.apps {
		prefix := app.App.EnvPrefix()

		host, _ := urlutil.ExtractHostname(app.URL)
		env[fmt.Sprintf("%sURL", prefix)] = app.URL

		hosts[host] = app.IP
	}

	for _, dep := range info.deps {
		// TODO: treat deps differently, only use these that were added as needs
		// + add secrets from plugin.PrepareRunDependency()
		prefix := dep.Dependency.EnvPrefix()

		env[fmt.Sprintf("%sHOST", prefix)] = loopbackHost
		env[fmt.Sprintf("%sPORT", prefix)] = strconv.Itoa(dep.Port)
	}

	// Fill envs per app/dep.
	for _, app := range info.apps {
		app.App.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, app.App.Env, env)

		app.App.Env["URL"] = app.URL
		app.App.Env["PORT"] = strconv.Itoa(app.Port)
	}

	for _, dep := range info.deps {
		dep.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, dep.Env, env)

		dep.Env["IP"] = dep.IP
		dep.Env["PORT"] = strconv.Itoa(dep.Port)
	}

	return info, nil
}

func (d *Run) newHTTPServer(routing map[*url.URL]*url.URL) *http.Server {
	mux := http.NewServeMux()

	for k, v := range routing {
		mux.HandleFunc(k.Hostname()+k.Path, httputil.NewSingleHostReverseProxy(v).ServeHTTP)
	}

	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", d.opts.ListenIP, d.opts.ListenPort),
		Handler: mux,
	}
}

func (d *Run) runAll(ctx context.Context, runInfo *runInfo) ([]*run.PluginRunResult, []*run.LocalRunResult, error) {
	spinner, _ := d.log.Spinner().Start("Starting apps and dependencies...")

	var (
		pluginRets []*run.PluginRunResult
		localRets  []*run.LocalRunResult
	)

	// Process remote plugin dependencies.
	if len(runInfo.pluginDepsMap) > 0 {
		pluginRet, err := run.ThroughPlugin(ctx, runInfo.pluginDepsMap)
		if err != nil {
			spinner.Stop()
			return nil, nil, err
		}

		pluginRets = append(pluginRets, pluginRet)

		go func() {
			<-ctx.Done()

			_ = pluginRet.Stop()
		}()
	}

	// Process local dependencies.
	if len(runInfo.localDeps) > 0 {
		localRet, err := run.Local(ctx, nil, runInfo.localDeps)

		if err != nil {
			spinner.Stop()
			return nil, nil, err
		}

		localRets = append(localRets, localRet)

		go func() {
			<-ctx.Done()

			_ = localRet.Stop()
		}()
	}

	// Process remote plugin apps.
	if len(runInfo.pluginAppsMap) > 0 {
		pluginRet, err := run.ThroughPlugin(ctx, runInfo.pluginAppsMap)
		if err != nil {
			spinner.Stop()
			return nil, nil, err
		}

		pluginRets = append(pluginRets, pluginRet)

		go func() {
			<-ctx.Done()

			_ = pluginRet.Stop()
		}()
	}

	// Process local apps.
	if len(runInfo.localApps) > 0 {
		localRet, err := run.Local(ctx, runInfo.localApps, nil)

		if err != nil {
			spinner.Stop()
			return nil, nil, err
		}

		localRets = append(localRets, localRet)

		go func() {
			<-ctx.Done()

			_ = localRet.Stop()
		}()
	}

	spinner.Stop()

	return pluginRets, localRets, nil
}

func (d *Run) waitAll(ctx context.Context, runInfo *runInfo) error {
	prog, _ := d.log.ProgressBar().WithTotal(len(runInfo.apps)).WithTitle("Waiting for apps and dependencies to be up...").Start()

	httpClient := &http.Client{
		Timeout: healthcheckTimeout,
	}

	g, _ := errgroup.WithContext(ctx)

	for _, app := range runInfo.apps {
		app := app

		req, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("http://%s:%d/", app.IP, app.Port), nil)
		if err != nil {
			return err
		}

		g.Go(func() error {
			for {
				resp, err := httpClient.Do(req)
				if errors.Is(err, context.Canceled) {
					return err
				}

				if err == nil {
					_ = resp.Body.Close()

					d.log.Printf("%s App '%s' is UP.\n", strings.Title(app.App.Type), app.App.Name)
					prog.Increment()

					return nil
				}

				time.Sleep(healthcheckSleep)
			}
		})
	}

	err := g.Wait()
	prog.Stop()

	return err
}

func formatRunOutput(log logger.Logger, cfg *config.Project, r *plugin_go.RunOutputResponse) {
	var prefix string

	msg := plugin_util.StripAnsiControl(r.Message)

	switch r.Source {
	case plugin_go.RunOutpoutSourceApp:
		app := cfg.AppByID(r.ID)
		prefix = fmt.Sprintf("APP:%s:%s:", app.Type(), app.Name())

	case plugin_go.RunOutpoutSourceDependency:
		prefix = fmt.Sprintf("DEP:%s", r.Name)
	}

	if r.IsStderr {
		log.Printf("%s %s\n", pterm.FgRed.Sprintf(prefix), msg)
	} else {
		log.Printf("%s %s\n", pterm.FgGreen.Sprintf(prefix), msg)
	}
}

func (d *Run) addAllHosts(runInfo *runInfo) (map[*url.URL]*url.URL, error) {
	hosts := map[string]struct{}{
		d.loopbackHost(): {},
	}

	routing := make(map[*url.URL]*url.URL)

	for _, s := range runInfo.apps {
		u, _ := url.Parse(s.URL)
		hosts[u.Hostname()] = struct{}{}

		uLocal := *u
		uLocal.Host = fmt.Sprintf("%s:%d", s.IP, s.Port)
		uLocal.Path = s.App.PathRedirect

		routing[u] = &uLocal
	}

	hostsList := make([]string, 0, len(hosts))

	for h := range hosts {
		hostsList = append(hostsList, h)
	}

	err := d.AddHosts(hostsList...)
	if err != nil {
		return nil, fmt.Errorf("are you running with sudo? or try running with hosts-routing disabled")
	}

	return routing, nil
}

func (d *Run) start(ctx context.Context, runInfo *runInfo) (*sync.WaitGroup, error) {
	var (
		wg      sync.WaitGroup
		routing map[*url.URL]*url.URL
		err     error
	)

	errCh := make(chan error, 1)

	runnerCtx, runnerCancel := context.WithCancel(ctx)
	defer runnerCancel()

	if d.opts.HostsRouting {
		routing, err = d.addAllHosts(runInfo)
		if err != nil {
			return &wg, err
		}
	}

	// Start all apps and deps.
	pluginRets, localRets, err := d.runAll(runnerCtx, runInfo)
	if err != nil {
		return &wg, err
	}

	total := len(localRets) + len(pluginRets)
	if total == 0 {
		return &wg, fmt.Errorf("nothing to run")
	}

	wg.Add(total)

	if d.opts.HostsRouting {
		wg.Add(1)

		go func() {
			err = d.runHTTPServer(runnerCtx, routing)

			wg.Done()

			if err != nil {
				runnerCancel()
				errCh <- err
			}
		}()
	}

	for _, localRet := range localRets {
		localRet := localRet

		go func() {
			for {
				msg, ok := <-localRet.OutputCh
				if !ok {
					return
				}

				formatRunOutput(d.log, d.cfg, msg)
			}
		}()

		go func() {
			err = localRet.Wait()

			wg.Done()

			if err != nil {
				runnerCancel()
				errCh <- err
			}
		}()
	}

	for _, pluginRet := range pluginRets {
		pluginRet := pluginRet

		go func() {
			for {
				msg, ok := <-pluginRet.OutputCh
				if !ok {
					return
				}

				formatRunOutput(d.log, d.cfg, msg)
			}
		}()

		go func() {
			err = pluginRet.Wait()

			wg.Done()

			if err != nil {
				runnerCancel()
				errCh <- err
			}
		}()
	}

	// Healthcheck.
	err = d.waitAll(runnerCtx, runInfo)
	if err != nil {
		select {
		case err := <-errCh:
			return &wg, err
		default:
		}

		return &wg, err
	}

	// Show apps status.
	d.log.Println()
	d.log.Println("All apps are UP.")

	for _, a := range runInfo.apps {
		d.log.Printf("%s App '%s' listening at %s\n", strings.Title(a.App.Type), a.App.Name, a.URL)
	}

	d.log.Println()

	<-runnerCtx.Done()

	select {
	case err := <-errCh:
		return &wg, err
	default:
	}

	return &wg, nil
}

func (d *Run) runSelfAsSudo() error {
	args := []string{"-E"}

	d.log.Println("Hosts routing requires admin privileges, re-running with sudo...")

	d.log.Level()
	d.log.SetLevel(logger.LogLevelError)

	args = append(args, os.Args...)
	cmd := exec.Command("sudo", args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func (d *Run) Run(ctx context.Context) error {
	if d.opts.HostsRouting && runtime.GOOS != "windows" && os.Geteuid() > 0 {
		return d.runSelfAsSudo()
	}

	err := d.init()
	if err != nil {
		return err
	}

	runInfo, err := d.prepareRun(d.cfg)
	if err != nil {
		return err
	}

	wg, err := d.start(ctx, runInfo)

	cleanupErr := d.cleanup()

	if err != nil {
		wg.Wait()

		if errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	}

	d.log.Println("Graceful shutdown...")

	wg.Wait()

	return cleanupErr
}

func (d *Run) runHTTPServer(ctx context.Context, routing map[*url.URL]*url.URL) error {
	var wg sync.WaitGroup

	errCh := make(chan error, 1)
	srv := d.newHTTPServer(routing)

	wg.Add(1)

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			errCh <- err
		}

		wg.Done()
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	_ = srv.Shutdown(ctx)

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}

		return err
	default:
	}

	return nil
}
