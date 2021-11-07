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
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
)

const deployCommand = "deploy"

type planParams struct {
	apps      []*types.AppPlan
	deps      []*types.DependencyPlan
	args      map[string]interface{}
	firstPass bool
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

	canceled, err := d.planAndApply(ctx, verify, state, stateRes)

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

	return d.showStateStatus(state.Apps, state.Dependencies)
}

func (d *Deploy) planAndApply(ctx context.Context, verify bool, state *types.StateData, stateRes *plugin_go.GetStateResponse) (canceled bool, err error) {
	stateBeforeStr, _ := json.Marshal(state)

	// Plan and apply.
	apps, skipAppIDs, destroy, err := filterApps(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps, d.opts.Destroy)
	if err != nil {
		return false, err
	}

	deps := filterDependencies(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps)

	planMap, err := calculatePlanMap(d.cfg, apps, deps, d.opts.TargetApps, skipAppIDs)
	if err != nil {
		return false, err
	}

	// Start plugins.
	for plug := range planMap {
		err = plug.Client().Start(ctx)
		if err != nil {
			return false, err
		}
	}

	spinner, _ := d.log.Spinner().WithRemoveWhenDone(true).Start("Planning...")

	// Proceed with plan - reset state apps and deps.
	state.Apps = make(map[string]*types.AppState)
	state.Dependencies = make(map[string]*types.DependencyState)

	planRetMap, err := plan(ctx, state, planMap, verify, destroy)
	if err != nil {
		_ = spinner.Stop()
		_ = releaseLock(d.cfg, stateRes.LockInfo)

		return false, err
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
		err = apply(context.Background(), state, planMap, destroy, callback)
	}

	// Merge state with current apps/deps if needed (they might not have a state defined).
	for _, app := range apps {
		if _, ok := state.Apps[app.ID]; ok {
			continue
		}

		state.Apps[app.ID] = types.NewAppState(&app.App)
	}

	for _, dep := range deps {
		if _, ok := state.Dependencies[dep.ID]; ok {
			continue
		}

		state.Dependencies[dep.ID] = types.NewDependencyState(&dep.Dependency)
	}

	// Proceed with saving.
	stateAfterStr, _ := json.Marshal(state)
	shouldSave := !canceled && (!empty || !bytes.Equal(stateBeforeStr, stateAfterStr))

	if shouldSave {
		_, saveErr = saveState(d.cfg, state)
	}

	if shouldApply && err == nil {
		d.log.Printf("All changes applied in %s.\n", time.Since(start).Truncate(timeTruncate))
	}

	if err == nil {
		err = saveErr
	}

	return canceled, err
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
		first := true

		for _, depState := range dependencyStates {
			dep := depState.Dependency

			if depState.DNS == nil {
				continue
			}

			if first {
				d.log.Section().Println("Dependency Status")

				first = false
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

	if len(unreadyApps) > 0 {
		return fmt.Errorf("not all apps are ready")
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

func calculatePlanMap(cfg *config.Project, apps []*types.AppState, deps []*types.DependencyState, targetAppIDs, skipAppIDs []string) (map[*plugins.Plugin]*planParams, error) {
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
			App:      app,
			IsDeploy: true,
			IsDNS:    includeDNS,
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
			App:      app,
			IsDeploy: false,
			IsDNS:    true,
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
			if skipAppIDsMap[app.App.ID] || (len(targetAppIDsMap) > 0 && !targetAppIDsMap[app.App.ID]) {
				app.Skip = true
			}
		}
	}

	return planMap
}

func plan(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, verify, destroy bool) (retMap map[*plugins.Plugin]*plugin_go.PlanResponse, err error) {
	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	retMap = make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(planMap))

	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	// Plan all plugins concurrently.
	for plug, params := range planMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state, params.apps, params.deps, params.args, verify, destroy)
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
			// TODO: validate app state returned
			state.Apps[k] = v
		}

		for k, v := range ret.DependencyStates {
			state.Dependencies[k] = v
		}
	}

	return retMap, err
}

func apply(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, destroy bool, callback func(*types.ApplyAction)) error {
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)
	retMap := make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	appStates := state.Apps
	dependencyStates := state.Dependencies

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
			ret, err := plug.Client().Apply(ctx, state, params.apps, params.deps, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err := g.Wait()
	if err != nil {
		return err
	}

	// Apply second pass plan (DNS and deployments with DNS).
	retMap = make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	for plug, params := range planMap {
		if params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.apps, params.deps, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err = g.Wait()

	return err
}
