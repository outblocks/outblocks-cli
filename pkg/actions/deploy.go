package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
)

type planParams struct {
	apps []*types.AppPlan
	deps []*types.DependencyPlan
}

type Deploy struct {
	log  logger.Logger
	opts DeployOptions
}

type DeployOptions struct {
	Verify  bool
	Destroy bool
}

func NewDeploy(log logger.Logger, opts DeployOptions) *Deploy {
	return &Deploy{
		log:  log,
		opts: opts,
	}
}

func (d *Deploy) Run(ctx context.Context, cfg *config.Project) error {
	verify := d.opts.Verify
	spinner, _ := d.log.Spinner().WithRemoveWhenDone(true).Start("Getting state...")

	stateRes, err := getState(ctx, cfg)
	if err != nil {
		_ = spinner.Stop()
		return err
	}

	_ = spinner.Stop()

	if stateRes.Source.Created {
		d.log.Infof("New state created: '%s'\n", stateRes.Source.Name)

		verify = true
	}

	spinner, _ = spinner.Start("Planning...")

	planMap, err := plan(ctx, stateRes.State, cfg.Apps, cfg.Dependencies, verify, d.opts.Destroy)
	if err != nil {
		_ = spinner.Stop()
		return err
	}

	deployChanges, dnsChanges := computeChange(planMap)

	_ = spinner.Stop()

	proceed := planPrompt(d.log, deployChanges, dnsChanges)
	if !proceed {
		return nil
	}

	start := time.Now()

	callback := applyProgress(d.log, deployChanges, dnsChanges)
	state, err := apply(ctx, stateRes.State, planMap, callback)

	_, saveErr := saveState(ctx, cfg, state)

	var releaseErr error

	// Release lock if needed.
	if stateRes.LockInfo != "" {
		releaseErr = releaseLock(ctx, cfg, stateRes.LockInfo)
	}

	switch {
	case err != nil:
		return err
	case releaseErr != nil:
		return releaseErr
	default:
	}

	d.log.Infof("All changes applied in %s.\n", time.Since(start).Truncate(timeTruncate))

	return saveErr
}

func saveState(ctx context.Context, cfg *config.Project, data *types.StateData) (*plugin_go.SaveStateResponse, error) {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		return &plugin_go.SaveStateResponse{}, state.SaveLocal(data)
	}

	return plug.Client().SaveState(ctx, data, state.Type, state.Env, state.Other)
}

func getState(ctx context.Context, cfg *config.Project) (*plugin_go.GetStateResponse, error) {
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

	ret, err := plug.Client().GetState(ctx, state.Type, state.Env, state.Other, false /*TODO: change to true */, client.YAMLContext{
		Prefix: "$.state",
		Data:   cfg.YAMLData(),
	})
	if err != nil {
		return nil, fmt.Errorf("deploy: plugin '%s' get state error: %w", plug.Name, err)
	}

	return ret, nil
}

func plan(ctx context.Context, state *types.StateData, apps []config.App, deps map[string]*config.Dependency, verify, destroy bool) (map[*plugins.Plugin]*plugin_go.PlanResponse, error) {
	planMap := make(map[*plugins.Plugin]*planParams)

	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	if state.AppStates == nil {
		state.AppStates = make(map[string]*types.AppState)
	}

	if state.DependencyStates == nil {
		state.DependencyStates = make(map[string]*types.DependencyState)
	}

	for _, app := range apps {
		dnsPlugin := app.DNSPlugin()
		deployPlugin := app.DeployPlugin()
		appType := app.PluginType()

		includeDNS := dnsPlugin != nil && dnsPlugin == deployPlugin

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{}
		}

		appReq := &types.AppPlan{
			IsDeploy: true,
			IsDNS:    includeDNS,
			App:      appType,
		}

		planMap[deployPlugin].apps = append(planMap[deployPlugin].apps, appReq)

		// Add DNS plugin if not already included (handled by same plugin).
		if includeDNS || dnsPlugin == nil {
			continue
		}

		if _, ok := planMap[dnsPlugin]; !ok {
			planMap[dnsPlugin] = &planParams{}
		}

		appReq = &types.AppPlan{
			IsDeploy: false,
			IsDNS:    true,
			App:      appType,
		}

		planMap[dnsPlugin].apps = append(planMap[dnsPlugin].apps, appReq)
	}

	for _, dep := range deps {
		t := dep.PluginType()

		p := dep.DeployPlugin()
		if _, ok := planMap[p]; !ok {
			planMap[p] = &planParams{}
		}

		planMap[p].deps = append(planMap[p].deps, &types.DependencyPlan{Dependency: t})
	}

	retMap := make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(planMap))

	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	// Plan all plugins concurrently.
	for plug, params := range planMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state, params.apps, params.deps, verify, destroy)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' plan error: %w", plug.Name, err)
			}

			retMap[plug] = ret
			if verify {
				state.PluginsMap[plug.Name] = ret.PluginMap
			}

			return nil
		})
	}

	err := g.Wait()

	// Merge state with new changes.
	if verify {
		for _, ret := range retMap {
			for k, v := range ret.AppStates {
				state.AppStates[k] = v
			}

			for k, v := range ret.DependencyStates {
				state.DependencyStates[k] = v
			}
		}
	}

	return retMap, err
}

func apply(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*plugin_go.PlanResponse, callback func(*types.ApplyAction)) (*types.StateData, error) {
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)
	retMap := make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	if state.PluginsMap == nil {
		state.PluginsMap = make(map[string]types.PluginStateMap)
	}

	if state.AppStates == nil {
		state.AppStates = make(map[string]*types.AppState)
	}

	if state.DependencyStates == nil {
		state.DependencyStates = make(map[string]*types.DependencyState)
	}

	for plug, ret := range planMap {
		if ret.DeployPlan == nil {
			continue
		}

		ret := ret

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, ret.DeployPlan, nil, callback)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' apply error: %w", plug.Name, err)
			}

			retMap[plug] = ret
			state.PluginsMap[plug.Name] = ret.PluginMap

			return nil
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
		return state, err
	}

	// Apply DNS plan.
	retMap = make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	for plug, ret := range planMap {
		if ret.DNSPlan == nil {
			continue
		}

		ret := ret

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, nil, ret.DNSPlan, callback)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' apply dns error: %w", plug.Name, err)
			}

			retMap[plug] = ret

			return nil
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

	return state, err
}
