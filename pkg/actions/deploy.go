package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ansel1/merry/v2"
	dockerclient "github.com/docker/docker/client"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/outblocks/outblocks-cli/internal/statediff"
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

const (
	deployCommand = "deploy"
	stateLockWait = 30 * time.Second
)

type planParams struct {
	appPlans []*apiv1.AppPlan
	depPlans []*apiv1.DependencyPlan
	args     map[string]interface{}
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
	SkipDNS              bool
	SkipDiff             bool
	SkipApply            bool
}

func NewDeploy(log logger.Logger, cfg *config.Project, opts *DeployOptions) *Deploy {
	return &Deploy{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *Deploy) preChecks(state *types.StateData) error {
	return d.checkIfDNSAreUsed(state.Apps)
}

func (d *Deploy) stateLockRun(ctx context.Context) error {
	verify := d.opts.Verify
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   d.cfg.YAMLData(),
	}

	// Get state.
	state, stateRes, err := getState(ctx, d.cfg.State, d.opts.Lock, d.opts.LockWait, false, yamlContext)
	if err != nil {
		return err
	}

	if stateRes.StateCreated {
		d.log.Infof("New state created: '%s' for environment: %s\n", stateRes.StateName, d.cfg.State.Env())

		verify = true
	}

	// Build apps.
	if !d.opts.SkipBuild {
		err := d.buildApps(ctx, state.Apps)
		if err != nil {
			return err
		}
	}

	// Plan and apply.
	stateBefore := state.DeepCopy()

	err = d.preChecks(state)
	if err != nil {
		_ = releaseStateLock(d.cfg, stateRes.LockInfo)

		return err
	}

	empty, canceled, dur, _, err := d.planAndApply(ctx, verify, state, nil, false)

	// Proceed with saving.
	stateAfter := state

	stateDiff, diffErr := statediff.New(stateBefore, stateAfter)
	if diffErr != nil {
		return merry.Errorf("computing state diff failed: %w", err)
	}

	shouldSave := !canceled && (!empty || !stateDiff.IsEmpty()) && !d.opts.SkipDiff

	if shouldSave {
		d.log.Debugln("Saving state.")

		if saveErr := saveState(d.cfg, state); err == nil {
			err = saveErr
		}
	}

	if !canceled && !empty && err == nil && !d.opts.SkipApply {
		d.log.Printf("All changes applied in %s.\n", dur.Truncate(timeTruncate))
	}

	// Release lock if needed.
	if releaseErr := releaseStateLock(d.cfg, stateRes.LockInfo); err == nil {
		err = releaseErr
	}

	switch {
	case err != nil:
		return err
	case canceled:
		return nil
	}

	return d.showStateStatus(state.Apps, state.Dependencies, state.DNSRecords)
}

func (d *Deploy) allLockIDs(state *types.StateData) []string {
	lockIDsMap := make(map[string]struct{})

	for _, app := range d.cfg.Apps {
		lockIDsMap[app.ID()] = struct{}{}

		dnsPlugin := app.DNSPlugin()
		deployPlugin := app.DeployPlugin()

		if dnsPlugin != nil {
			lockIDsMap[dnsPlugin.ID()] = struct{}{}
		}

		if deployPlugin != nil {
			lockIDsMap[deployPlugin.ID()] = struct{}{}
		}
	}

	for key, app := range state.Apps {
		lockIDsMap[key] = struct{}{}

		if app == nil || app.App == nil {
			continue
		}

		if app.App.DnsPlugin != "" {
			lockIDsMap[plugins.ComputePluginID(app.App.DnsPlugin)] = struct{}{}
		}

		if app.App.DeployPlugin != "" {
			lockIDsMap[plugins.ComputePluginID(app.App.DeployPlugin)] = struct{}{}
		}
	}

	for _, dep := range d.cfg.Dependencies {
		lockIDsMap[dep.ID()] = struct{}{}

		deployPlugin := dep.DeployPlugin()

		if deployPlugin != nil {
			lockIDsMap[deployPlugin.ID()] = struct{}{}
		}
	}

	for key, dep := range state.Dependencies {
		lockIDsMap[key] = struct{}{}

		if dep == nil || dep.Dependency == nil {
			continue
		}

		if dep.Dependency.DeployPlugin != "" {
			lockIDsMap[plugins.ComputePluginID(dep.Dependency.DeployPlugin)] = struct{}{}
		}
	}

	locks := make([]string, 0, len(lockIDsMap))
	for key := range lockIDsMap {
		locks = append(locks, key)
	}

	return locks
}

func (d *Deploy) partialLockIDs(state *types.StateData) []string {
	if d.opts.SkipAllApps {
		return nil
	}

	targetAppIDsMap := util.StringArrayToSet(d.opts.TargetApps)
	skipAppIDsMap := util.StringArrayToSet(d.opts.SkipApps)
	lockIDsMap := make(map[string]struct{})

	for _, app := range d.cfg.Apps {
		lockIDsMap[app.ID()] = struct{}{}
	}

	for key := range state.Apps {
		lockIDsMap[key] = struct{}{}
	}

	// Filter targeted locking.
	lockIDsMapTemp := make(map[string]struct{})

	for key := range lockIDsMap {
		if len(targetAppIDsMap) > 0 && !targetAppIDsMap[key] {
			continue
		}

		if skipAppIDsMap[key] {
			continue
		}

		lockIDsMapTemp[key] = struct{}{}
	}

	lockIDsMap = lockIDsMapTemp

	locks := make([]string, 0, len(lockIDsMap))
	for key := range lockIDsMap {
		locks = append(locks, key)
	}

	return locks
}

func (d *Deploy) checkIfDNSAreUsed(stateApps map[string]*apiv1.AppState) error {
	var stateDomains []string

	for _, app := range stateApps {
		if app.App == nil {
			continue
		}

		u, _ := config.ParseAppURL(strings.ToLower(app.App.Url))
		if u != nil {
			stateDomains = append(stateDomains, u.Host)
		}
	}

	// Check state app domains as well.
	for _, dns := range d.cfg.DNS {
		if dns.IsUsed() {
			continue
		}

		for _, d := range stateDomains {
			if dns.Match(d) {
				dns.MarkAsUsed()
			}
		}
	}

	for i, dns := range d.cfg.DNS {
		if dns.IsUsed() {
			continue
		}

		n, err := yaml.PathString(fmt.Sprintf("$.dns[%d]", i))
		if err != nil {
			return err
		}

		file, err := parser.ParseBytes(d.cfg.YAMLData(), 0)
		if err != nil {
			return err
		}

		node, err := n.FilterFile(file)
		if err != nil {
			return err
		}

		d.log.Warnf("One or more project DNS configurations are unused!\nfile: %s, line: %d\n", d.cfg.YAMLPath(), node.GetToken().Position.Line)

		return nil
	}

	return nil
}

func (d *Deploy) multilockPlanAndApply(ctx context.Context, state *types.StateData, partialLock, verify bool, yamlContext *client.YAMLContext) (stateDiff *statediff.Diff, shouldSave bool, acquiredLocks map[string]string, dur time.Duration, err error) {
	var (
		locks           []string
		missingLocks    []string
		stateBefore     *types.StateData
		empty, canceled bool
	)

	// Acquire necessary acquiredLocks.
	if partialLock {
		locks = d.partialLockIDs(state)
	} else {
		locks = d.allLockIDs(state)
	}

	first := true
	statePlugin := d.cfg.State.Plugin()

	for {
		acquiredLocks, err = statePlugin.Client().AcquireLocks(ctx, d.cfg.State.Other, locks, d.opts.LockWait, yamlContext)
		if err != nil {
			return nil, false, nil, dur, err
		}

		d.log.Debugf("Acquired locks: %s\n", acquiredLocks)

		// Build apps.
		if first && !d.opts.SkipBuild {
			err = d.buildApps(ctx, state.Apps)
			if err != nil {
				_ = releaseLocks(d.cfg, acquiredLocks)
				return nil, false, nil, dur, err
			}

			first = false
		}

		// Plan and apply.
		stateBefore = state.DeepCopy()

		err = d.preChecks(state)
		if err != nil {
			_ = releaseLocks(d.cfg, acquiredLocks)
			return nil, false, nil, dur, err
		}

		empty, canceled, dur, missingLocks, err = d.planAndApply(ctx, verify, state, acquiredLocks, true)
		if err != nil {
			_ = releaseLocks(d.cfg, acquiredLocks)
			acquiredLocks = nil

			// Proceed to save partial work if needed.
			break
		}

		if len(missingLocks) == 0 {
			break
		}

		locks = append(locks, missingLocks...)

		err = releaseLocks(d.cfg, acquiredLocks)
		if err != nil {
			return nil, false, nil, dur, err
		}

		state, _, err = getState(ctx, d.cfg.State, false, d.opts.LockWait, false, yamlContext)
		if err != nil {
			return nil, false, nil, dur, err
		}
	}

	stateAfter := state

	stateDiff, diffErr := statediff.New(stateBefore, stateAfter)
	if diffErr != nil {
		return nil, false, nil, dur, merry.Errorf("computing state diff failed: %w", err)
	}

	shouldSave = !canceled && (!empty || !stateDiff.IsEmpty()) && !d.opts.SkipDiff

	if !stateDiff.IsEmpty() {
		d.log.Debugf("State Diff (to apply: %t)\n%s\n", shouldSave, stateDiff)
	}

	return stateDiff, shouldSave, acquiredLocks, dur, err
}

func (d *Deploy) multilockRun(ctx context.Context) error {
	verify := d.opts.Verify
	partialLock := len(d.opts.TargetApps) > 0 || len(d.opts.SkipApps) > 0
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   d.cfg.YAMLData(),
	}

	// Get state.
	state, stateRes, err := getState(ctx, d.cfg.State, false, d.opts.LockWait, false, yamlContext)
	if err != nil {
		return err
	}

	if stateRes.StateCreated {
		d.log.Infof("New state created: '%s' for environment: %s\n", stateRes.StateName, d.cfg.State.Env())

		verify = true
		partialLock = false
	}

	// Acquire locks and plan+apply.
	stateDiff, shouldSave, acquiredLocks, dur, err := d.multilockPlanAndApply(ctx, state, partialLock, verify, yamlContext)
	if err != nil {
		return err
	}

	if shouldSave {
		d.log.Debugln("Saving state.")

		var stateErr error

		state, stateRes, stateErr = getState(context.Background(), d.cfg.State, true, stateLockWait, false, yamlContext)
		if stateErr != nil {
			_ = releaseLocks(d.cfg, acquiredLocks)
			return stateErr
		}

		// These are "less important" errors.
		if e := stateDiff.Apply(state); err == nil {
			err = e
		}

		if e := saveState(d.cfg, state); err == nil {
			err = e
		}

		if e := releaseStateLock(d.cfg, stateRes.LockInfo); err == nil {
			err = e
		}
	}

	if shouldSave && err == nil {
		d.log.Printf("All changes applied in %s.\n", dur.Truncate(timeTruncate))
	}

	// Release locks.
	if releaseErr := releaseLocks(d.cfg, acquiredLocks); err == nil {
		err = releaseErr
	}

	switch {
	case err != nil:
		return err
	case !shouldSave:
		return nil
	}

	return d.showStateStatus(state.Apps, state.Dependencies, state.DNSRecords)
}

func (d *Deploy) Run(ctx context.Context) error {
	if d.opts.Lock && d.cfg.State.Plugin().HasAction(plugins.ActionLock) {
		return d.multilockRun(ctx)
	}

	return d.stateLockRun(ctx)
}

func (d *Deploy) promptDiff(deployChanges []*change, acquiredLocks map[string]string, checkLocks bool) (empty, canceled bool, missingLocks []string) {
	missingLocksMap := make(map[string]struct{})

	if checkLocks {
		for _, chg := range deployChanges {
			var lockID string

			switch {
			case chg.app != nil:
				lockID = chg.app.Id
			case chg.dep != nil:
				lockID = chg.dep.Id
			case chg.plugin != nil:
				lockID = chg.plugin.ID()
			}

			if _, ok := acquiredLocks[lockID]; !ok {
				missingLocksMap[lockID] = struct{}{}
			}
		}

		if len(missingLocksMap) > 0 {
			for k := range missingLocksMap {
				missingLocks = append(missingLocks, k)
			}

			return false, false, missingLocks
		}
	}

	empty, canceled = planPrompt(d.log, d.cfg.Env(), deployChanges, nil, d.opts.AutoApprove, d.opts.ForceApprove)

	return empty, canceled, missingLocks
}

func (d *Deploy) planAndApply(ctx context.Context, verify bool, state *types.StateData, acquiredLocks map[string]string, checkLocks bool) (empty, canceled bool, dur time.Duration, missingLocks []string, err error) {
	var domains []*apiv1.DomainInfo

	if d.opts.SkipDNS {
		domains = state.DomainsInfo
	} else {
		domains = d.cfg.DomainInfoProto()
	}

	// Plan and apply.
	apps, skipAppIDs, destroy, err := filterApps(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps, d.opts.Destroy)
	if err != nil {
		return false, false, dur, nil, err
	}

	deps := filterDependencies(d.cfg, state, d.opts.TargetApps, d.opts.SkipApps, d.opts.SkipAllApps)

	planMap, err := calculatePlanMap(d.cfg, apps, deps, d.opts.TargetApps, skipAppIDs)
	if err != nil {
		return false, false, dur, nil, err
	}

	// Start plugins.
	for plug := range planMap {
		err = plug.Client().Start(ctx)
		if err != nil {
			return false, false, dur, nil, err
		}
	}

	msg := "Planning..."
	if d.opts.SkipApply {
		msg = "Checking..."
	}

	spinner, _ := d.log.Spinner().Start(msg)

	// Proceed with plan - reset state apps and deps.
	oldState := *state
	state.Reset()

	planRetMap, err := plan(ctx, state, planMap, domains, verify, destroy)
	if err != nil {
		spinner.Stop()

		return false, true, dur, nil, err
	}

	spinner.Stop()

	if !d.opts.SkipDiff {
		deployChanges := computeChange(d.cfg, &oldState, state, planRetMap)

		empty, canceled, missingLocks = d.promptDiff(deployChanges, acquiredLocks, checkLocks)
		if len(missingLocks) != 0 {
			return empty, canceled, dur, missingLocks, nil
		}

		// Apply if needed.
		if !canceled && !empty && !d.opts.SkipApply {
			callback := applyProgress(d.log, deployChanges, nil)
			err = apply(context.Background(), state, planMap, domains, destroy, callback)
		}
	}

	start := time.Now()

	// Merge state with current apps/deps if needed (they might not have a state defined).
	for _, app := range apps {
		if _, ok := state.Apps[app.State.App.Id]; ok {
			continue
		}

		state.Apps[app.State.App.Id] = &apiv1.AppState{App: app.State.App}
	}

	for _, dep := range deps {
		if _, ok := state.Dependencies[dep.State.Dependency.Id]; ok {
			continue
		}

		state.Dependencies[dep.State.Dependency.Id] = &apiv1.DependencyState{Dependency: dep.State.Dependency}
	}

	state.DomainsInfo = domains

	return empty, canceled, time.Since(start), nil, err
}

func (d *Deploy) prepareAppSSLMap(appStates map[string]*apiv1.AppState) (map[string]*apiv1.DNSState, error) {
	sslMap := make(map[string]*apiv1.DNSState)

	for _, appState := range appStates {
		if appState.Dns == nil || appState.Dns.SslStatus == apiv1.DNSState_SSL_STATUS_UNSPECIFIED || (appState.Dns.Cname == "" && appState.Dns.Ip == "") {
			continue
		}

		host, err := urlutil.ExtractHostname(appState.Dns.Url)
		if err != nil {
			return nil, err
		}

		sslMap[host] = appState.Dns
	}

	return sslMap, nil
}

func (d *Deploy) showAppStates(appStates map[string]*apiv1.AppState, appNameStyle *pterm.Style) (allReady bool) {
	appURLStyle := pterm.NewStyle(pterm.FgGreen, pterm.Underscore)
	appFailingStyle := pterm.NewStyle(pterm.FgRed, pterm.Bold)

	var (
		readyApps   []*apiv1.AppState
		unreadyApps []*apiv1.AppState
	)

	for _, appState := range appStates {
		if appState.Deployment == nil {
			continue
		}

		if appState.Deployment.Ready {
			readyApps = append(readyApps, appState)
		} else {
			unreadyApps = append(unreadyApps, appState)
		}
	}

	sort.Slice(readyApps, func(i int, j int) bool {
		return readyApps[i].App.Name < readyApps[j].App.Name
	})

	sort.Slice(unreadyApps, func(i int, j int) bool {
		return unreadyApps[i].App.Name < unreadyApps[j].App.Name
	})

	if len(readyApps) > 0 || len(unreadyApps) > 0 {
		d.log.Section().Println("App Status")

		for _, appState := range readyApps {
			app := appState.App

			var appInfo []string

			if app.Url != "" {
				appInfo = append(appInfo, pterm.Gray("URL"), appURLStyle.Sprint(app.Url))
			} else if appState.Dns != nil {
				var privateURL string

				switch {
				case appState.Dns.InternalUrl != "":
					privateURL = appState.Dns.InternalUrl
				case appState.Dns.InternalIp != "":
					privateURL = appState.Dns.InternalIp
				default:
					continue
				}

				appInfo = append(appInfo, pterm.Gray("private"), appURLStyle.Sprint(privateURL))
			}

			d.log.Printf("%s (%s)\n  %s\n", appNameStyle.Sprint(app.Name), app.Type, strings.Join(appInfo, " "))

			if appState.Dns.CloudUrl != "" {
				fmt.Printf("  %s %s\n", pterm.Gray("cloud URL"), appURLStyle.Sprint(appState.Dns.CloudUrl))
			}
		}

		for _, appState := range unreadyApps {
			app := appState.App

			d.log.Printf("%s (%s) %s %s\n", appNameStyle.Sprint(app.Name), app.Type, pterm.Gray("==>"), appFailingStyle.Sprint("FAILING"))
			d.log.Errorln(appState.Deployment.Message)
		}
	}

	return len(unreadyApps) == 0
}

func (d *Deploy) showStateStatus(appStates map[string]*apiv1.AppState, dependencyStates map[string]*apiv1.DependencyState, dnsRecords types.DNSRecordMap) error {
	var dns []*apiv1.DNSRecord

	for k, v := range dnsRecords {
		if !v.Created {
			dns = append(dns, &apiv1.DNSRecord{
				Record: k.Record,
				Type:   k.Type,
				Value:  v.Value,
			})
		}
	}

	sort.Slice(dns, func(i, j int) bool {
		return dns[i].Record < dns[j].Record
	})

	// Show info about DNS records still requiring setup.
	data := [][]string{
		{"Record", "Type", "Value"},
	}

	for _, v := range dns {
		typ := v.Type.String()[len("TYPE_"):]

		data = append(data, []string{pterm.Green(v.Record), pterm.Yellow(typ), v.Value})
	}

	if len(dns) > 0 {
		d.log.Section().Println("DNS Setup (manual)")
		_ = d.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
	}

	// App Status.
	appNameStyle := pterm.NewStyle(pterm.Reset, pterm.Bold)
	allReady := d.showAppStates(appStates, appNameStyle)

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

			d.log.Printf("%s (%s) %s %s\n", appNameStyle.Sprint(dep.Name), dep.Type, pterm.Gray("==>"), pterm.Green(depState.Dns.ConnectionInfo))
		}
	}

	// Show info about SSL status.
	sslMap, err := d.prepareAppSSLMap(appStates)
	if err != nil {
		return err
	}

	if len(sslMap) > 0 {
		data := make([][]string, 0, len(sslMap))

		for host, v := range sslMap {
			data = append(data, []string{pterm.Green(host), pterm.Yellow(v.SslStatus.String()[len("SSL_STATUS_"):]), v.SslStatusInfo})
		}

		sort.Slice(data, func(i, j int) bool {
			return data[i][0] < data[j][0]
		})

		data = append([][]string{{"Domain", "Status", "Info"}}, data...)

		d.log.Section().Println("SSL Certificates")
		_ = d.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
	}

	if !allReady {
		return merry.Errorf("not all apps are ready")
	}

	return nil
}

func saveState(cfg *config.Project, data *types.StateData) error {
	state := cfg.State
	plug := state.Plugin()

	if state.IsLocal() {
		return state.SaveLocal(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	_, err := plug.Client().SaveState(ctx, data, state.Type, state.Other)

	return err
}

func getState(ctx context.Context, state *config.State, lock bool, lockWait time.Duration, skipCreate bool, yamlContext *client.YAMLContext) (stateData *types.StateData, stateRes *apiv1.GetStateResponse_State, err error) {
	plug := state.Plugin()

	if state.IsLocal() {
		stateData, err = state.LoadLocal()
		if err != nil {
			return nil, nil, err
		}

		return stateData, &apiv1.GetStateResponse_State{}, nil
	}

	ret, err := plug.Client().GetState(ctx, state.Type, state.Other, lock, lockWait, skipCreate, yamlContext)
	if err != nil {
		return nil, nil, err
	}

	stateData = types.NewStateData()

	if len(ret.State) != 0 {
		err = json.Unmarshal(ret.State, &stateData)
	}

	return stateData, ret, err
}

func calculatePlanMap(cfg *config.Project, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, targetAppIDs, skipAppIDs []string) (map[*plugins.Plugin]*planParams, error) {
	planMap := make(map[*plugins.Plugin]*planParams)

	for _, app := range apps {
		if app.State.App == nil {
			continue
		}

		deployPlugin := cfg.FindLoadedPlugin(app.State.App.DeployPlugin)
		if deployPlugin == nil {
			return nil, merry.Errorf("missing deploy plugin: %s used for app: %s", app.State.App.DeployPlugin, app.State.App.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		planMap[deployPlugin].appPlans = append(planMap[deployPlugin].appPlans, app)
	}

	// Process dependencies.
	for _, dep := range deps {
		deployPlugin := cfg.FindLoadedPlugin(dep.State.Dependency.DeployPlugin)
		if deployPlugin == nil {
			return nil, merry.Errorf("missing deploy plugin: %s used for dependency: %s", dep.State.Dependency.DeployPlugin, dep.State.Dependency.Name)
		}

		if _, ok := planMap[deployPlugin]; !ok {
			planMap[deployPlugin] = &planParams{
				args: deployPlugin.CommandArgs(deployCommand),
			}
		}

		planMap[deployPlugin].depPlans = append(planMap[deployPlugin].depPlans, &apiv1.DependencyPlan{
			State: dep.State,
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

func mergeState(state *types.StateData, pluginName string, pluginState *apiv1.PluginState, appStates map[string]*apiv1.AppState, depStates map[string]*apiv1.DependencyState, dnsRecords []*apiv1.DNSRecord) {
	state.Plugins[pluginName] = types.PluginStateFromProto(pluginState)

	// Merge state with new changes.
	for k, v := range appStates {
		state.Apps[k] = v
	}

	for k, v := range depStates {
		state.Dependencies[k] = v
	}

	for _, v := range dnsRecords {
		state.AddDNSRecord(v)
	}
}

func plan(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, domains []*apiv1.DomainInfo, verify, destroy bool) (retMap map[*plugins.Plugin]*apiv1.PlanResponse, err error) {
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
		mergeState(state, plug.Name, ret.State, ret.AppStates, ret.DependencyStates, ret.DnsRecords)

		mu.Unlock()
	}

	// Plan all plugins concurrently.
	for plug, params := range planMap {
		plug := plug
		params := params

		g.Go(func() error {
			ret, err := plug.Client().Plan(ctx, state, params.appPlans, params.depPlans, domains, params.args, verify, destroy)
			if err != nil {
				return err
			}

			processResponse(plug, ret)

			return nil
		})
	}

	err = g.Wait()

	return retMap, err
}

func apply(ctx context.Context, state *types.StateData, planMap map[*plugins.Plugin]*planParams, domains []*apiv1.DomainInfo, destroy bool, callback func(*apiv1.ApplyAction)) error {
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
		mergeState(state, plug.Name, ret.State, ret.AppStates, ret.DependencyStates, ret.DnsRecords)

		mu.Unlock()
	}

	// Apply second pass plan (DNS and deployments with DNS).
	for plug, params := range planMap {
		g.Go(func() error {
			ret, err := plug.Client().Apply(ctx, state, params.appPlans, params.depPlans, domains, params.args, destroy, callback)
			processResponse(plug, ret)

			return err
		})
	}

	return g.Wait()
}
