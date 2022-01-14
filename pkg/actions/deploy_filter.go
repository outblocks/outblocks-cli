package actions

import (
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func filterAppsNormal(cfg *config.Project, state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) map[string]*apiv1.AppState {
	appsMap := make(map[string]*apiv1.AppState, len(state.Apps))

	// Use state apps as base unless skip mode is enabled and they are NOT to be skipped.
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

		appType := app.Proto()
		mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

		appType.Properties = plugin_util.MustNewStruct(mergedProps)
		appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

		appsMap[app.ID()] = &apiv1.AppState{
			App: appType,
		}
	}

	return appsMap
}

func filterAppsDestroy(state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) (appsMap map[string]*apiv1.AppState, skipAppIDs []string) {
	skipAppIDs = make([]string, 0, len(state.Apps))
	appsMap = make(map[string]*apiv1.AppState, len(state.Apps))

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

		appsMap[key] = app
	}

	return appsMap, skipAppIDs
}

func filterApps(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, destroy bool) (apps []*apiv1.AppState, retSkipAppIDs []string, retDestroy bool, err error) {
	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		apps = make([]*apiv1.AppState, 0, len(cfg.Apps))

		for _, app := range cfg.Apps {
			appType := app.Proto()
			mergedProps := plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties.AsMap(), app.DeployInfo().Other)

			appType.Properties = plugin_util.MustNewStruct(mergedProps)
			appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

			apps = append(apps, &apiv1.AppState{
				App: appType,
			})
		}

		return apps, nil, destroy, nil
	}

	var appsMap map[string]*apiv1.AppState

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
	apps = make([]*apiv1.AppState, 0, len(appsMap))
	for _, app := range appsMap {
		apps = append(apps, app)
	}

	return apps, skipAppIDs, false, err
}

func filterDependencies(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string) (deps []*apiv1.DependencyState) {
	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		deps = make([]*apiv1.DependencyState, 0, len(cfg.Dependencies))
		for _, dep := range cfg.Dependencies {
			deps = append(deps, &apiv1.DependencyState{Dependency: dep.Proto()})
		}

		return deps
	}

	dependenciesMap := make(map[string]*apiv1.DependencyState, len(state.Dependencies))

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

		dependenciesMap[key] = dep
	}

	for _, dep := range cfg.Dependencies {
		dependenciesMap[dep.ID()] = &apiv1.DependencyState{Dependency: dep.Proto()}
	}

	// Flatten maps to list.
	deps = make([]*apiv1.DependencyState, 0, len(dependenciesMap))
	for _, dep := range dependenciesMap {
		deps = append(deps, dep)
	}

	return deps
}
