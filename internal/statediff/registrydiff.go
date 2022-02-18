package statediff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-plugin-go/registry"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type ResourceDiff struct {
	Res          *registry.ResourceSerialized
	Properties   *MapDiff
	Dependencies *MapDiff
	DependedBy   *MapDiff
}

type RegistryDiff struct {
	update map[registry.ResourceID]ResourceDiff
	delete map[registry.ResourceID]struct{}
}

func NewRegistryDiff(m1, m2 *types.PluginState) (*RegistryDiff, error) {
	loaded1, err := registryMap(m1.Registry)
	if err != nil {
		return nil, err
	}

	loaded2, err := registryMap(m2.Registry)
	if err != nil {
		return nil, err
	}

	d, err := NewMapDiff(loaded1, loaded2, 1)
	if err != nil {
		return nil, err
	}

	ret := &RegistryDiff{
		update: make(map[registry.ResourceID]ResourceDiff, len(d.update)),
		delete: make(map[registry.ResourceID]struct{}, len(d.delete)),
	}

	for k := range d.delete {
		ret.delete[k.(registry.ResourceID)] = struct{}{}
	}

	for k, v := range d.update {
		rid := k.(registry.ResourceID)
		dest := v.(*registry.ResourceSerialized)
		org, ok := loaded1[rid]

		if !ok {
			ret.update[rid] = ResourceDiff{
				Res: dest,
			}

			continue
		}

		propDiff, err := NewMapDiff(org.Properties, dest.Properties, 1)
		if err != nil {
			return nil, err
		}

		depsDiff, err := NewMapDiff(registrySliceToMap(org.Dependencies), registrySliceToMap(dest.Dependencies), 1)
		if err != nil {
			return nil, err
		}

		dependedByDiff, err := NewMapDiff(registrySliceToMap(org.DependedBy), registrySliceToMap(dest.DependedBy), 1)
		if err != nil {
			return nil, err
		}

		diff := ResourceDiff{
			Res:          dest,
			Properties:   propDiff,
			Dependencies: depsDiff,
			DependedBy:   dependedByDiff,
		}

		ret.update[rid] = diff
	}

	return ret, nil
}

func (d *RegistryDiff) Apply(m *types.PluginState) error {
	loaded, err := registryMap(m.Registry)
	if err != nil {
		return err
	}

	for k := range d.delete {
		delete(loaded, k)
	}

	for k, v := range d.update {
		org, ok := loaded[k]
		if !ok {
			loaded[k] = v.Res

			continue
		}

		org.ReferenceID = v.Res.ReferenceID

		if v.Properties != nil {
			if org.Properties == nil {
				org.Properties = make(map[string]interface{})
			}

			v.Properties.Apply(org.Properties)
		}

		if v.Dependencies != nil {
			deps := registrySliceToMap(org.Dependencies)

			v.Dependencies.Apply(deps)
			org.Dependencies = flattenResourceIDMap(deps)
		}

		if v.DependedBy != nil {
			deps := registrySliceToMap(org.DependedBy)

			v.DependedBy.Apply(deps)
			org.DependedBy = flattenResourceIDMap(deps)
		}

		org.ReferenceID = v.Res.ReferenceID
		org.IsNew = v.Res.IsNew
	}

	// Serialize resources.
	resources := make([]*registry.ResourceSerialized, 0, len(loaded))

	for _, r := range loaded {
		resources = append(resources, r)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Less(&resources[j].ResourceID)
	})

	b, err := json.Marshal(resources)
	if err != nil {
		return err
	}

	m.Registry = json.RawMessage(b)

	return nil
}

func (d *RegistryDiff) IsEmpty() bool {
	return len(d.delete) == 0 && len(d.update) == 0
}

func (d *RegistryDiff) String() string {
	ret := ""

	for k := range d.delete {
		ret += fmt.Sprintf("x delete: %#v\n", k)
	}

	for k, v := range d.update {
		ret += fmt.Sprintf("~ update: %#v\n", k)

		if v.Properties != nil && !v.Properties.IsEmpty() {
			ret += fmt.Sprintf("  properties\n%s\n", util.IndentString(v.Properties.String(), "    "))
		}

		if v.Dependencies != nil && !v.Dependencies.IsEmpty() {
			ret += fmt.Sprintf("  dependencies\n%s\n", util.IndentString(v.Dependencies.String(), "    "))
		}

		if v.DependedBy != nil && !v.DependedBy.IsEmpty() {
			ret += fmt.Sprintf("  depended by\n%s\n", util.IndentString(v.DependedBy.String(), "    "))
		}
	}

	return strings.TrimRight(ret, "\n")
}

func registryMap(reg json.RawMessage) (map[registry.ResourceID]*registry.ResourceSerialized, error) {
	var loaded []*registry.ResourceSerialized

	if len(reg) == 0 {
		return nil, nil
	}

	err := json.Unmarshal(reg, &loaded)
	if err != nil {
		return nil, err
	}

	ret := make(map[registry.ResourceID]*registry.ResourceSerialized, len(loaded))

	for _, e := range loaded {
		ret[e.ResourceID] = e
	}

	return ret, nil
}

func registrySliceToMap(r []registry.ResourceID) map[registry.ResourceID]struct{} {
	ret := make(map[registry.ResourceID]struct{}, len(r))

	for _, e := range r {
		ret[e] = struct{}{}
	}

	return ret
}

func flattenResourceIDMap(m map[registry.ResourceID]struct{}) []registry.ResourceID {
	ret := make([]registry.ResourceID, 0, len(m))

	for k := range m {
		ret = append(ret, k)
	}

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Less(&ret[j])
	})

	return ret
}
