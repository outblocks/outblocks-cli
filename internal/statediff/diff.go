package statediff

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type Diff struct {
	Apps            *MapDiff
	Dependencies    *MapDiff
	DNSRecords      *MapDiff
	DomainsInfo     *MapDiff
	PluginsRegistry map[string]*RegistryDiff
	PluginsOther    map[string]*MapDiff
	PluginsDelete   map[string]struct{}
}

func (s *Diff) IsEmpty() bool {
	return s.Apps.IsEmpty() && s.Dependencies.IsEmpty() && s.DNSRecords.IsEmpty() && s.DomainsInfo.IsEmpty() && len(s.PluginsRegistry) == 0 && len(s.PluginsOther) == 0 && len(s.PluginsDelete) == 0
}

func (s *Diff) Apply(state *types.StateData) error {
	if state.Apps == nil {
		state.Apps = make(map[string]*apiv1.AppState)
	}

	apps := toJSONObject(state.Apps)
	state.Apps = make(map[string]*apiv1.AppState)

	s.Apps.Apply(apps)

	fromJSONObject(apps, &state.Apps)

	if state.Dependencies == nil {
		state.Dependencies = make(map[string]*apiv1.DependencyState)
	}

	deps := toJSONObject(state.Dependencies)
	state.Dependencies = make(map[string]*apiv1.DependencyState)

	s.Dependencies.Apply(deps)

	fromJSONObject(deps, &state.Dependencies)

	// DNS Records.
	if state.DNSRecords == nil {
		state.DNSRecords = make(types.DNSRecordMap)
	}

	s.DNSRecords.Apply(map[types.DNSRecordKey]types.DNSRecordValue(state.DNSRecords))

	// Domains info.
	if state.DomainsInfo == nil {
		state.DomainsInfo = make([]*apiv1.DomainInfo, 0)
	}

	domainsInfoMap := domainsInfoAsMap(state.DomainsInfo)
	s.DomainsInfo.Apply(domainsInfoMap)

	state.DomainsInfo = make([]*apiv1.DomainInfo, 0, len(domainsInfoMap))

	for _, d := range domainsInfoMap {
		state.DomainsInfo = append(state.DomainsInfo, d)
	}

	// Plugin state.
	if state.Plugins == nil {
		state.Plugins = make(map[string]*types.PluginState)
	}

	for k, v := range s.PluginsRegistry {
		if _, ok := state.Plugins[k]; !ok {
			state.Plugins[k] = &types.PluginState{}
		}

		err := v.Apply(state.Plugins[k])
		if err != nil {
			return merry.Errorf("error applying plugin registry diff: %w", err)
		}
	}

	for k, v := range s.PluginsOther {
		if _, ok := state.Plugins[k]; !ok {
			state.Plugins[k] = &types.PluginState{}
		}

		if state.Plugins[k].Other == nil {
			state.Plugins[k].Other = make(map[string]json.RawMessage)
		}

		v.Apply(state.Plugins[k].Other)
	}

	for k := range s.PluginsDelete {
		delete(state.Plugins, k)
	}

	return nil
}

func (s *Diff) String() string {
	var out string

	if !s.Apps.IsEmpty() {
		out += fmt.Sprintf("Apps:\n%s\n\n", util.IndentString(s.Apps.String(), "  "))
	}

	if !s.Dependencies.IsEmpty() {
		out += fmt.Sprintf("Deps:\n%s\n\n", util.IndentString(s.Dependencies.String(), "  "))
	}

	if !s.DNSRecords.IsEmpty() {
		out += fmt.Sprintf("DNS:\n%s\n\n", util.IndentString(s.DNSRecords.String(), "  "))
	}

	if !s.DomainsInfo.IsEmpty() {
		out += fmt.Sprintf("DomainsInfo:\n%s\n\n", util.IndentString(s.DomainsInfo.String(), "  "))
	}

	for k, v := range s.PluginsRegistry {
		out += fmt.Sprintf("Plugin Registry for %s:\n%s\n\n", k, util.IndentString(v.String(), "  "))
	}

	for k, v := range s.PluginsOther {
		out += fmt.Sprintf("Plugin Other for %s:\n%s\n\n", k, util.IndentString(v.String(), "  "))
	}

	for k := range s.PluginsDelete {
		out += fmt.Sprintf("Plugin Deleted: %s", k)
	}

	return strings.TrimRight(out, "\n")
}

func domainsInfoAsMap(domains []*apiv1.DomainInfo) map[string]*apiv1.DomainInfo {
	m := make(map[string]*apiv1.DomainInfo, len(domains))

	for _, d := range domains {
		if d == nil {
			continue
		}

		m[strings.Join(d.Domains, ";")] = d
	}

	return m
}

func toJSONObject(i interface{}) map[string]interface{} {
	b, _ := json.Marshal(i)

	var ret map[string]interface{}
	_ = json.Unmarshal(b, &ret)

	return ret
}

func fromJSONObject(i, out interface{}) {
	b, _ := json.Marshal(i)
	_ = json.Unmarshal(b, out)
}

func New(state1, state2 *types.StateData) (*Diff, error) {
	apps, err := NewMapDiff(toJSONObject(state1.Apps), toJSONObject(state2.Apps), 2)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	deps, err := NewMapDiff(toJSONObject(state1.Dependencies), toJSONObject(state2.Dependencies), 2)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	dnsRecords, err := NewMapDiff(map[types.DNSRecordKey]types.DNSRecordValue(state1.DNSRecords), map[types.DNSRecordKey]types.DNSRecordValue(state2.DNSRecords), 1)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	domains, err := NewMapDiff(domainsInfoAsMap(state1.DomainsInfo), domainsInfoAsMap(state2.DomainsInfo), 1)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	regDiffMap := make(map[string]*RegistryDiff)
	otherDiffMap := make(map[string]*MapDiff)

	for k, v2 := range state2.Plugins {
		v1 := state1.Plugins[k]
		if v1 == nil {
			v1 = &types.PluginState{}
		}

		rdiff, err := NewRegistryDiff(v1, v2)
		if err != nil {
			return nil, merry.Wrap(err)
		}

		if !rdiff.IsEmpty() {
			regDiffMap[k] = rdiff
		}

		mdiff, err := NewMapDiff(v1.Other, v2.Other, 1)
		if err != nil {
			return nil, merry.Wrap(err)
		}

		if !mdiff.IsEmpty() {
			otherDiffMap[k] = mdiff
		}
	}

	pluginsDeleted := make(map[string]struct{})

	for k := range state1.Plugins {
		if _, ok := state2.Plugins[k]; ok {
			continue
		}

		pluginsDeleted[k] = struct{}{}
	}

	return &Diff{
		Apps:            apps,
		Dependencies:    deps,
		DNSRecords:      dnsRecords,
		DomainsInfo:     domains,
		PluginsRegistry: regDiffMap,
		PluginsOther:    otherDiffMap,
		PluginsDelete:   pluginsDeleted,
	}, nil
}
