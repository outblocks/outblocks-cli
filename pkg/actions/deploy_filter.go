package actions

import (
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func filterAppsNormal(cfg *config.Project, state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) map[string]*apiv1.AppPlan {
	appsMap := make(map[string]*apiv1.AppPlan, len(state.Apps))

	// Use state apps as base unless skip mode is enabled and they are NOT to be skipped.
	for key, app := range state.Apps {
		if len(skipAppIDsMap) > 0 && !skipAppIDsMap[key] {
			continue
		}

		appsMap[key] = &apiv1.AppPlan{
			State: app,
		}
	}

	// Overwrite definition of non-skipped or target apps from project definition.
	for _, app := range cfg.Apps {
		if len(targetAppIDsMap) > 0 && !targetAppIDsMap[app.ID()] {
			continue
		}

		if skipAppIDsMap[app.ID()] {
			continue
		}

		appType := app.Proto()
		mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

		appType.Properties = plugin_util.MustNewStruct(mergedProps)
		appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

		appsMap[app.ID()] = &apiv1.AppPlan{
			State: &apiv1.AppState{
				App: appType,
			},
			Build: app.BuildProto(),
		}
	}

	return appsMap
}

func filterAppsDestroy(state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) (appsMap map[string]*apiv1.AppPlan, skipAppIDs []string) {
	skipAppIDs = make([]string, 0, len(state.Apps))
	appsMap = make(map[string]*apiv1.AppPlan, len(state.Apps))

	// Use state apps as base unless skip mode is enabled and they are NOT to be skipped or they are targeted.
	for key, app := range state.Apps {
		if len(skipAppIDsMap) > 0 && !skipAppIDsMap[key] {
			continue
		}

		// Add app to skip IDs as we don't want to deal with it's changes.
		skipAppIDs = append(skipAppIDs, key)

		if len(targetAppIDsMap) > 0 && targetAppIDsMap[key] {
			delete(targetAppIDsMap, key)

			continue
		}

		appsMap[key] = &apiv1.AppPlan{
			State: app,
		}
	}

	return appsMap, skipAppIDs
}

func filterApps(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps, destroy bool) (apps []*apiv1.AppPlan, retSkipAppIDs []string, retDestroy bool, err error) {
	if skipAllApps {
		retSkipAppIDs := make([]string, 0, len(state.Apps))
		apps = make([]*apiv1.AppPlan, 0, len(state.Apps))

		for _, appState := range state.Apps {
			apps = append(apps, &apiv1.AppPlan{
				State: appState,
			})

			retSkipAppIDs = append(retSkipAppIDs, appState.App.Id)
		}

		return apps, retSkipAppIDs, destroy, nil
	}

	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		apps = make([]*apiv1.AppPlan, 0, len(cfg.Apps))

		for _, app := range cfg.Apps {
			appType := app.Proto()
			mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

			appType.Properties = plugin_util.MustNewStruct(mergedProps)
			appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

			apps = append(apps, &apiv1.AppPlan{
				State: &apiv1.AppState{
					App: appType,
				},
				Build: app.BuildProto(),
			})
		}

		return apps, nil, destroy, nil
	}

	var appsMap map[string]*apiv1.AppPlan

	targetAppIDsMap := util.StringArrayToSet(targetAppIDs)
	skipAppIDsMap := util.StringArrayToSet(skipAppIDs)

	if destroy {
		// Destroy mode.
		appsMap, skipAppIDs = filterAppsDestroy(state, targetAppIDsMap, skipAppIDsMap)
	} else {
		// Normal mode.
		appsMap = filterAppsNormal(cfg, state, targetAppIDsMap, skipAppIDsMap)
	}

	// If there are any left target/skip apps without definition, that's an error.
	for app := range targetAppIDsMap {
		if appsMap[app] == nil {
			return nil, nil, false, merry.Errorf("unknown target app specified: app with ID '%s' is missing definition", app)
		}
	}

	for app := range skipAppIDsMap {
		if appsMap[app] == nil {
			return nil, nil, false, merry.Errorf("unknown skip app specified: app with ID '%s' is missing definition", app)
		}
	}

	// Flatten maps to list.
	apps = make([]*apiv1.AppPlan, 0, len(appsMap))
	for _, app := range appsMap {
		apps = append(apps, app)
	}

	return apps, skipAppIDs, false, err
}

func filterDependencies(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps bool) (deps []*apiv1.DependencyPlan) {
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
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
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
