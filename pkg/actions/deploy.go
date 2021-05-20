package actions

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	"golang.org/x/sync/errgroup"
)

type deployParams struct {
	apps []*types.AppInfo
	deps []*types.DependencyInfo
}

type Deploy struct {
	log  logger.Logger
	opts DeployOptions
}

type DeployOptions struct {
	Verify bool
}

func NewDeploy(log logger.Logger, opts DeployOptions) *Deploy {
	return &Deploy{
		log: log,
	}
}

func (d *Deploy) Run(ctx context.Context, cfg *config.Project) error {
	// TODO: add support for local state

	stateRes, err := getState(ctx, cfg)
	if err != nil {
		return err
	}

	// TODO: show info about state being created stateRes.Source

	planMap, err := plan(ctx, stateRes.State, cfg.Apps, cfg.Dependencies, d.opts.Verify)
	if err != nil {
		return err
	}

	// TODO: prompt to accept

	applyMap, err := apply(ctx, planMap)
	if err != nil {
		return err
	}

	// TODO: save state
	fmt.Println(applyMap)

	return nil
}

func getState(ctx context.Context, cfg *config.Project) (*plugin_go.GetStateResponse, error) {
	state := cfg.State
	plug := state.Plugin()

	ret, err := plug.Client().GetState(ctx, state.Type, state.Env, state.Other, false /*TODO: change to true */, client.YAMLContext{
		Prefix: "$.state",
		Data:   cfg.YAMLData(),
	})
	if err != nil {
		return nil, fmt.Errorf("deploy: plugin '%s' get state error: %w", plug.Name, err)
	}

	return ret, nil
}

func plan(ctx context.Context, state *types.StateData, apps []config.App, deps map[string]*config.Dependency, verify bool) (map[*plugins.Plugin]*plugin_go.PlanResponse, error) {
	deployMap := make(map[*plugins.Plugin]*deployParams)

	for _, app := range apps {
		dnsPlugin := app.DNSPlugin()
		deployPlugin := app.DeployPlugin()
		appType := app.PluginType()

		includeDNS := dnsPlugin == nil || dnsPlugin == deployPlugin

		if _, ok := deployMap[deployPlugin]; !ok {
			deployMap[deployPlugin] = &deployParams{}
		}

		appReq := &types.AppInfo{
			IsDeploy: true,
			IsDNS:    includeDNS,
			App:      appType,
		}

		deployMap[deployPlugin].apps = append(deployMap[deployPlugin].apps, appReq)

		// Add DNS plugin if not already included (handled by same plugin).
		if includeDNS {
			continue
		}

		if _, ok := deployMap[dnsPlugin]; !ok {
			deployMap[dnsPlugin] = &deployParams{}
		}

		appReq = &types.AppInfo{
			IsDeploy: false,
			IsDNS:    true,
			App:      appType,
		}

		deployMap[dnsPlugin].apps = append(deployMap[dnsPlugin].apps, appReq)
	}

	for _, dep := range deps {
		t := dep.PluginType()

		p := dep.DeployPlugin()
		if _, ok := deployMap[p]; !ok {
			deployMap[p] = &deployParams{}
		}

		deployMap[p].deps = append(deployMap[p].deps, &types.DependencyInfo{Dependency: t})
	}

	planMap := make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(deployMap))

	g, _ := errgroup.WithContext(ctx)

	for plug, params := range deployMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state.PluginsMap[plug.Name], params.apps, params.deps, verify)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' plan error: %w", plug.Name, err)
			}

			planMap[plug] = ret

			return nil
		})
	}

	return planMap, g.Wait()
}

func apply(ctx context.Context, planMap map[*plugins.Plugin]*plugin_go.PlanResponse) (map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, error) {
	g, _ := errgroup.WithContext(ctx)
	applyMap := make(map[*plugins.Plugin]*plugin_go.ApplyDoneResponse, len(planMap))

	for plug, ret := range planMap {
		if ret.DeployPlan == nil {
			continue
		}

		ret := ret

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, ret.DeployPlan)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' apply error: %w", plug.Name, err)
			}

			applyMap[plug] = ret

			return nil
		})
	}

	// TODO: merge ret.Deploy (StateDeploy) and add it

	for plug, ret := range planMap {
		if ret.DNSPlan == nil {
			continue
		}

		ret := ret

		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, ret.DNSPlan)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' apply dns error: %w", plug.Name, err)
			}

			applyMap[plug] = ret

			return nil
		})
	}

	return applyMap, g.Wait()
}
