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
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
	"github.com/pterm/pterm"
)

const deployCommand = "deploy"

type planParams struct {
	appPlans  []*apiv1.AppPlan
	depPlans  []*apiv1.DependencyPlan
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
	ForceApprove         bool
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

	if stateRes.StateCreated {
		d.log.Infof("New state created: '%s'\n", stateRes.StateName)

		verify = true
	}

	canceled, err := d.planAndApply(ctx, verify, state, stateRes)

	// Release lock if needed.
	releaseErr := releaseStateLock(d.cfg, stateRes.LockInfo)

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

func (d *Deploy) planAndApply(ctx context.Context, verify bool, state *types.StateData, stateRes *apiv1.GetStateResponse_State) (canceled bool, err error) {
	stateBeforeStr, _ := json.Marshal(state)

	// Plan and apply.
	appStates, skipAppIDs, destroy, err := filterApps(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps, d.opts.Destroy)
	if err != nil {
		return false, err
	}

	depStates := filterDependencies(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps)

	planMap, err := calculatePlanMap(d.cfg, appStates, depStates, d.opts.TargetApps, skipAppIDs)
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

	spinner, _ := d.log.Spinner().Start("Planning...")

	// Proceed with plan - reset state apps and deps.
	state.Apps = make(map[string]*apiv1.AppState)
	state.Dependencies = make(map[string]*apiv1.DependencyState)

	planRetMap, err := plan(ctx, state, planMap, verify, destroy)
	if err != nil {
		spinner.Stop()

		_ = releaseStateLock(d.cfg, stateRes.LockInfo)

		return false, err
	}

	deployChanges, dnsChanges := computeChange(d.cfg, state, planRetMap)

	spinner.Stop()

	empty, canceled := planPrompt(d.log, deployChanges, dnsChanges, d.opts.AutoApprove, d.opts.ForceApprove)

	shouldApply := !canceled && !empty

	start := time.Now()

	var saveErr error

	// Apply if needed.
	if shouldApply {
		callback := applyProgress(d.log, deployChanges, dnsChanges)
		err = apply(context.Background(), state, planMap, destroy, callback)
	}

	// Merge state with current apps/deps if needed (they might not have a state defined).
	for _, appState := range appStates {
		if _, ok := state.Apps[appState.App.Id]; ok {
			continue
		}

		state.Apps[appState.App.Id] = &apiv1.AppState{App: appState.App}
	}

	for _, depState := range depStates {
		if _, ok := state.Dependencies[depState.Dependency.Id]; ok {
			continue
		}

		state.Dependencies[depState.Dependency.Id] = &apiv1.DependencyState{Dependency: depState.Dependency}
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
	dns    *apiv1.DNSState
}

func (d *Deploy) prepareAppDNSMap(appStates map[string]*apiv1.AppState) (map[string]*apiv1.DNSState, error) {
	dnsMap := make(map[string]*apiv1.DNSState)

	for _, appState := range appStates {
		if appState.Dns == nil || !appState.Dns.Manual || (appState.Dns.Cname == "" && appState.Dns.Ip == "") {
			continue
		}

		host, err := urlutil.ExtractHostname(appState.Dns.Url)
		if err != nil {
			return nil, err
		}

		dnsMap[host] = appState.Dns
	}

	return dnsMap, nil
}

func (d *Deploy) showStateStatus(appStates map[string]*apiv1.AppState, dependencyStates map[string]*apiv1.DependencyState) error {
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
		val := v.dns.Ip

		if v.dns.Cname != "" {
			typ = "CNAME"
			val = v.dns.Cname
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

	readyApps := make(map[string]*apiv1.AppState)
	unreadyApps := make(map[string]*apiv1.AppState)

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

			d.log.Printf("%s %s %s (%s)\n", appURLStyle.Sprint(app.Url), pterm.Gray("==>"), appNameStyle.Sprint(app.Name), app.Type)
		}

		for _, appState := range unreadyApps {
			app := appState.App

			d.log.Printf("%s %s %s (%s) %s\n", appURLErrorStyle.Sprint(app.Url), pterm.Gray("==>"), appNameStyle.Sprint(app.Name), app.Type, appFailingStyle.Sprint("FAILING"))
			d.log.Errorln(appState.Deployment.Message)
		}
	}

	// Dependency Status.
	if len(dependencyStates) > 0 {
		first := true

		for _, depState := range dependencyStates {
			dep := depState.Dependency

			if depState.Dns == nil {
				continue
			}

			if first {
				d.log.Section().Println("Dependency Status")

				first = false
			}

			d.log.Printf("%s %s %s (%s)\n", pterm.Green(depState.Dns.ConnectionInfo), pterm.Gray("==>"), appNameStyle.Sprint(dep.Name), dep.Type)
		}
	}

	// Show info about SSL status.
	if len(dnsMap) > 0 {
		d.log.Section().Println("SSL Certificates")

		data := make([][]string, 0, len(dnsMap))

		for host, v := range dnsMap {
			data = append(data, []string{pterm.Green(host), pterm.Yellow(v.SslStatus), v.SslStatusInfo})
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

func saveState(cfg *config.Project, data *types.StateData) (*apiv1.SaveStateResponse, error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		return &apiv1.SaveStateResponse{}, state.SaveLocal(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	return plug.Client().SaveState(ctx, data, state.Type, state.Other)
}

func getState(ctx context.Context, cfg *config.Project, lock bool, lockWait time.Duration) (stateData *types.StateData, stateRes *apiv1.GetStateResponse_State, err error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		stateData, err = state.LoadLocal()
		if err != nil {
			return nil, nil, err
		}

		return stateData, &apiv1.GetStateResponse_State{}, nil
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

func calculatePlanMap(cfg *config.Project, appStates []*apiv1.AppState, depStates []*apiv1.DependencyState, targetAppIDs, skipAppIDs []string) (map[*plugins.Plugin]*planParams, error) {
	planMap := make(map[*plugins.Plugin]*planParams)

	for _, appState := range appStates {
		includeDNS := appState.App.DnsPlugin != "" && appState.App.DnsPlugin == appState.App.DeployPlugin

		deployPlugin := cfg.FindLoadedPlugin(appState.App.DeployPlugin)
		if deployPlugin == nil {
			return nil, fmt.Errorf("missing deploy plugin: %s used for app: %s", appState.App.DeployPlugin, appState.App.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		appReq := &apiv1.AppPlan{
			State:    appState,
			IsDeploy: true,
			IsDns:    includeDNS,
		}

		planMap[deployPlugin].appPlans = append(planMap[deployPlugin].appPlans, appReq)
		planMap[deployPlugin].firstPass = !includeDNS // if dns is handled by different plugin, plan this as a first pass

		// Add DNS plugin if not already included (handled by same plugin).
		if includeDNS || appState.App.DnsPlugin == "" {
			continue
		}

		dnsPlugin := cfg.FindLoadedPlugin(appState.App.DnsPlugin)
		if dnsPlugin == nil {
			return nil, fmt.Errorf("missing dns plugin: %s used for app: %s", appState.App.DnsPlugin, appState.App.Name)
		}

		if _, ok := planMap[dnsPlugin]; !ok {
			planMap[dnsPlugin] = &planParams{
				args: dnsPlugin.CommandArgs(deployCommand),
			}
		}

		appReq = &apiv1.AppPlan{
			State:    appState,
			IsDeploy: false,
			IsDns:    true,
		}

		planMap[dnsPlugin].appPlans = append(planMap[dnsPlugin].appPlans, appReq)
	}

	// Process dependencies.
	for _, depState := range depStates {
		deployPlugin := cfg.FindLoadedPlugin(depState.Dependency.DeployPlugin)
		if deployPlugin == nil {
			return nil, fmt.Errorf("missing deploy plugin: %s used for dependency: %s", depState.Dependency.DeployPlugin, depState.Dependency.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		planMap[deployPlugin].depPlans = append(planMap[deployPlugin].depPlans, &apiv1.DependencyPlan{
			State: depState,
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
		for _, appPlan := range planParam.appPlans {
			if skipAppIDsMap[appPlan.State.App.Id] || (len(targetAppIDsMap) > 0 && !targetAppIDsMap[appPlan.State.App.Id]) {
				appPlan.Skip = true
			}
		}
	}

	return planMap
}

func mergeState(state *types.StateData, pluginName string, pluginState *apiv1.PluginState, appStates map[string]*apiv1.AppState, depStates map[string]*apiv1.DependencyState) {
	state.Plugins[pluginName] = types.PluginStateFromProto(pluginState)

	// Merge state with new changes.
	for k, v := range appStates {
		state.Apps[k] = v
	}

	for k, v := range depStates {
		state.Dependencies[k] = v
	}
}

func plan(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, verify, destroy bool) (retMap map[*plugins.Plugin]*apiv1.PlanResponse, err error) {
	if state.Plugins == nil {
		state.Plugins = make(map[string]*types.PluginState)
	}

	retMap = make(map[*plugins.Plugin]*apiv1.PlanResponse, len(planMap))
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	var mu sync.Mutex

	processResponse := func(plug *plugins.Plugin, ret *apiv1.PlanResponse) {
		if ret == nil {
			return
		}

		mu.Lock()

		retMap[plug] = ret

		// Merge state with new changes.
		mergeState(state, plug.Name, ret.State, ret.AppStates, ret.DependencyStates)

		mu.Unlock()
	}

	// Plan all plugins concurrently.
	for plug, params := range planMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state, params.appPlans, params.depPlans, params.args, verify, destroy)
			if err != nil {
				return err
			}

			processResponse(plug, ret)

			return nil
		})
	}

	err = g.Wait()

	// Merge state with new changes.
	for p, ret := range retMap {
		mergeState(state, p.Name, ret.State, ret.AppStates, ret.DependencyStates)
	}

	return retMap, err
}

func apply(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, destroy bool, callback func(*apiv1.ApplyAction)) error {
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	if state.Plugins == nil {
		state.Plugins = make(map[string]*types.PluginState)
	}

	var mu sync.Mutex

	processResponse := func(plug *plugins.Plugin, ret *apiv1.ApplyDoneResponse) {
		if ret == nil {
			return
		}

		mu.Lock()

		// Merge state with new changes.
		mergeState(state, plug.Name, ret.State, ret.AppStates, ret.DependencyStates)

		mu.Unlock()
	}

	// Apply first pass plan (deployments without DNS).
	for plug, params := range planMap {
		if !params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.appPlans, params.depPlans, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err := g.Wait()
	if err != nil {
		return err
	}

	// Apply second pass plan (DNS and deployments with DNS).
	for plug, params := range planMap {
		if params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.appPlans, params.depPlans, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	err = g.Wait()

	return err
}
