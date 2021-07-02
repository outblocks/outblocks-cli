package actions

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
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
	app    *types.App
	dep    *types.Dependency
	plugin *plugins.Plugin
	obj    string
	info   map[changeID][]string
}

func (i *change) Name() string {
	if i.app != nil {
		return i.app.TargetName()
	}

	if i.dep != nil {
		return i.dep.TargetName()
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
	case types.PlanDelete:
		return pterm.Red("- delete")
	}

	panic("unknown type")
}

func computeChangeInfo(actions []*types.PlanAction) (ret map[changeID][]string) {
	ret = make(map[changeID][]string)

	for _, act := range actions {
		key := changeID{
			planType:   act.Type,
			objectType: act.ObjectType,
		}

		ret[key] = append(ret[key], act.ObjectName)
	}

	return ret
}

func computeChange(planMap map[*plugins.Plugin]*plugin_go.PlanResponse) (deploy, dns []*change) {
	for plugin, p := range planMap {
		if p.DeployPlan != nil {
			for _, act := range p.DeployPlan.Plugin {
				deploy = append(deploy, &change{
					plugin: plugin,
					obj:    act.Object,
					info:   computeChangeInfo(act.Actions),
				})
			}

			for _, app := range p.DeployPlan.Apps {
				deploy = append(deploy, &change{
					app:  app.App,
					info: computeChangeInfo(app.Actions),
				})
			}

			for _, dep := range p.DeployPlan.Dependencies {
				deploy = append(deploy, &change{
					dep:  dep.Dependency,
					info: computeChangeInfo(dep.Actions),
				})
			}
		}

		if p.DNSPlan != nil {
			for _, act := range p.DNSPlan.Plugin {
				dns = append(dns, &change{
					plugin: plugin,
					obj:    act.Object,
					info:   computeChangeInfo(act.Actions),
				})
			}

			for _, app := range p.DNSPlan.Apps {
				dns = append(dns, &change{
					app:  app.App,
					info: computeChangeInfo(app.Actions),
				})
			}

			for _, dep := range p.DNSPlan.Dependencies {
				dns = append(dns, &change{
					dep:  dep.Dependency,
					info: computeChangeInfo(dep.Actions),
				})
			}
		}
	}

	return deploy, dns
}

func calculateTotal(chg []*change) (add, change, destroy int) {
	for _, c := range chg {
		for chID, objs := range c.info {
			switch chID.planType {
			case types.PlanCreate:
				add += len(objs)
			case types.PlanRecreate, types.PlanUpdate:
				change += len(objs)
			case types.PlanDelete:
				destroy += len(objs)
			}
		}
	}

	return add, change, destroy
}

func calculateTotalSteps(chg []*change) int {
	steps := 0

	for _, c := range chg {
		for _, v := range c.info {
			steps += len(v)
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

func planPrompt(log logger.Logger, deploy, dns []*change) (empty, canceled bool) {
	sort.Slice(deploy, func(i, j int) bool {
		return deploy[i].Name() < deploy[j].Name()
	})

	info := "Outblocks will perform the following actions to your architecture:\n\n"
	empty = true

	// Deployment
	add, change, destroy := calculateTotal(deploy)
	header := pterm.NewStyle(pterm.FgWhite, pterm.Bold)
	headerInfo := pterm.NewStyle(pterm.FgWhite, pterm.Reset)

	if len(deploy) != 0 {
		info += fmt.Sprintf("%s %s\n", header.Sprintf("Deployment:"), headerInfo.Sprintf("(%d to add, %d to change, %d to destroy)", add, change, destroy))
	}

	for _, chg := range deploy {
		empty = false
		info += fmt.Sprintf("  %s\n", pterm.Bold.Sprintf("\n  %s changes:", chg.Name()))

		for k, i := range chg.info {
			info += formatChangeInfo(k, i)
		}
	}

	// DNS
	add, change, destroy = calculateTotal(dns)

	if len(dns) != 0 {
		info += fmt.Sprintf("%s %s\n", header.Sprintf("DNS:"), headerInfo.Sprintf("(%d to add, %d to change, %d to destroy)", add, change, destroy))
	}

	for _, chg := range dns {
		empty = false
		info += fmt.Sprintf("  %s\n", pterm.Bold.Sprintf("\n  %s changes:", chg.Name()))

		for k, i := range chg.info {
			info += formatChangeInfo(k, i)
		}
	}

	if empty {
		log.Infoln("No changes detected.")

		return true, false
	}

	log.Println(info)

	prompt := promptui.Prompt{
		Label:     pterm.Bold.Sprintf("Do you want to perform these actions"),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		log.Println("Apply canceled.")
		return false, true
	}

	return false, false
}

type targetUnique struct {
	ns, obj, typ string
}

func applyProgress(log logger.Logger, deployChanges, dnsChanges []*change) func(*types.ApplyAction) {
	changes := append(deployChanges, dnsChanges...) // nolint: gocritic
	total := calculateTotalSteps(changes)

	// Create progressbar as fork from the default progressbar.
	p, _ := log.ProgressBar().WithRemoveWhenDone(true).WithTotal(total).WithTitle("Applying...").Start()

	var m sync.Mutex

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for {
			<-t.C
			m.Lock()

			if !p.IsActive {
				m.Unlock()

				return
			}

			p.Add(0)
			m.Unlock()
		}
	}()

	startMap := make(map[targetUnique]time.Time)

	return func(act *types.ApplyAction) {
		key := targetUnique{ns: act.Namespace, typ: act.ObjectType, obj: act.ObjectID}

		if act.Progress == 0 {
			startMap[key] = time.Now()
			return
		}

		m.Lock()

		var t, success string

		switch act.Type {
		case types.PlanCreate:
			t = "creating"
		case types.PlanDelete:
			t = "deleting"
		case types.PlanUpdate:
			t = "updating"
		case types.PlanRecreate:
			t = "recreating"
		}

		success = fmt.Sprintf("%s %s '%s'", t, act.ObjectType, act.ObjectName)

		if act.Progress == act.Total {
			start := startMap[key]

			if !start.IsZero() {
				success += fmt.Sprintf(": %s - took %s.", pterm.Bold.Sprint("DONE"), time.Since(start).Truncate(timeTruncate))
			}
		} else {
			success += fmt.Sprintf(": step %d of %d", act.Progress, act.Total)
		}

		log.Successln(success)

		if act.Progress == act.Total {
			p.Increment()
		}

		m.Unlock()
	}
}
