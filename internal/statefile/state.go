package statefile

import (
	"encoding/json"

	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

func PluginStateFromProto(in *apiv1.PluginState) *PluginState {
	other := make(map[string]json.RawMessage, len(in.Other))
	for k, v := range in.Other {
		other[k] = v
	}

	return &PluginState{
		Registry: in.Registry,
		Other:    other,
	}
}

type PluginState struct {
	Registry json.RawMessage            `json:"registry,omitempty"`
	Other    map[string]json.RawMessage `json:"other,omitempty"`
}

func (p *PluginState) Proto() *apiv1.PluginState {
	if p == nil {
		return &apiv1.PluginState{
			Other: make(map[string][]byte),
		}
	}

	other := make(map[string][]byte, len(p.Other))
	for k, v := range p.Other {
		other[k] = v
	}

	return &apiv1.PluginState{
		Registry: p.Registry,
		Other:    other,
	}
}

type DNSRecordKey struct {
	Record string
	Type   apiv1.DNSRecord_Type
}

type DNSRecordValue struct {
	Value   string
	Created bool
}

type DNSRecordMap map[DNSRecordKey]DNSRecordValue

func (m DNSRecordMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.List())
}

func (m DNSRecordMap) List() []*apiv1.DNSRecord {
	out := make([]*apiv1.DNSRecord, 0, len(m))

	for k, v := range m {
		out = append(out, &apiv1.DNSRecord{
			Record:  k.Record,
			Type:    k.Type,
			Value:   v.Value,
			Created: v.Created,
		})
	}

	return out
}

func (m DNSRecordMap) UnmarshalJSON(b []byte) error {
	var out []*apiv1.DNSRecord

	err := json.Unmarshal(b, &out)
	if err != nil {
		return err
	}

	for _, v := range out {
		m[DNSRecordKey{
			Record: v.Record,
			Type:   v.Type,
		}] = DNSRecordValue{
			Value:   v.Value,
			Created: v.Created,
		}
	}

	return nil
}

type StateData struct {
	Version      int                               `json:"version"`
	Apps         map[string]*apiv1.AppState        `json:"apps"`
	Dependencies map[string]*apiv1.DependencyState `json:"dependencies"`
	Plugins      map[string]*PluginState           `json:"plugins_state"` // plugin name -> object -> state

	DNSRecords  DNSRecordMap          `json:"dns_records"`
	DomainsInfo []*apiv1.DomainInfo   `json:"domains_info"`
	Monitoring  *apiv1.MonitoringData `json:"monitoring"`
}

func NewStateData() *StateData {
	return &StateData{
		Version:      latestStateVersion,
		Apps:         make(map[string]*apiv1.AppState),
		Plugins:      make(map[string]*PluginState),
		Dependencies: make(map[string]*apiv1.DependencyState),
		DNSRecords:   make(DNSRecordMap),
	}
}

func (d *StateData) IsEmpty() bool {
	return len(d.Apps) == 0 && len(d.Dependencies) == 0 && len(d.Plugins) == 0 && len(d.DNSRecords) == 0 && len(d.DomainsInfo) == 0
}

func (d *StateData) Reset() {
	d.Apps = make(map[string]*apiv1.AppState)
	d.Dependencies = make(map[string]*apiv1.DependencyState)
	d.DNSRecords = make(DNSRecordMap)
}

func (d *StateData) AddDNSRecord(v *apiv1.DNSRecord) {
	d.DNSRecords[DNSRecordKey{
		Record: v.Record,
		Type:   v.Type,
	}] = DNSRecordValue{
		Value:   v.Value,
		Created: v.Created,
	}
}

func (d *StateData) DeepCopy() *StateData {
	b, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	d2 := NewStateData()

	err = json.Unmarshal(b, &d2)
	if err != nil {
		panic(err)
	}

	return d2
}
