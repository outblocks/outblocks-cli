package actions

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func filterAppsNormal(cfg *config.Project, state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) map[string]*types.AppState {
	appsMap := make(map[string]*types.AppState, len(state.Apps))

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

		appType := app.PluginType()
		appType.Properties = plugin_util.MergeMaps(cfg.Defaults.Deploy.Other, appType.Properties, app.DeployInfo().Other)
		appType.Env = plugin_util.MergeStringMaps(cfg.Defaults.Run.Env, appType.Env, app.DeployInfo().Env)

		appsMap[app.ID()] = types.NewAppState(appType)
	}

	return appsMap
}

func filterAppsDestroy(state *types.StateData, targetAppIDsMap, skipAppIDsMap map[string]bool) (appsMap map[string]*types.AppState, skipAppIDs []string) {
	skipAppIDs = make([]string, 0, len(state.Apps))
	appsMap = make(map[string]*types.AppState, len(state.Apps))

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

func filterApps(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps, destroy bool) (apps []*types.AppState, retSkipAppIDs []string, retDestroy bool, err error) {
	if skipAllApps {
		retSkipAppIDs := make([]string, 0, len(state.Apps))
		apps = make([]*types.AppState, 0, len(state.Apps))

		for _, app := range state.Apps {
			apps = append(apps, app)
			retSkipAppIDs = append(retSkipAppIDs, app.ID)
		}

		return apps, retSkipAppIDs, destroy, nil
	}

	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		apps = make([]*types.AppState, 0, len(cfg.Apps))
		for _, app := range cfg.Apps {
			apps = append(apps, types.NewAppState(app.PluginType()))
		}

		return apps, nil, destroy, nil
	}

	var appsMap map[string]*types.AppState

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
			return nil, nil, false, fmt.Errorf("unknown target app specified: app with ID '%s' is missing definition", app)
		}
	}

	for app := range skipAppIDsMap {
		if appsMap[app] == nil {
			return nil, nil, false, fmt.Errorf("unknown skip app specified: app with ID '%s' is missing definition", app)
		}
	}

	// Flatten maps to list.
	apps = make([]*types.AppState, 0, len(appsMap))
	for _, app := range appsMap {
		apps = append(apps, app)
	}

	return apps, skipAppIDs, false, err
}

func filterDependencies(cfg *config.Project, state *types.StateData, targetAppIDs, skipAppIDs []string, skipAllApps bool) (deps []*types.DependencyState) {
	if skipAllApps {
		deps = make([]*types.DependencyState, 0, len(state.Dependencies))
		for _, dep := range state.Dependencies {
			deps = append(deps, dep)
		}

		return deps
	}

	// In non target and non skip mode, use config apps and deps.
	if len(skipAppIDs) == 0 && len(targetAppIDs) == 0 {
		deps = make([]*types.DependencyState, 0, len(cfg.Dependencies))
		for _, dep := range cfg.Dependencies {
			deps = append(deps, types.NewDependencyState(dep.PluginType()))
		}

		return deps
	}

	dependenciesMap := make(map[string]*types.DependencyState, len(state.Dependencies))

	// Always merge with dependencies from state unless there is no app needing them.
	needs := make(map[string]bool)

	for _, app := range state.Apps {
		for n := range app.Needs {
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
		dependenciesMap[dep.ID()] = types.NewDependencyState(dep.PluginType())
	}

	// Flatten maps to list.
	deps = make([]*types.DependencyState, 0, len(dependenciesMap))
	for _, dep := range dependenciesMap {
		deps = append(deps, dep)
	}

	return deps
}
