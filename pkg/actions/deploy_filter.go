package actions

import (
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func filterAppsNormal(cfg *config.Project, state *statefile.StateData, targets, skips *util.TargetMatcher) map[string]*apiv1.AppPlan {
	appsMap := make(map[string]*apiv1.AppPlan, len(state.Apps))

	// Use state apps as base unless skip mode is enabled and they are NOT to be skipped.
	for key, app := range state.Apps {
		if !skips.IsEmpty() && skips.Matches(key) {
			continue
		}

		appsMap[key] = &apiv1.AppPlan{
			State: app,
		}
	}

	// Overwrite definition of non-skipped or target apps from project definition.
	for _, app := range cfg.Apps {
		if !targets.IsEmpty() && !skips.IsEmpty() {
			continue
		}

		if skips.Matches(app.ID()) {
			continue
		}

		appType := app.Proto()
		mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

		appType.Properties = plugin_util.MustNewStruct(mergedProps)
		appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Deploy.Env, appType.Env, app.DeployInfo().Env)

		appsMap[app.ID()] = &apiv1.AppPlan{
			State: &apiv1.AppState{
				App: appType,
			},
			Build: app.BuildProto(),
		}
	}

	return appsMap
}

func filterAppsDestroy(state *statefile.StateData, targets, skips *util.TargetMatcher) map[string]*apiv1.AppPlan {
	appsMap := make(map[string]*apiv1.AppPlan, len(state.Apps))

	// Use state apps as base unless skip mode is enabled and they are NOT to be skipped or they are targeted.
	for key, app := range state.Apps {
		if !skips.IsEmpty() && !skips.Matches(key) {
			continue
		}

		// Add app to skip IDs as we don't want to deal with it's changes.
		_ = skips.Add(key)
		skips.Matches(key)

		if !targets.IsEmpty() && targets.Matches(key) {
			targets.Remove(key)

			continue
		}

		appsMap[key] = &apiv1.AppPlan{
			State: app,
		}
	}

	return appsMap
}

func filterApps(cfg *config.Project, state *statefile.StateData, targets, skips *util.TargetMatcher, skipAllApps, destroy bool) (apps []*apiv1.AppPlan, retSkips *util.TargetMatcher, retDestroy bool, err error) {
	if skipAllApps {
		retSkips := util.NewTargetMatcher()
		apps = make([]*apiv1.AppPlan, 0, len(state.Apps))

		for _, appState := range state.Apps {
			apps = append(apps, &apiv1.AppPlan{
				State: appState,
			})

			_ = retSkips.AddApp(appState.App.Id)
		}

		return apps, retSkips, destroy, nil
	}

	// In non target and non skip mode, use config apps and deps.
	if targets.IsEmpty() && skips.IsEmpty() {
		apps = make([]*apiv1.AppPlan, 0, len(cfg.Apps))

		for _, app := range cfg.Apps {
			appType := app.Proto()
			mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

			appType.Properties = plugin_util.MustNewStruct(mergedProps)
			appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Deploy.Env, appType.Env, app.DeployInfo().Env)

			apps = append(apps, &apiv1.AppPlan{
				State: &apiv1.AppState{
					App: appType,
				},
				Build: app.BuildProto(),
			})
		}

		return apps, skips, destroy, nil
	}

	var appsMap map[string]*apiv1.AppPlan

	if destroy {
		// Destroy mode.
		appsMap = filterAppsDestroy(state, targets, skips)
	} else {
		// Normal mode.
		appsMap = filterAppsNormal(cfg, state, targets, skips)
	}

	// If there are any left target/skip apps without definition, that's an error.
	for _, t := range targets.Unmatched() {
		return nil, nil, false, merry.Errorf("unknown target specified: '%s' is missing definition", t.Input())
	}

	for _, t := range skips.Unmatched() {
		return nil, nil, false, merry.Errorf("unknown skip specified: '%s' is missing definition", t.Input())
	}

	// Flatten maps to list.
	apps = make([]*apiv1.AppPlan, 0, len(appsMap))
	for _, app := range appsMap {
		apps = append(apps, app)
	}

	return apps, skips, false, err
}

func filterDependencies(cfg *config.Project, state *statefile.StateData, targets, skips *util.TargetMatcher, skipAllApps bool) (deps []*apiv1.DependencyPlan) {
	if skipAllApps {
		deps = make([]*apiv1.DependencyPlan, 0, len(state.Dependencies))
		for _, dep := range state.Dependencies {
			deps = append(deps, &apiv1.DependencyPlan{
				State: dep,
			})
		}

		return deps
	}

	// In non target and non skip mode, use config apps and deps.
	if targets.IsEmpty() && skips.IsEmpty() {
		deps = make([]*apiv1.DependencyPlan, 0, len(cfg.Dependencies))
		for _, dep := range cfg.Dependencies {
			deps = append(deps, &apiv1.DependencyPlan{
				State: &apiv1.DependencyState{Dependency: dep.Proto()},
			})
		}

		return deps
	}

	dependenciesMap := make(map[string]*apiv1.DependencyPlan, len(state.Dependencies))

	// Always merge with dependencies from state unless there is no app needing them.
	needs := make(map[string]bool)

	for _, appState := range state.Apps {
		if appState.App == nil {
			continue
		}

		for n := range appState.App.Needs {
			needs[n] = true
		}
	}

	for key, dep := range state.Dependencies {
		if !needs[dep.Dependency.Name] {
			continue
		}

		dependenciesMap[key] = &apiv1.DependencyPlan{
			State: dep,
		}
	}

	for _, dep := range cfg.Dependencies {
		dependenciesMap[dep.ID()] = &apiv1.DependencyPlan{
			State: &apiv1.DependencyState{Dependency: dep.Proto()},
		}
	}

	// Flatten maps to list.
	deps = make([]*apiv1.DependencyPlan, 0, len(dependenciesMap))
	for _, dep := range dependenciesMap {
		deps = append(deps, dep)
	}

	return deps
}
