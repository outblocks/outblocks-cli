package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/outblocks/outblocks-cli/internal/urlutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
)

const deployCommand = "deploy"

type planParams struct {
	apps                 []*types.AppPlan
	deps                 []*types.DependencyPlan
	targetApps, skipApps []string
	args                 map[string]interface{}
	firstPass            bool
}

type Deploy struct {
	log  logger.Logger
	cfg  *config.Project
	opts *DeployOptions

	dockerCli *dockerclient.Client
	once      struct {
		dockerCli sync.Once
	}
}

type DeployOptions struct {
	Verify               bool
	Destroy              bool
	SkipBuild            bool
	Lock                 bool
	LockWait             time.Duration
	AutoApprove          bool
	TargetApps, SkipApps []string
	SkipAllApps          bool
}

func NewDeploy(log logger.Logger, cfg *config.Project, opts *DeployOptions) *Deploy {
	return &Deploy{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *Deploy) Run(ctx context.Context) error {
	verify := d.opts.Verify

	// Build apps.
	if !d.opts.SkipBuild && !d.opts.SkipAllApps {
		err := d.buildApps(ctx)
		if err != nil {
			return err
		}
	}

	// Get state.
	state, stateRes, err := getState(ctx, d.cfg, d.opts.Lock, d.opts.LockWait)
	if err != nil {
		return err
	}

	if stateRes.Source != nil && stateRes.Source.Created {
		d.log.Infof("New state created: '%s'\n", stateRes.Source.Name)

		verify = true
	}

	appStates, dependencyStates, canceled, err := d.planAndApply(ctx, verify, state, stateRes)

	// Release lock if needed.
	releaseErr := releaseLock(d.cfg, stateRes.LockInfo)

	switch {
	case err != nil:
		return err
	case releaseErr != nil:
		return releaseErr
	case canceled:
		return nil
	}

	return d.showStateStatus(appStates, dependencyStates)
}

func (d *Deploy) planAndApply(ctx context.Context, verify bool, state *types.StateData, stateRes *plugin_go.GetStateResponse) (appStates map[string]*types.AppState, dependencyStates map[string]*types.DependencyState, canceled bool, err error) {
	stateBeforeStr, _ := json.Marshal(state)

	// Plan and apply.
	apps, skipAppIDs, err := filterApps(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps)
	if err != nil {
		return nil, nil, false, err
	}

	deps := filterDependencies(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps)

	planMap, err := calculatePlanMap(d.cfg, apps, deps, d.opts.TargetApps, skipAppIDs)
	if err != nil {
		return nil, nil, false, err
	}

	// Start plugins.
	for plug := range planMap {
		err = plug.Client().Start(ctx)
		if err != nil {
			return nil, nil, false, err
		}
	}

	spinner, _ := d.log.Spinner().WithRemoveWhenDone(true).Start("Planning...")

	// Proceed with plan.
	planRetMap, appStates, dependencyStates, err := plan(ctx, state, planMap, verify, d.opts.Destroy)
	if err != nil {
		_ = spinner.Stop()
		_ = releaseLock(d.cfg, stateRes.LockInfo)

		return nil, nil, false, err
	}

	deployChanges, dnsChanges := computeChange(d.cfg, state, planRetMap)

	_ = spinner.Stop()

	empty, canceled := planPrompt(d.log, deployChanges, dnsChanges, d.opts.AutoApprove)

	shouldApply := !canceled && !empty

	start := time.Now()

	var saveErr error

	// Apply if needed.
	if shouldApply {
		callback := applyProgress(d.log, deployChanges, dnsChanges)
		appStates, dependencyStates, err = apply(context.Background(), state, planMap, d.opts.Destroy, callback)
	}

	// Save current apps/dependencies.
	state.Apps = make(map[string]*types.App)
	state.Dependencies = make(map[string]*types.Dependency)

	for _, app := range apps {
		state.Apps[app.ID] = app
	}

	for _, dep := range deps {
		state.Dependencies[dep.ID] = dep
	}

	stateAfterStr, _ := json.Marshal(state)
	shouldSave := !canceled && (!empty || !bytes.Equal(stateBeforeStr, stateAfterStr))

	if shouldSave {
		_, saveErr = saveState(d.cfg, state)
	}

	if shouldApply {
		d.log.Printf("All changes applied in %s.\n", time.Since(start).Truncate(timeTruncate))
	}

	if err == nil {
		err = saveErr
	}

	return appStates, dependencyStates, canceled, err
}

type dnsSetup struct {
	record string
	dns    *types.DNSState
}

func (d *Deploy) prepareAppDNSMap(appStates map[string]*types.AppState) (map[string]*types.DNSState, error) {
	dnsMap := make(map[string]*types.DNSState)

	for _, appState := range appStates {
		if appState.DNS == nil || !appState.DNS.Manual || (appState.DNS.CNAME == "" && appState.DNS.IP == "") {
			continue
		}

		host, err := urlutil.ExtractHostname(appState.DNS.URL)
		if err != nil {
			return nil, err
		}

		dnsMap[host] = appState.DNS
	}

	return dnsMap, nil
}

func (d *Deploy) showStateStatus(appStates map[string]*types.AppState, dependencyStates map[string]*types.DependencyState) error {
	dnsMap, err := d.prepareAppDNSMap(appStates)
	if err != nil {
		return err
	}

	var dns []*dnsSetup

	for host, v := range dnsMap {
		dns = append(dns, &dnsSetup{
			record: host,
			dns:    v,
		})
	}

	sort.Slice(dns, func(i, j int) bool {
		return dns[i].record < dns[j].record
	})

	// Show info about manual DNS setup.
	data := [][]string{
		{"Record", "Type", "Value"},
	}

	for _, v := range dns {
		typ := "A"
		val := v.dns.IP

		if v.dns.CNAME != "" {
			typ = "CNAME"
			val = v.dns.CNAME
		}

		data = append(data, []string{pterm.Green(v.record), pterm.Yellow(typ), val})
	}

	if len(dns) > 0 {
		d.log.Section().Println("DNS Setup (manual)")
		_ = d.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
	}

	// App Status.
	appURLStyle := pterm.NewStyle(pterm.FgGreen, pterm.Underscore)
	appURLErrorStyle := pterm.NewStyle(pterm.FgRed, pterm.Underscore)
	appNameStyle := pterm.NewStyle(pterm.Reset, pterm.Bold)
	appFailingStyle := pterm.NewStyle(pterm.FgRed, pterm.Bold)

	readyApps := make(map[string]*types.AppState)
	unreadyApps := make(map[string]*types.AppState)

	for k, appState := range appStates {
		if appState.Deployment == nil {
			continue
		}

		if appState.Deployment.Ready {
			readyApps[k] = appState
		} else {
			unreadyApps[k] = appState
		}
	}

	if len(readyApps) > 0 || len(unreadyApps) > 0 {
		d.log.Section().Println("App Status")

		for _, appState := range readyApps {
			app := appState.App

			d.log.Printf("%s %s %s (%s)\n", appURLStyle.Sprint(app.URL), pterm.Gray("==>"), appNameStyle.Sprint(app.Name), app.Type)
		}

		for _, appState := range unreadyApps {
			app := appState.App

			d.log.Printf("%s %s %s (%s) %s\n", appURLErrorStyle.Sprint(app.URL), pterm.Gray("==>"), appNameStyle.Sprint(app.Name), app.Type, appFailingStyle.Sprint("FAILING"))
			d.log.Errorln(appState.Deployment.Message)
		}
	}

	// Dependency Status.
	if len(dependencyStates) > 0 {
		d.log.Section().Println("Dependency Status")

		for _, depState := range dependencyStates {
			dep := depState.Dependency

			if depState.DNS == nil {
				continue
			}

			d.log.Printf("%s %s %s (%s)\n", pterm.Green(depState.DNS.ConnectionInfo), pterm.Gray("==>"), appNameStyle.Sprint(dep.Name), dep.Type)
		}
	}

	// Show info about SSL status.
	if len(dnsMap) > 0 {
		d.log.Section().Println("SSL Certificates")

		data := make([][]string, 0, len(dnsMap))

		for host, v := range dnsMap {
			data = append(data, []string{pterm.Green(host), pterm.Yellow(v.SSLStatus), v.SSLStatusInfo})
		}

		sort.Slice(data, func(i, j int) bool {
			return data[i][0] < data[j][0]
		})

		data = append([][]string{{"Domain", "Status", "Info"}}, data...)

		_ = d.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
	}

	return nil
}

func saveState(cfg *config.Project, data *types.StateData) (*plugin_go.SaveStateResponse, error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		return &plugin_go.SaveStateResponse{}, state.SaveLocal(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	return plug.Client().SaveState(ctx, data, state.Type, state.Other)
}

func getState(ctx context.Context, cfg *config.Project, lock bool, lockWait time.Duration) (stateData *types.StateData, stateRes *plugin_go.GetStateResponse, err error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		stateData, err = state.LoadLocal()
		if err != nil {
			return nil, nil, err
		}

		return stateData, &plugin_go.GetStateResponse{
			Source: &types.StateSource{},
		}, nil
	}

	ret, err := plug.Client().GetState(ctx, state.Type, state.Other, lock, lockWait, client.YAMLContext{
		Prefix: "$.state",
		Data:   cfg.YAMLData(),
	})
	if err != nil {
		return nil, nil, err
	}

	stateData = &types.StateData{}
	err = json.Unmarshal(ret.State, &stateData)

	if stateData == nil {
		stateData = &types.StateData{}
	}

	return stateData, ret, err
}
func filterApps(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps bool) (apps []*types.App, retSkipAppIDs []string, err error) {
	if skipAllApps {
		retSkipAppIDs := make([]string, 0, len(state.Apps))
		apps = make([]*types.App, 0, len(state.Apps))

		for _, app := range state.Apps {
			apps = append(apps, app)
			retSkipAppIDs = append(retSkipAppIDs, app.ID)
		}

		return apps, retSkipAppIDs, nil
	}

	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		apps = make([]*types.App, 0, len(cfg.Apps))
		for _, app := range cfg.Apps {
			apps = append(apps, app.PluginType())
		}

		return apps, nil, nil
	}

	appsMap := make(map[string]*types.App, len(state.Apps))
	targetAppIDsMap := util.StringArrayToSet(targetAppIDs)
	skipAppIDsMap := util.StringArrayToSet(skipAppIDs)

	// Use state apps as base unless skip mode is enabled and they are not to be skipped.
	for key, app := range state.Apps {
		if len(skipAppIDsMap) > 0 && !skipAppIDsMap[key] {
			continue
		}

		appsMap[key] = app
	}

	// Overwrite definition of non-skipped or target apps from project definition.
	for _, app := range cfg.Apps {
		if len(targetAppIDsMap) > 0 && !targetAppIDsMap[app.ID()] {
			continue
		}

		if skipAppIDsMap[app.ID()] {
			continue
		}

		appType := app.PluginType()
		appType.Properties = plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties, app.DeployInfo().Other)
		appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

		appsMap[app.ID()] = appType
	}

	// If there are any left target/skip apps without definition, that's an error.
	for app := range targetAppIDsMap {
		if appsMap[app] == nil {
			return nil, nil, fmt.Errorf("unknown target app specified: app with ID '%s' is missing definition", app)
		}
	}

	for app := range skipAppIDsMap {
		if appsMap[app] == nil {
			return nil, nil, fmt.Errorf("unknown skip app specified: app with ID '%s' is missing definition", app)
		}
	}

	// Flatten maps to list.
	apps = make([]*types.App, 0, len(appsMap))
	for _, app := range appsMap {
		apps = append(apps, app)
	}

	return apps, skipAppIDs, err
}

func filterDependencies(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps bool) (deps []*types.Dependency) {
	if skipAllApps {
		deps = make([]*types.Dependency, 0, len(state.Dependencies))
		for _, dep := range state.Dependencies {
			deps = append(deps, dep)
		}

		return deps
	}

	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		deps = make([]*types.Dependency, 0, len(cfg.Dependencies))
		for _, dep := range cfg.Dependencies {
			deps = append(deps, dep.PluginType())
		}

		return deps
	}

	dependenciesMap := make(map[string]*types.Dependency, len(state.Dependencies))

	// Always merge with dependencies from state.
	for key, dep := range state.Dependencies {
		dependenciesMap[key] = dep
	}

	for _, dep := range cfg.Dependencies {
		dependenciesMap[dep.ID()] = dep.PluginType()
	}

	// Flatten maps to list.
	deps = make([]*types.Dependency, 0, len(dependenciesMap))
	for _, dep := range dependenciesMap {
		deps = append(deps, dep)
	}

	return deps
}

func calculatePlanMap(cfg *config.Project, apps []*types.App, deps []*types.Dependency, targetAppIDs, skipAppIDs []string) (map[*plugins.Plugin]*planParams, error) {
	planMap := make(map[*plugins.Plugin]*planParams)

	for _, app := range apps {
		includeDNS := app.DNSPlugin != "" && app.DNSPlugin == app.DeployPlugin

		deployPlugin := cfg.FindLoadedPlugin(app.DeployPlugin)
		if deployPlugin == nil {
			return nil, fmt.Errorf("missing deploy plugin: %s used for app: %s", app.DeployPlugin, app.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		appReq := &types.AppPlan{
			IsDeploy: true,
			IsDNS:    includeDNS,
			App:      app,
		}

		planMap[deployPlugin].apps = append(planMap[deployPlugin].apps, appReq)
		planMap[deployPlugin].firstPass = !includeDNS // if dns is handled by different plugin, plan this as a first pass

		// Add DNS plugin if not already included (handled by same plugin).
		if includeDNS || app.DNSPlugin == "" {
			continue
		}

		dnsPlugin := cfg.FindLoadedPlugin(app.DNSPlugin)
		if dnsPlugin == nil {
			return nil, fmt.Errorf("missing dns plugin: %s used for app: %s", app.DNSPlugin, app.Name)
		}

		if _, ok := planMap[dnsPlugin]; !ok {
			planMap[dnsPlugin] = &planParams{
				args: dnsPlugin.CommandArgs(deployCommand),
			}
		}

		appReq = &types.AppPlan{
			IsDeploy: false,
			IsDNS:    true,
			App:      app,
		}

		planMap[dnsPlugin].apps = append(planMap[dnsPlugin].apps, appReq)
	}

	// Process dependencies.
	for _, dep := range deps {
		deployPlugin := cfg.FindLoadedPlugin(dep.DeployPlugin)
		if deployPlugin == nil {
			return nil, fmt.Errorf("missing deploy plugin: %s used for dependency: %s", dep.DeployPlugin, dep.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		planMap[deployPlugin].deps = append(planMap[deployPlugin].deps, &types.DependencyPlan{
			Dependency: dep,
		})
	}

	return addPlanTargetAndSkipApps(planMap, targetAppIDs, skipAppIDs), nil
}

func addPlanTargetAndSkipApps(planMap map[*plugins.Plugin]*planParams, targetAppIDs, skipAppIDs []string) map[*plugins.Plugin]*planParams {
	if len(targetAppIDs) == 0 && len(skipAppIDs) == 0 {
		return planMap
	}

	// Add target and skip app ids.
	targetAppIDsMap := util.StringArrayToSet(targetAppIDs)
	skipAppIDsMap := util.StringArrayToSet(skipAppIDs)

	for _, planParam := range planMap {
		for _, app := range planParam.apps {
			if targetAppIDsMap[app.App.ID] {
				planParam.targetApps = append(planParam.targetApps, app.App.ID)
			}

			if skipAppIDsMap[app.App.ID] {
				planParam.skipApps = append(planParam.skipApps, app.App.ID)
			}
		}
	}

	return planMap
}

func plan(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, verify, destroy bool) (retMap map[*plugins.Plugin]*plugin_go.PlanResponse, appStates map[string]*types.AppState, dependencyStates map[string]*types.DependencyState, err error) {
	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	appStates = make(map[string]*types.AppState)
	dependencyStates = make(map[string]*types.DependencyState)
	retMap = make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(planMap))

	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	// Plan all plugins concurrently.
	for plug, params := range planMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state, params.apps, params.deps, params.targetApps, params.skipApps, params.args, verify, destroy)
			if err != nil {
				return err
			}

			retMap[plug] = ret

			return nil
		})
	}

	err = g.Wait()

	// Merge state with new changes.
	for p, ret := range retMap {
		state.PluginsMap[p.Name] = ret.PluginMap

		for k, v := range ret.AppStates {
			appStates[k] = v
		}

		for k, v := range ret.DependencyStates {
			dependencyStates[k] = v
		}
	}

	return retMap, appStates, dependencyStates, err
}

func apply(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, destroy bool, callback func(*types.ApplyAction)) (appStates map[string]*types.AppState, dependencyStates map[string]*types.DependencyState, err error) {
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)
	retMap := make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))
	state.Apps = make(map[string]*types.App)
	state.Dependencies = make(map[string]*types.Dependency)

	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	appStates = make(map[string]*types.AppState)
	dependencyStates = make(map[string]*types.DependencyState)

	var mu sync.Mutex

	processResponse := func(plug *plugins.Plugin, ret *plugin_go.ApplyDoneResponse) {
		if ret == nil {
			return
		}

		mu.Lock()

		retMap[plug] = ret
		state.PluginsMap[plug.Name] = ret.PluginMap

		// Merge state with new changes.
		for k, v := range ret.AppStates {
			appStates[k] = v
		}

		for k, v := range ret.DependencyStates {
			dependencyStates[k] = v
		}

		mu.Unlock()
	}

	// Apply first pass plan (deployments without DNS).
	for plug, params := range planMap {
		if !params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.apps, params.deps, params.targetApps, params.skipApps, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, nil, err
	}

	// Apply second pass plan (DNS and deployments with DNS).
	retMap = make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	for plug, params := range planMap {
		if params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.apps, params.deps, params.targetApps, params.skipApps, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err = g.Wait()

	return appStates, dependencyStates, err
}
