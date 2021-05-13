package actions

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/cli"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	"golang.org/x/sync/errgroup"
)

type deployParams struct {
	apps []*types.App
	deps []*types.Dependency
}

type Deploy struct {
	ctx *cli.Context
	cfg *config.ProjectConfig
}

func NewDeploy(ctx *cli.Context, cfg *config.ProjectConfig) *Deploy {
	return &Deploy{
		ctx: ctx,
		cfg: cfg,
	}
}

func (d *Deploy) Run() error {
	planMap, err := plan(d.ctx.Ctx, d.cfg.Apps, d.cfg.Dependencies)
	if err != nil {
		return err
	}

	// TODO: prompt

	return apply(d.ctx.Ctx, planMap)
}

func plan(ctx context.Context, apps []config.App, deps map[string]*config.Dependency) (map[*plugins.Plugin]*plugin_go.PlanResponse, error) {
	deployMap := make(map[*plugins.Plugin]*deployParams)

	for _, app := range apps {
		t := app.PluginType()

		deployPlugin := app.DeployPlugin()
		if _, ok := deployMap[deployPlugin]; !ok {
			deployMap[deployPlugin] = &deployParams{}
		}

		deployMap[deployPlugin].apps = append(deployMap[deployPlugin].apps, t)

		// Add DNS plugin if needed.
		dnsPlugin := app.DNSPlugin()
		if dnsPlugin == nil || dnsPlugin == deployPlugin {
			continue
		}

		if _, ok := deployMap[dnsPlugin]; !ok {
			deployMap[dnsPlugin] = &deployParams{}
		}

		deployMap[dnsPlugin].apps = append(deployMap[dnsPlugin].apps, t)
	}

	for _, dep := range deps {
		t := dep.PluginType()

		p := dep.DeployPlugin()
		if _, ok := deployMap[p]; !ok {
			deployMap[p] = &deployParams{}
		}

		deployMap[p].deps = append(deployMap[p].deps, t)
	}

	planMap := make(map[*plugins.Plugin]*plugin_go.PlanResponse, len(deployMap))

	g, _ := errgroup.WithContext(ctx)

	for plug, params := range deployMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(params.apps, params.deps)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' plan error: %w", plug.Name, err)
			}

			planMap[plug] = ret

			return nil
		})
	}

	return planMap, g.Wait()
}

func apply(ctx context.Context, planMap map[*plugins.Plugin]*plugin_go.PlanResponse) error {
	g, _ := errgroup.WithContext(ctx)

	for plug, ret := range planMap {
		ret := ret

		g.Go(func() error {
			err := plug.Client().Apply(ret.Apps, ret.Dependencies)
			if err != nil {
				return fmt.Errorf("deploy: plugin '%s' apply error: %w", plug.Name, err)
			}

			return nil
		})
	}

	return g.Wait()
}
