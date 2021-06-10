package actions

import (
	"fmt"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/pterm/pterm"
)

type change struct {
	app    *types.App
	dep    *types.Dependency
	plugin *plugins.Plugin
	obj    string
	info   map[string][]*changeInfo
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

type changeInfo struct {
	idx   int
	typ   types.PlanType
	steps int
	desc  string
}

func (i *changeInfo) Type() string {
	switch i.typ {
	case types.PlanCreate:
		return pterm.Green("+")
	case types.PlanRecreate:
		return pterm.Red("~")
	case types.PlanUpdate:
		return pterm.Yellow("~")
	case types.PlanDelete:
		return pterm.Red("-")
	}

	panic("unknown type")
}

func (i *changeInfo) Info() string {
	info := fmt.Sprintf("%s - %d step(s)", i.desc, i.steps)

	if i.idx >= 0 {
		return fmt.Sprintf("[%d] %s", i.idx, info)
	}

	return info
}

func computeChangeInfo(actions []*types.PlanAction) (ret map[string][]*changeInfo) {
	ret = make(map[string][]*changeInfo)

	for _, act := range actions {
		ret[act.Key] = append(ret[act.Key], &changeInfo{
			idx:   act.Index,
			typ:   act.Type,
			desc:  act.Description,
			steps: act.TotalSteps(),
		})
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
		for _, infos := range c.info {
			for _, i := range infos {
				switch i.typ {
				case types.PlanCreate:
					add++
				case types.PlanRecreate, types.PlanUpdate:
					change++
				case types.PlanDelete:
					destroy++
				}
			}
		}
	}

	return add, change, destroy
}

func calculateTotalSteps(chg []*change) int {
	steps := 0

	for _, c := range chg {
		for _, infos := range c.info {
			for _, i := range infos {
				steps += i.steps
			}
		}
	}

	return steps
}

func planPrompt(log logger.Logger, deploy, dns []*change) (empty, canceled bool) {
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

		for _, is := range chg.info {
			for _, i := range is {
				info += fmt.Sprintf("    %s %s\n", i.Type(), pterm.Normal(i.Info()))
			}
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

		for _, is := range chg.info {
			for _, i := range is {
				info += fmt.Sprintf("    %s %s\n", i.Type(), pterm.Normal(i.Info()))
			}
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

func findChangeTarget(changes []*change, id string, typ types.TargetType) *change {
	for _, chg := range changes {
		switch typ {
		case types.TargetTypeApp:
			if chg.app != nil && chg.app.ID == id {
				return chg
			}
		case types.TargetTypeDependency:
			if chg.dep != nil && chg.dep.ID == id {
				return chg
			}
		case types.TargetTypePlugin:
			if chg.plugin != nil && chg.plugin.Name == id {
				return chg
			}
		}
	}

	return nil
}

func findChangeInfo(change *change, obj string, idx int) *changeInfo {
	if change == nil {
		return nil
	}

	for _, info := range change.info[obj] {
		if info.idx == idx {
			return info
		}
	}

	return nil
}

type targetUnique struct {
	id, obj string
	idx     int
	typ     types.TargetType
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
		key := targetUnique{id: act.TargetID, idx: act.Index, typ: act.TargetType, obj: act.Object}

		if act.Progress == 0 {
			startMap[key] = time.Now()
			return
		}

		desc := act.Description
		if len(desc) > 50 {
			desc = desc[:50] + ".."
		}

		m.Lock()

		if act.Index >= 0 {
			desc = fmt.Sprintf("[%d] %s", act.Index, desc)
		}

		p.Title = fmt.Sprintf("Applying... %s", desc)
		p.Add(0) // force title update

		if act.Progress == act.Total {
			showSuccessInfo(log, changes, act, startMap[key])
		}

		p.Increment()
		m.Unlock()
	}
}

func showSuccessInfo(log logger.Logger, changes []*change, act *types.ApplyAction, start time.Time) {
	chg := findChangeTarget(changes, act.TargetID, act.TargetType)
	if chg == nil {
		return
	}

	info := findChangeInfo(chg, act.Object, act.Index)
	if info == nil {
		return
	}

	success := fmt.Sprintf("%s: %s", chg.Name(), info.desc)

	if !start.IsZero() {
		success += fmt.Sprintf(" took %s.", time.Since(start).Truncate(timeTruncate))
	}

	log.Successln(success)
}
