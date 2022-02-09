package actions

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/registry"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/r3labs/diff/v2"
)

type stateDiff struct {
	apps            diff.Changelog
	deps            diff.Changelog
	dnsRecords      diff.Changelog
	pluginsRegistry diff.Changelog
	pluginsState    diff.Changelog
	domainsInfo     diff.Changelog
}

func (s *stateDiff) IsEmpty() bool {
	return len(s.apps) == 0 && len(s.deps) == 0 && len(s.dnsRecords) == 0 && len(s.pluginsRegistry) == 0 && len(s.pluginsState) == 0
}

func firstPatchLogError(plog diff.PatchLog) error {
	for _, l := range plog {
		if l.Errors != nil {
			return l.Errors
		}
	}

	return nil
}

func (s *stateDiff) Apply(state *types.StateData) error { // nolint:gocyclo
	if state.Apps == nil {
		state.Apps = make(map[string]*apiv1.AppState)
	}

	ret := diff.Patch(s.apps, &state.Apps)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.apps: %w", firstPatchLogError(ret))
	}

	if state.Dependencies == nil {
		state.Dependencies = make(map[string]*apiv1.DependencyState)
	}

	ret = diff.Patch(s.deps, &state.Dependencies)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.dependencies: %w", firstPatchLogError(ret))
	}

	// Domains info.
	domainsInfo := domainsInfoAsMap(state.DomainsInfo)

	ret = diff.Patch(s.domainsInfo, &domainsInfo)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.domainsinfo: %w", firstPatchLogError(ret))
	}

	state.DomainsInfo = make([]*apiv1.DomainInfo, 0, len(domainsInfo))

	for _, d := range domainsInfo {
		state.DomainsInfo = append(state.DomainsInfo, d)
	}

	// DNS Records.
	dnsRecords := dnsRecordsAsMap(state.DNSRecords)

	ret = diff.Patch(s.dnsRecords, &dnsRecords)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.dnsrecords: %w", firstPatchLogError(ret))
	}

	state.DNSRecords = make(types.DNSRecordMap)

	for _, rec := range dnsRecords {
		state.DNSRecords[rec.Key] = rec.Val
	}

	// Plugin state.
	if state.Plugins == nil {
		state.Plugins = make(map[string]*types.PluginState)
	}

	pluginState, err := pluginsStateWithoutRegistry(state.Plugins)
	if err != nil {
		return err
	}

	ret = diff.Patch(s.pluginsState, pluginState)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.plugins_state: %w", firstPatchLogError(ret))
	}

	pluginRegistry, err := pluginsRegistryAsMap(state.Plugins)
	if err != nil {
		return err
	}

	ret = diff.Patch(s.pluginsRegistry, &pluginRegistry)
	if ret.HasErrors() {
		return merry.Errorf("error applying patch on state.plugins registry: %w", firstPatchLogError(ret))
	}

	for k, v := range pluginState {
		vals := make(map[string]json.RawMessage, len(v))

		if _, ok := state.Plugins[k]; !ok {
			state.Plugins[k] = &types.PluginState{}
		}

		for mapk, mapv := range v {
			raw, _ := json.Marshal(mapv)
			vals[mapk] = raw
		}

		state.Plugins[k].Other = vals
	}

	for k, v := range pluginRegistry {
		resources := make([]*registry.ResourceSerialized, 0, len(v))

		if _, ok := state.Plugins[k]; !ok {
			state.Plugins[k] = &types.PluginState{}
		}

		for _, r := range v {
			resources = append(resources, r)
		}

		sort.Slice(resources, func(i, j int) bool {
			return resources[i].Less(&resources[j].ResourceID)
		})

		out, err := json.Marshal(resources)
		if err != nil {
			return merry.Errorf("error marshaling plugin registry: %w", err)
		}

		state.Plugins[k].Registry = out
	}

	return nil
}

func formatIndentedJSON(i interface{}) string {
	out, _ := json.MarshalIndent(i, "", "  ")
	return string(out)
}

func (s *stateDiff) String() string {
	var out string

	if len(s.apps) != 0 {
		out += fmt.Sprintf("Apps = %s\n", formatIndentedJSON(s.apps))
	}

	if len(s.deps) != 0 {
		out += fmt.Sprintf("Deps = %s\n", formatIndentedJSON(s.deps))
	}

	if len(s.dnsRecords) != 0 {
		out += fmt.Sprintf("DNS = %s\n", formatIndentedJSON(s.dnsRecords))
	}

	if len(s.domainsInfo) != 0 {
		out += fmt.Sprintf("DomainsInfo = %s\n", formatIndentedJSON(s.domainsInfo))
	}

	if len(s.pluginsRegistry) != 0 {
		out += fmt.Sprintf("Registry = %s\n", formatIndentedJSON(s.pluginsRegistry))
	}

	if len(s.pluginsState) != 0 {
		out += fmt.Sprintf("State = %s\n", formatIndentedJSON(s.pluginsState))
	}

	return out
}

func pluginsRegistryAsMap(plugins map[string]*types.PluginState) (map[string]map[string]*registry.ResourceSerialized, error) {
	pluginRegistry := make(map[string]map[string]*registry.ResourceSerialized, len(plugins))

	for k, v := range plugins {
		var loaded []*registry.ResourceSerialized

		if v == nil || v.Registry == nil {
			continue
		}

		err := json.Unmarshal(v.Registry, &loaded)
		if err != nil {
			return nil, err
		}

		out := make(map[string]*registry.ResourceSerialized, len(loaded))

		for _, r := range loaded {
			out[fmt.Sprintf("%s::%s::%s::%s", r.ResourceID.Source, r.ResourceID.Namespace, r.ResourceID.ID, r.ResourceID.Type)] = r
		}

		pluginRegistry[k] = out
	}

	return pluginRegistry, nil
}

func domainsInfoAsMap(domains []*apiv1.DomainInfo) map[string]*apiv1.DomainInfo {
	m := make(map[string]*apiv1.DomainInfo, len(domains))

	for _, d := range domains {
		m[strings.Join(d.Domains, ";")] = d
	}

	return m
}

type dnsRecordValue struct {
	Val types.DNSRecordValue
	Key types.DNSRecordKey
}

func dnsRecordsAsMap(records types.DNSRecordMap) map[string]dnsRecordValue {
	ret := make(map[string]dnsRecordValue, len(records))

	for k, v := range records {
		ret[fmt.Sprintf("%s::%d", k.Record, k.Type)] = dnsRecordValue{
			Key: k,
			Val: v,
		}
	}

	return ret
}

func pluginsStateWithoutRegistry(plugins map[string]*types.PluginState) (map[string]map[string]interface{}, error) {
	ret := make(map[string]map[string]interface{}, len(plugins))

	for k, v := range plugins {
		if v == nil || len(v.Other) == 0 {
			continue
		}

		ret[k] = make(map[string]interface{}, len(v.Other))

		for otherk, otherv := range v.Other {
			var val interface{}
			err := json.Unmarshal(otherv, &val)

			if err != nil {
				return nil, err
			}

			ret[k][otherk] = val
		}
	}

	return ret, nil
}

func diffOnlyExported(path []string, parent reflect.Type, field reflect.StructField) bool { // nolint:gocritic
	return field.IsExported()
}

func computeDiff(in1, in2 interface{}) (diff.Changelog, error) {
	return diff.Diff(in1, in2, diff.Filter(diffOnlyExported), diff.SliceOrdering(true), diff.DisableStructValues(), diff.FlattenEmbeddedStructs())
}

func computeStateDiff(state1, state2 *types.StateData) (*stateDiff, error) {
	apps, err := computeDiff(state1.Apps, state2.Apps)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	deps, err := computeDiff(state1.Dependencies, state2.Dependencies)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	dnsRecords, err := computeDiff(dnsRecordsAsMap(state1.DNSRecords), dnsRecordsAsMap(state2.DNSRecords))
	if err != nil {
		return nil, merry.Wrap(err)
	}

	state1DomainsInfo := domainsInfoAsMap(state1.DomainsInfo)
	state2DomainsInfo := domainsInfoAsMap(state2.DomainsInfo)

	domainsInfo, err := computeDiff(state1DomainsInfo, state2DomainsInfo)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	state1PluginRegistry, err := pluginsRegistryAsMap(state1.Plugins)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	state2PluginRegistry, err := pluginsRegistryAsMap(state2.Plugins)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	pluginsRegistry, err := computeDiff(state1PluginRegistry, state2PluginRegistry)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	state1PluginState, err := pluginsStateWithoutRegistry(state1.Plugins)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	state2PluginState, err := pluginsStateWithoutRegistry(state2.Plugins)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	pluginsState, err := computeDiff(state1PluginState, state2PluginState)
	if err != nil {
		return nil, merry.Wrap(err)
	}

	return &stateDiff{
		apps:            apps,
		deps:            deps,
		dnsRecords:      dnsRecords,
		domainsInfo:     domainsInfo,
		pluginsRegistry: pluginsRegistry,
		pluginsState:    pluginsState,
	}, nil
}
