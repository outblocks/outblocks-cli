package actions

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/pterm/pterm"
)

type changeID struct {
	planType   apiv1.PlanType
	objectType string
}

type change struct {
	app         *apiv1.App
	dep         *apiv1.Dependency
	plugin      *plugins.Plugin
	obj         string
	criticalMap map[changeID]bool
	infoMap     map[changeID][]string
}

func (i *change) Name() string {
	if i.app != nil {
		return fmt.Sprintf("%s App '%s'", strings.Title(i.app.Type), i.app.Name)
	}

	if i.dep != nil {
		return fmt.Sprintf("Dependency '%s'", i.dep.Name)
	}

	return fmt.Sprintf("Plugin '%s' %s", i.plugin.Name, i.obj)
}

func (i *changeID) Type() string {
	switch i.planType {
	case apiv1.PlanType_PLAN_TYPE_CREATE:
		return pterm.Green("+ add")
	case apiv1.PlanType_PLAN_TYPE_RECREATE:
		return pterm.Red("~ recreate")
	case apiv1.PlanType_PLAN_TYPE_UPDATE:
		return pterm.Yellow("~ update")
	case apiv1.PlanType_PLAN_TYPE_PROCESS:
		return pterm.Blue("~ process")
	case apiv1.PlanType_PLAN_TYPE_DELETE:
		return pterm.Red("- delete")
	case apiv1.PlanType_PLAN_TYPE_UNSPECIFIED:
	}

	panic("unknown type")
}

func newChangeFromPlanAction(cfg *config.Project, act *apiv1.PlanAction, state *types.StateData, plugin *plugins.Plugin) *change {
	switch act.Source {
	case types.SourceApp:
		var app *apiv1.App

		if a := cfg.AppByID(act.Namespace); a != nil {
			app = a.Proto()
		}

		if app == nil && state.Apps[act.Namespace] != nil {
			app = state.Apps[act.Namespace].App
		}

		if app != nil {
			return &change{
				app: app,
			}
		}
	case types.SourceDependency:
		var dep *apiv1.Dependency

		if d := cfg.DependencyByID(act.Namespace); d != nil {
			dep = d.Proto()
		}

		if dep == nil && state.Dependencies[act.Namespace] != nil {
			dep = state.Dependencies[act.Namespace].Dependency
		}

		if dep != nil {
			return &change{
				dep: dep,
			}
		}
	}

	return &change{
		plugin: plugin,
		obj:    act.Namespace,
	}
}

func computeChangeInfo(cfg *config.Project, state *types.StateData, plugin *plugins.Plugin, actions []*apiv1.PlanAction) (changes []*change) {
	changesMap := make(map[string]*change)

	for _, act := range actions {
		chg := changesMap[act.Namespace]

		if chg == nil {
			chg = newChangeFromPlanAction(cfg, act, state, plugin)
			chg.infoMap = make(map[changeID][]string)
			chg.criticalMap = make(map[changeID]bool)
			changesMap[act.Namespace] = chg
			changes = append(changes, chg)
		}

		key := changeID{
			planType:   act.Type,
			objectType: act.ObjectType,
		}

		chg.criticalMap[key] = chg.criticalMap[key] || act.Critical
		chg.infoMap[key] = append(chg.infoMap[key], act.ObjectName)
	}

	return changes
}

func computeChange(cfg *config.Project, oldState, state *types.StateData, planMap map[*plugins.Plugin]*apiv1.PlanResponse) []*change { // nolint:unparam
	var changes []*change

	for plugin, p := range planMap {
		chg := computeChangeInfo(cfg, state, plugin, p.Deploy.Actions)
		changes = append(changes, chg...)
	}

	return changes
}

func calculateTotal(chg []*change) (add, change, process, destroy int) {
	for _, c := range chg {
		for chID, objs := range c.infoMap {
			switch chID.planType {
			case apiv1.PlanType_PLAN_TYPE_CREATE:
				add += len(objs)
			case apiv1.PlanType_PLAN_TYPE_RECREATE, apiv1.PlanType_PLAN_TYPE_UPDATE:
				change += len(objs)
			case apiv1.PlanType_PLAN_TYPE_DELETE:
				destroy += len(objs)
			case apiv1.PlanType_PLAN_TYPE_PROCESS:
				process += len(objs)
			case apiv1.PlanType_PLAN_TYPE_UNSPECIFIED:
				panic("unknown plan type")
			}
		}
	}

	return add, change, process, destroy
}

func calculateTotalSteps(chg []*change) int {
	steps := 0

	for _, c := range chg {
		for changeID, v := range c.infoMap {
			if changeID.planType == apiv1.PlanType_PLAN_TYPE_RECREATE {
				// Recreate steps are doubled.
				steps += 2 * len(v)
			} else {
				steps += len(v)
			}
		}
	}

	return steps
}

func formatChangeInfo(chID changeID, objs []string, critical bool) string {
	var suffix string

	if critical {
		suffix = " " + pterm.Red("!!!")
	}

	if len(objs) == 1 {
		return fmt.Sprintf("    %s %s '%s'%s\n", chID.Type(), chID.objectType, pterm.Normal(objs[0]), suffix)
	}

	if len(objs) <= 5 {
		return fmt.Sprintf("    %s %d of %s ['%s']%s\n", chID.Type(), len(objs), chID.objectType, pterm.Normal(strings.Join(objs, "', '")), suffix)
	}

	return fmt.Sprintf("    %s %d of %s%s\n", chID.Type(), len(objs), chID.objectType, suffix)
}

func planChangeInfo(header string, changes []*change) (info string, anyCritical bool) {
	if len(changes) == 0 {
		return "", false
	}

	headerStyle := pterm.NewStyle(pterm.FgWhite, pterm.Bold)
	headerInfoStyle := pterm.NewStyle(pterm.FgWhite, pterm.Reset)

	add, change, process, destroy := calculateTotal(changes)
	info += fmt.Sprintf("%s %s\n", headerStyle.Sprintf(header), headerInfoStyle.Sprintf("(%d to add, %d to change, %d to destroy, %d to process)", add, change, destroy, process))

	for _, chg := range changes {
		info += fmt.Sprintf("  %s\n", pterm.Bold.Sprintf("\n  %s changes:", chg.Name()))

		for k, i := range chg.infoMap {
			critical := chg.criticalMap[k]
			anyCritical = anyCritical || critical
			info += formatChangeInfo(k, i, critical)
		}
	}

	return info, anyCritical
}

func planPrompt(log logger.Logger, env string, deploy, dns []*change, approve, force bool) (empty, canceled bool) {
	sort.Slice(deploy, func(i, j int) bool {
		if deploy[i].app == nil && deploy[j].app != nil {
			return false
		}

		if deploy[i].app != nil && deploy[j].app == nil {
			return true
		}

		return deploy[i].Name() < deploy[j].Name()
	})

	info := []string{fmt.Sprintf("Outblocks will perform the following changes to your '%s' environment:", pterm.Bold.Sprint(env))}
	empty = true
	critical := false

	// Deployment
	deployInfo, deployCritical := planChangeInfo("Deployment:", deploy)
	if deployInfo != "" {
		empty = false

		info = append(info, deployInfo)
	}

	// DNS
	// TODO: handle dns as a diff of records
	dnsInfo, dnsCritical := planChangeInfo("DNS:", dns)
	if dnsInfo != "" {
		empty = false

		info = append(info, dnsInfo)
	}

	if empty {
		log.Println("No changes detected.")

		return true, false
	}

	critical = deployCritical || dnsCritical

	log.Println(strings.Join(info, "\n\n"))

	if (!critical && approve) || force {
		return false, false
	}

	proceed := false
	prompt := &survey.Confirm{
		Message: "Do you want to perform these actions?",
	}

	if critical {
		prompt.Message = fmt.Sprintf("%s Some changes are potentially destructive! Are you really sure you want to perform these actions?", pterm.Red("Warning!"))
	}

	_ = survey.AskOne(prompt, &proceed)

	if !proceed {
		log.Println("Apply canceled.")
		return false, true
	}

	return false, false
}

type applyTargetKey struct {
	ns, obj, typ string
}

type applyTarget struct {
	act           *apiv1.ApplyAction
	start, notify time.Time
}

func newApplyTarget(act *apiv1.ApplyAction) *applyTarget {
	t := time.Now()

	return &applyTarget{
		act:    act,
		start:  t,
		notify: t,
	}
}

func applyActionType(act *apiv1.ApplyAction) string {
	switch act.Type {
	case apiv1.PlanType_PLAN_TYPE_CREATE:
		return "creating"
	case apiv1.PlanType_PLAN_TYPE_DELETE:
		return "deleting"
	case apiv1.PlanType_PLAN_TYPE_UPDATE:
		return "updating"
	case apiv1.PlanType_PLAN_TYPE_RECREATE:
		return "recreating"
	case apiv1.PlanType_PLAN_TYPE_PROCESS:
		return "processing"
	case apiv1.PlanType_PLAN_TYPE_UNSPECIFIED:
	}

	return "unknown"
}

func applyProgress(log logger.Logger, deployChanges, dnsChanges []*change) func(*apiv1.ApplyAction) {
	changes := append(deployChanges, dnsChanges...) // nolint: gocritic
	total := calculateTotalSteps(changes)

	// Create progressbar as fork from the default progressbar.
	p, _ := log.ProgressBar().WithTotal(total).WithTitle("Applying...").Start()

	startMap := make(map[applyTargetKey]*applyTarget)

	var m sync.Mutex

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for {
			<-t.C
			m.Lock()

			now := time.Now()

			for _, v := range startMap {
				if time.Since(v.notify) > 10*time.Second {
					log.Infof("Still %s %s '%s'... elapsed %s.\n", applyActionType(v.act), v.act.ObjectType, v.act.ObjectName, time.Since(v.start).Truncate(timeTruncate))

					v.notify = now
				}
			}

			m.Unlock()
		}
	}()

	timeInfo := pterm.NewStyle(pterm.FgWhite, pterm.Reset)

	return func(act *apiv1.ApplyAction) {
		key := applyTargetKey{ns: act.Namespace, typ: act.ObjectType, obj: act.ObjectId}

		if act.Progress == 0 {
			m.Lock()
			startMap[key] = newApplyTarget(act)
			m.Unlock()

			return
		}

		success := fmt.Sprintf("%s %s '%s'", strings.Title(applyActionType(act)), act.ObjectType, act.ObjectName)

		if act.Progress == act.Total {
			m.Lock()
			start := startMap[key]
			delete(startMap, key)
			m.Unlock()

			if start != nil {
				success += fmt.Sprintf(": %s %s", pterm.Bold.Sprint("DONE"),
					timeInfo.Sprintf("- took %s.", time.Since(start.start).Truncate(timeTruncate)))
			}
		} else {
			success += fmt.Sprintf(": step %d of %d", act.Progress, act.Total)
		}

		log.Successln(success)

		if act.Progress == act.Total || act.Type == apiv1.PlanType_PLAN_TYPE_RECREATE {
			p.Increment()
		}
	}
}
