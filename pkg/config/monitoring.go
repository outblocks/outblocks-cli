package config

import (
	"fmt"
	"net/url"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type MonitoringChannel struct {
	Type  string                 `json:"type"`
	Other map[string]interface{} `yaml:"-,remain"`
}

func (c *MonitoringChannel) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.Type, validation.Required),
	)
}

func (c *MonitoringChannel) Proto() *apiv1.MonitoringChannel {
	return &apiv1.MonitoringChannel{
		Type:       c.Type,
		Properties: plugin_util.MustNewStruct(c.Other),
	}
}

type MonitoringTarget struct {
	TargetURL string   `json:"url"`
	Frequency int      `json:"frequency,omitempty"`
	Locations []string `json:"locations,omitempty"`

	url *url.URL
}

func (t *MonitoringTarget) Validate() error {
	return validation.ValidateStruct(t,
		validation.Field(&t.TargetURL, validation.Required),
	)
}

func (t *MonitoringTarget) URL() *url.URL {
	return t.url
}

func (t *MonitoringTarget) Proto() *apiv1.MonitoringTarget {
	return &apiv1.MonitoringTarget{
		Url:       t.URL().String(),
		Frequency: int32(t.Frequency),
		Locations: t.Locations,
	}
}

type Monitoring struct {
	PluginWanted string                 `json:"plugin,omitempty"`
	Channels     []*MonitoringChannel   `json:"channels,omitempty"`
	Targets      []*MonitoringTarget    `json:"targets,omitempty"`
	Other        map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (m *Monitoring) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Channels),
		validation.Field(&m.Targets),
	)
}

func (m *Monitoring) Normalize(cfg *Project) error {
	var err error

	for _, ch := range m.Channels {
		ch.Type = strings.ToLower(ch.Type)
	}

	for i, t := range m.Targets {
		t.TargetURL = strings.ToLower(t.TargetURL)

		for i, v := range t.Locations {
			t.Locations[i] = strings.ToLower(v)
		}

		t.url, err = ParseURL(t.TargetURL, true)
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.monitoring.targets.%d.url", i), "url is invalid")
		}

		if t.Frequency == 0 {
			t.Frequency = 5
		}

		if len(t.Locations) == 0 {
			t.Locations = []string{"all"}
		}
	}

	return nil
}

func (m *Monitoring) Check(cfg *Project) error {
	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionMonitoring) {
			continue
		}

		if m.PluginWanted != "" && m.PluginWanted != plug.Name {
			continue
		}

		m.plugin = plug
	}

	return nil
}

func (m *Monitoring) Plugin() *plugins.Plugin {
	return m.plugin
}

func (m *Monitoring) Proto() *apiv1.MonitoringData {
	plugin := ""

	if m.plugin != nil {
		plugin = m.plugin.Name
	}

	targets := make([]*apiv1.MonitoringTarget, len(m.Targets))
	channels := make([]*apiv1.MonitoringChannel, len(m.Channels))

	for i, v := range m.Targets {
		targets[i] = v.Proto()
	}

	for i, v := range m.Channels {
		channels[i] = v.Proto()
	}

	return &apiv1.MonitoringData{
		Plugin:   plugin,
		Targets:  targets,
		Channels: channels,
	}
}
