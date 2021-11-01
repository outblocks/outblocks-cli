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
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/pterm/pterm"
)

type changeID struct {
	planType   types.PlanType
	objectType string
}

type change struct {
	app    config.App
	dep    *config.Dependency
	plugin *plugins.Plugin
	obj    string
	info   map[changeID][]string
}

func (i *change) Name() string {
	if i.app != nil {
		return fmt.Sprintf("%s App '%s'", strings.Title(i.app.Type()), i.app.Name())
	}

	if i.dep != nil {
		return fmt.Sprintf("Dependency '%s'", i.dep.Name)
	}

	return fmt.Sprintf("Plugin '%s' %s", i.plugin.Name, i.obj)
}

func (i *changeID) Type() string {
	switch i.planType {
	case types.PlanCreate:
		return pterm.Green("+ add")
	case types.PlanRecreate:
		return pterm.Red("~ recreate")
	case types.PlanUpdate:
		return pterm.Yellow("~ update")
	case types.PlanProcess:
		return pterm.Blue("~ process")
	case types.PlanDelete:
		return pterm.Red("- delete")
	}

	panic("unknown type")
}

func computeChangeInfo(appMap map[string]config.App, depMap map[string]*config.Dependency, plugin *plugins.Plugin, actions []*types.PlanAction) (changes []*change) {
	changesMap := make(map[string]*change)

	for _, act := range actions {
		chg := changesMap[act.Namespace]

		if chg == nil {
			if app, ok := appMap[act.Namespace]; ok && act.Source == types.SourceApp {
				chg = &change{
					app: app,
				}
			} else if dep, ok := depMap[act.Namespace]; ok && act.Source == types.SourceDependency {
				chg = &change{
					dep: dep,
				}
			} else {
				chg = &change{
					plugin: plugin,
					obj:    act.Namespace,
				}
			}

			chg.info = make(map[changeID][]string)
			changesMap[act.Namespace] = chg
			changes = append(changes, chg)
		}

		key := changeID{
			planType:   act.Type,
			objectType: act.ObjectType,
		}

		chg.info[key] = append(chg.info[key], act.ObjectName)
	}

	return changes
}

func computeChange(appMap map[string]config.App, depMap map[string]*config.Dependency, planMap map[*plugins.Plugin]*plugin_go.PlanResponse) (deploy, dns []*change) {
	for plugin, p := range planMap {
		if p.DeployPlan != nil {
			deploy = computeChangeInfo(appMap, depMap, plugin, p.DeployPlan.Actions)
		}

		if p.DNSPlan != nil {
			dns = computeChangeInfo(appMap, depMap, plugin, p.DeployPlan.Actions)
		}
	}

	return deploy, dns
}

func calculateTotal(chg []*change) (add, change, process, destroy int) {
	for _, c := range chg {
		for chID, objs := range c.info {
			switch chID.planType {
			case types.PlanCreate:
				add += len(objs)
			case types.PlanRecreate, types.PlanUpdate:
				change += len(objs)
			case types.PlanDelete:
				destroy += len(objs)
			case types.PlanProcess:
				process += len(objs)
			}
		}
	}

	return add, change, process, destroy
}

func calculateTotalSteps(chg []*change) int {
	steps := 0

	for _, c := range chg {
		for changeID, v := range c.info {
			if changeID.planType == types.PlanRecreate {
				// Recreate steps are doubled.
				steps += 2 * len(v)
			} else {
				steps += len(v)
			}
		}
	}

	return steps
}

func formatChangeInfo(chID changeID, objs []string) string {
	if len(objs) == 1 {
		return fmt.Sprintf("    %s %s '%s'\n", chID.Type(), chID.objectType, pterm.Normal(objs[0]))
	}

	if len(objs) <= 5 {
		return fmt.Sprintf("    %s %d of %s ['%s']\n", chID.Type(), len(objs), chID.objectType, pterm.Normal(strings.Join(objs, "', '")))
	}

	return fmt.Sprintf("    %s %d of %s\n", chID.Type(), len(objs), chID.objectType)
}

func planChangeInfo(header string, changes []*change) (info string) {
	if len(changes) == 0 {
		return ""
	}

	headerStyle := pterm.NewStyle(pterm.FgWhite, pterm.Bold)
	headerInfoStyle := pterm.NewStyle(pterm.FgWhite, pterm.Reset)

	add, change, process, destroy := calculateTotal(changes)
	info += fmt.Sprintf("%s %s\n", headerStyle.Sprintf(header), headerInfoStyle.Sprintf("(%d to add, %d to change, %d to destroy, %d to process)", add, change, destroy, process))

	for _, chg := range changes {
		info += fmt.Sprintf("  %s\n", pterm.Bold.Sprintf("\n  %s changes:", chg.Name()))

		for k, i := range chg.info {
			info += formatChangeInfo(k, i)
		}
	}

	return info
}

func planPrompt(log logger.Logger, deploy, dns []*change, approve bool) (empty, canceled bool) {
	sort.Slice(deploy, func(i, j int) bool {
		if deploy[i].app == nil && deploy[j].app != nil {
			return false
		}

		if deploy[i].app != nil && deploy[j].app == nil {
			return true
		}

		return deploy[i].Name() < deploy[j].Name()
	})

	info := []string{"Outblocks will perform the following actions to your architecture:"}
	empty = true

	// Deployment
	deployInfo := planChangeInfo("Deployment:", deploy)
	if deployInfo != "" {
		empty = false

		info = append(info, deployInfo)
	}

	// DNS
	dnsInfo := planChangeInfo("DNS:", dns)
	if dnsInfo != "" {
		empty = false

		info = append(info, dnsInfo)
	}

	if empty {
		log.Println("No changes detected.")

		return true, false
	}

	if approve {
		return false, false
	}

	log.Println(strings.Join(info, "\n\n"))

	proceed := false
	prompt := &survey.Confirm{
		Message: "Do you want to perform these actions?",
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
	act           *types.ApplyAction
	start, notify time.Time
}

func newApplyTarget(act *types.ApplyAction) *applyTarget {
	t := time.Now()

	return &applyTarget{
		act:    act,
		start:  t,
		notify: t,
	}
}

func applyActionType(act *types.ApplyAction) string {
	switch act.Type {
	case types.PlanCreate:
		return "creating"
	case types.PlanDelete:
		return "deleting"
	case types.PlanUpdate:
		return "updating"
	case types.PlanRecreate:
		return "recreating"
	case types.PlanProcess:
		return "processing"
	}

	return "unknown"
}

func applyProgress(log logger.Logger, deployChanges, dnsChanges []*change) func(*types.ApplyAction) {
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

	return func(act *types.ApplyAction) {
		key := applyTargetKey{ns: act.Namespace, typ: act.ObjectType, obj: act.ObjectID}

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

		if act.Progress == act.Total || act.Type == types.PlanRecreate {
			p.Increment()
		}
	}
}
