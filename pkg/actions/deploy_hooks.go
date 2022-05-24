package actions

import (
	"context"
	"sync"

	"github.com/outblocks/outblocks-cli/internal/statefile"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/util/errgroup"
)

type deployHookRes interface {
	GetState() *apiv1.PluginState
	GetAppStates() map[string]*apiv1.AppState
	GetDependencyStates() map[string]*apiv1.DependencyState
	GetDomains() []*apiv1.DomainInfo
	GetDnsRecords() []*apiv1.DNSRecord
}

func deployHookResponseCallback(state *statefile.StateData) func(plug *plugins.Plugin, ret deployHookRes) {
	var mu sync.Mutex

	return func(plug *plugins.Plugin, ret deployHookRes) {
		if ret == nil {
			return
		}

		mu.Lock()

		// Merge state with new changes.
		mergeState(state, plug.Name, ret.GetState(), ret.GetAppStates(), ret.GetDependencyStates(), ret.GetDomains(), ret.GetDnsRecords())

		mu.Unlock()
	}
}

func (d *Deploy) genericDeployHook(ctx context.Context, state *statefile.StateData, f func(plug *plugins.Plugin) (deployHookRes, error)) error {
	g, _ := errgroup.WithConcurrency(ctx, defaultConcurrency)

	processResponse := deployHookResponseCallback(state)

	for _, plug := range d.cfg.LoadedPlugins() {
		if !plug.HasAction(plugins.ActionDeployHook) {
			continue
		}

		g.Go(func() error {
			ret, err := f(plug)
			if err != nil {
				return err
			}

			processResponse(plug, ret)

			return nil
		})
	}

	err := g.Wait()

	return err
}

func (d *Deploy) prePlanHook(ctx context.Context, state *statefile.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, verify, destroy bool) error {
	return d.genericDeployHook(ctx, state, func(plug *plugins.Plugin) (deployHookRes, error) {
		return plug.Client().DeployHook(ctx, apiv1.DeployHookRequest_STAGE_PRE_PLAN, state, apps, deps, plug.CommandArgs(deployCommand), verify, destroy)
	})
}

func (d *Deploy) preApplyHook(ctx context.Context, state *statefile.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, verify, destroy bool) error {
	return d.genericDeployHook(ctx, state, func(plug *plugins.Plugin) (deployHookRes, error) {
		return plug.Client().DeployHook(ctx, apiv1.DeployHookRequest_STAGE_PRE_APPLY, state, apps, deps, plug.CommandArgs(deployCommand), verify, destroy)
	})
}

func (d *Deploy) postApplyHook(ctx context.Context, state *statefile.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, verify, destroy bool) error {
	return d.genericDeployHook(ctx, state, func(plug *plugins.Plugin) (deployHookRes, error) {
		return plug.Client().DeployHook(ctx, apiv1.DeployHookRequest_STAGE_POST_APPLY, state, apps, deps, plug.CommandArgs(deployCommand), verify, destroy)
	})
}

func (d *Deploy) postDeployHook(ctx context.Context, state *statefile.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, verify, destroy bool) error {
	return d.genericDeployHook(ctx, state, func(plug *plugins.Plugin) (deployHookRes, error) {
		return plug.Client().DeployHook(ctx, apiv1.DeployHookRequest_STAGE_POST_DEPLOY, state, apps, deps, plug.CommandArgs(deployCommand), verify, destroy)
	})
}
