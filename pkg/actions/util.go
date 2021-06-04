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
	info   map[string]*changeInfo
}

func (i *change) Name() string {
	if i.app != nil {
		return i.app.TargetName()
	}

	if i.dep != nil {
		return i.dep.TargetName()
	}

	return fmt.Sprintf("Plugin '%s'", i.plugin.Name)
}

type changeInfo struct {
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

func computeChangeInfo(acts map[string]*types.PlanAction) (ret map[string]*changeInfo) {
	ret = make(map[string]*changeInfo, len(acts))

	for obj, act := range acts {
		ret[obj] = &changeInfo{
			typ:   act.Type,
			desc:  act.Description,
			steps: act.TotalSteps(),
		}
	}

	return ret
}

func computeChange(planMap map[*plugins.Plugin]*plugin_go.PlanResponse) (deploy, dns []*change) {
	for plugin, p := range planMap {
		if p.DeployPlan != nil {
			for _, act := range p.DeployPlan.Plugin {
				deploy = append(deploy, &change{
					plugin: plugin,
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
		for _, i := range c.info {
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

	return add, change, destroy
}

func calculateTotalSteps(chg []*change) int {
	steps := 0

	for _, c := range chg {
		for _, i := range c.info {
			steps += i.steps
		}
	}

	return steps
}

func planPrompt(log logger.Logger, deploy, dns []*change) bool {
	info := "Outblocks will perform the following actions:\n\n"
	empty := true

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

		for _, i := range chg.info {
			info += fmt.Sprintf("    %s %s - %d step(s)\n", i.Type(), pterm.Normal(i.desc), i.steps)
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

		for _, i := range chg.info {
			info += fmt.Sprintf("    %s %s\n", i.Type(), pterm.Normal(i.desc))
		}
	}

	if empty {
		log.Infoln("No changes detected.")

		return false
	}

	log.Println(info)

	prompt := promptui.Prompt{
		Label:     pterm.Bold.Sprintf("Do you want to perform these actions"),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		log.Println("Apply canceled.")
		return false
	}

	return true
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

type targetUnique struct {
	id, obj string
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
		key := targetUnique{id: act.TargetID, typ: act.TargetType, obj: act.Object}

		if act.Progress == 0 {
			startMap[key] = time.Now()
			return
		}

		desc := act.Description
		if len(desc) > 50 {
			desc = desc[:50] + ".."
		}

		m.Lock()

		p.Title = fmt.Sprintf("Applying... %s - (%d of %d)", desc, act.Progress, act.Total)
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

	info := chg.info[act.Object]

	if info == nil {
		return
	}

	success := fmt.Sprintf("%s (%s): %s", chg.Name(), act.Object, info.desc)

	if !start.IsZero() {
		success += fmt.Sprintf(" took %s.", time.Since(start).Truncate(timeTruncate))
	}

	log.Successln(success)
}
