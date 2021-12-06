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
	"strings"
	"sync"
	"time"

	"github.com/outblocks/outblocks-cli/internal/urlutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/actions/run"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
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
	apps []*apiv1.AppRun
	deps []*apiv1.DependencyRun

	localApps     []*run.LocalApp
	localDeps     []*run.LocalDependency
	pluginAppsMap map[*plugins.Plugin]*apiv1.RunRequest
	pluginDepsMap map[*plugins.Plugin]*apiv1.RunRequest
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
		pluginAppsMap: make(map[*plugins.Plugin]*apiv1.RunRequest),
		pluginDepsMap: make(map[*plugins.Plugin]*apiv1.RunRequest),
	}

	loopbackHost := d.loopbackHost()
	hosts := map[string]string{
		loopbackHost: loopbackIP,
	}

	// Apps.
	port := d.opts.ListenPort + 1

	for _, app := range cfg.Apps {
		if app.RunInfo().Command == "" {
			return nil, app.YAMLError("$.run.command", "run.command is required to run app")
		}

		runInfo := app.RunInfo()

		appPort := int(runInfo.Port)
		if appPort == 0 {
			appPort = port
			port++
		}

		appType := app.Proto()
		mergedProps := plugin_util.MergeMaps(cfg.Defaults.Run.Other, appType.Properties.AsMap(), runInfo.Other)
		appType.Env = plugin_util.MergeStringMaps(appType.Env, runInfo.Env)
		appType.Properties = plugin_util.MustNewStruct(mergedProps)

		appRun := &apiv1.AppRun{
			App:  appType,
			Url:  d.localURL(app.URL(), appPort, app.PathRedirect()),
			Ip:   loopbackIP,
			Port: int32(appPort),
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
			info.pluginAppsMap[runPlugin] = &apiv1.RunRequest{
				Args:  plugin_util.MustNewStruct(runPlugin.CommandArgs(runCommand)),
				Hosts: hosts,
			}
		}

		info.pluginAppsMap[runPlugin].Apps = append(info.pluginAppsMap[runPlugin].Apps, appRun)
	}

	// Dependencies.
	for _, dep := range cfg.Dependencies {
		depType := dep.Proto()
		depPort := dep.Run.Port

		if depPort == 0 {
			depPort = port
			port++
		}

		mergedProps := plugin_util.MergeMaps(cfg.Defaults.Run.Other, depType.Properties.AsMap())
		depType.Properties = plugin_util.MustNewStruct(mergedProps)

		depRun := &apiv1.DependencyRun{
			Dependency: depType,
			Ip:         loopbackIP,
			Port:       int32(depPort),
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
			info.pluginDepsMap[runPlugin] = &apiv1.RunRequest{
				Args: plugin_util.MustNewStruct(runPlugin.CommandArgs(runCommand)),
			}
		}

		info.pluginDepsMap[runPlugin].Dependencies = append(info.pluginDepsMap[runPlugin].Dependencies, depRun)
	}

	// Gather hosts.
	for _, app := range info.apps {
		host, _ := urlutil.ExtractHostname(app.Url)
		hosts[host] = app.Ip
	}

	// Fill envs per app.
	for _, app := range info.apps {
		app.App.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, app.App.Env)
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

	// Process env vars.
	depVars := make(map[string]map[string]string)

	for _, ret := range pluginRets {
		for _, i := range ret.Info {
			for k, v := range i.Response.Vars {
				depVars[d.cfg.DependencyByID(k).Name] = v.Vars
			}
		}
	}

	var err error

	appVars := types.AppVarsFromAppRun(runInfo.apps)

	for _, app := range runInfo.apps {
		eval := util.NewVarEvaluator(types.VarsForApp(appVars, app.App, depVars))

		app.App.Env, err = eval.ExpandStringMap(app.App.Env)
		if err != nil {
			return nil, nil, err
		}
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

		req, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("http://%s:%d/", app.Ip, app.Port), nil)
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

func formatRunOutput(log logger.Logger, cfg *config.Project, r *apiv1.RunOutputResponse) {
	var prefix string

	msg := plugin_util.StripAnsiControl(r.Message)

	switch r.Source {
	case apiv1.RunOutputResponse_SOURCE_APP:
		app := cfg.AppByID(r.Id)
		prefix = fmt.Sprintf("APP:%s:%s:", app.Type(), app.Name())

	case apiv1.RunOutputResponse_SOURCE_DEPENDENCY:
		prefix = fmt.Sprintf("DEP:%s", r.Name)

	case apiv1.RunOutputResponse_SOURCE_UNSPECIFIED:
		prefix = fmt.Sprintf("UNKNOWN:%s", r.Name)
	}

	if r.Stream == apiv1.RunOutputResponse_STREAM_STDERR {
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
		u, _ := url.Parse(s.Url)
		hosts[u.Hostname()] = struct{}{}

		uLocal := *u
		uLocal.Host = fmt.Sprintf("%s:%d", s.Ip, s.Port)
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
	d.log.Successln("All apps are UP.")

	for _, a := range runInfo.apps {
		d.log.Printf("%s App '%s' listening at %s\n", strings.Title(a.App.Type), a.App.Name, a.Url)
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
