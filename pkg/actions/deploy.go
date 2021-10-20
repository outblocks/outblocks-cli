package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/outblocks/outblocks-cli/internal/urlutil"
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
	Verify    bool
	Destroy   bool
	SkipBuild bool
	Lock      bool
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
	if !d.opts.SkipBuild {
		err := d.buildApps(ctx)
		if err != nil {
			return err
		}
	}

	// Get state.
	stateRes, err := getState(ctx, d.cfg, d.opts.Lock)
	if err != nil {
		return err
	}

	if stateRes.Source.Created {
		d.log.Infof("New state created: '%s'\n", stateRes.Source.Name)

		verify = true
	}

	stateBeforeStr, _ := json.Marshal(stateRes.State)

	// Plan and apply.
	planMap := calculatePlanMap(d.cfg)

	// Start plugins.
	for plug := range planMap {
		err = plug.Client().Start(ctx)
		if err != nil {
			return err
		}
	}

	spinner, _ := d.log.Spinner().WithRemoveWhenDone(true).Start("Planning...")

	planRetMap, err := plan(ctx, stateRes.State, planMap, verify, d.opts.Destroy)
	if err != nil {
		_ = spinner.Stop()
		_ = releaseLock(d.cfg, stateRes.LockInfo)

		return err
	}

	deployChanges, dnsChanges := computeChange(planRetMap)

	_ = spinner.Stop()

	stateAfterStr, _ := json.Marshal(stateRes.State)
	empty, canceled := planPrompt(d.log, deployChanges, dnsChanges)

	shouldApply := !canceled && !empty
	shouldSave := !canceled && (!empty || !bytes.Equal(stateBeforeStr, stateAfterStr))

	start := time.Now()

	var saveErr error

	if shouldApply {
		callback := applyProgress(d.log, deployChanges, dnsChanges)
		err = apply(ctx, stateRes.State, planMap, d.opts.Destroy, callback)
	}

	if shouldSave {
		_, saveErr = saveState(d.cfg, stateRes.State)
	}

	// Release lock if needed.
	releaseErr := releaseLock(d.cfg, stateRes.LockInfo)

	switch {
	case err != nil:
		return err
	case releaseErr != nil:
		return releaseErr
	default:
	}

	if shouldApply {
		d.log.Printf("All changes applied in %s.\n", time.Since(start).Truncate(timeTruncate))
	}

	err = d.showStateStatus(stateRes.State)
	if err != nil {
		return err
	}

	return saveErr
}

type dnsSetup struct {
	record string
	dns    *types.DNS
}

func (d *Deploy) showStateStatus(state *types.StateData) error {
	var dns []*dnsSetup

	dnsMap := make(map[string]*types.DNS)

	for _, app := range d.cfg.Apps {
		appState, ok := state.AppStates[app.ID()]
		if !ok || !appState.DNS.Manual || (appState.DNS.CNAME == "" && appState.DNS.IP == "") {
			continue
		}

		host, err := urlutil.ExtractHostname(appState.DNS.URL)
		if err != nil {
			return err
		}

		dnsMap[host] = appState.DNS
	}

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

	// External App URLs.
	var apps []config.App

	for _, app := range d.cfg.Apps {
		_, ok := state.AppStates[app.ID()]
		if !ok {
			continue
		}

		apps = append(apps, app)
	}

	appURLStyle := pterm.NewStyle(pterm.FgGreen, pterm.Underscore)
	appNameStyle := pterm.NewStyle(pterm.Reset, pterm.Bold)

	if len(apps) > 0 {
		d.log.Section().Println("App External URLs")

		for _, app := range apps {
			d.log.Printf("%s %s %s (%s)\n", appURLStyle.Sprint(app.URL()), pterm.Gray("==>"), appNameStyle.Sprint(app.Name()), app.Type())
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

func getState(ctx context.Context, cfg *config.Project, lock bool) (*plugin_go.GetStateResponse, error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		data, err := state.LoadLocal()
		if err != nil {
			return nil, err
		}

		return &plugin_go.GetStateResponse{
			State: data,
		}, nil
	}

	ret, err := plug.Client().GetState(ctx, state.Type, state.Other, lock, client.YAMLContext{
		Prefix: "$.state",
		Data:   cfg.YAMLData(),
	})
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func calculatePlanMap(cfg *config.Project) map[*plugins.Plugin]*planParams {
	planMap := make(map[*plugins.Plugin]*planParams)

	for _, app := range cfg.Apps {
		dnsPlugin := app.DNSPlugin()
		deployPlugin := app.DeployPlugin()
		appType := app.PluginType()
		deployProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, app.DeployInfo().Other)

		includeDNS := dnsPlugin != nil && dnsPlugin == deployPlugin

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs("deploy"),
			}
		}

		appReq := &types.AppPlan{
			IsDeploy:   true,
			IsDNS:      includeDNS,
			App:        appType,
			Env:        plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, app.Env(), app.DeployInfo().Env),
			Properties: deployProps,
		}

		planMap[deployPlugin].apps = append(planMap[deployPlugin].apps, appReq)
		planMap[deployPlugin].firstPass = !includeDNS // if dns is handled by different plugin, plan this as a first pass

		// Add DNS plugin if not already included (handled by same plugin).
		if includeDNS || dnsPlugin == nil {
			continue
		}

		if _, ok := planMap[dnsPlugin]; !ok {
			planMap[dnsPlugin] = &planParams{
				args: dnsPlugin.CommandArgs("deploy"),
			}
		}

		appReq = &types.AppPlan{
			IsDeploy:   false,
			IsDNS:      true,
			App:        appType,
			Properties: deployProps,
		}

		planMap[dnsPlugin].apps = append(planMap[dnsPlugin].apps, appReq)
	}

	// Process dependencies.
	for _, dep := range cfg.Dependencies {
		t := dep.PluginType()

		p := dep.DeployPlugin()
		if _, ok := planMap[p]; !ok {
			planMap[p] = &planParams{
				args: p.CommandArgs("deploy"),
			}
		}

		planMap[p].deps = append(planMap[p].deps, &types.DependencyPlan{
			Dependency: t,
		})
	}

	return planMap
}

func plan(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, verify, destroy bool) (map[*plugins.Plugin]*plugin_go.PlanResponse, error) {
	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	state.AppStates = make(map[string]*types.AppState)
	state.DependencyStates = make(map[string]*types.DependencyState)

	retMap := make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(planMap))

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

	err := g.Wait()

	// Merge state with new changes.
	for p, ret := range retMap {
		state.PluginsMap[p.Name] = ret.PluginMap

		for k, v := range ret.AppStates {
			state.AppStates[k] = v
		}

		for k, v := range ret.DependencyStates {
			state.DependencyStates[k] = v
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

	state.AppStates = make(map[string]*types.AppState)
	state.DependencyStates = make(map[string]*types.DependencyState)

	// Apply first pass plan (deployments without DNS).
	for plug, params := range planMap {
		if !params.firstPass {
			continue
		}

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.apps, params.deps, params.args, destroy, callback)
			if ret != nil {
				retMap[plug] = ret
				state.PluginsMap[plug.Name] = ret.PluginMap
			}

			return err
		})
	}

	err := g.Wait()

	// Merge state with new changes.
	for _, ret := range retMap {
		for k, v := range ret.AppStates {
			state.AppStates[k] = v
		}

		for k, v := range ret.DependencyStates {
			state.DependencyStates[k] = v
		}
	}

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
			if ret != nil {
				retMap[plug] = ret
				state.PluginsMap[plug.Name] = ret.PluginMap
			}

			return err
		})
	}

	err = g.Wait()

	// Merge state with new changes.
	for _, ret := range retMap {
		for k, v := range ret.AppStates {
			state.AppStates[k] = v
		}

		for k, v := range ret.DependencyStates {
			state.DependencyStates[k] = v
		}
	}

	return err
}
