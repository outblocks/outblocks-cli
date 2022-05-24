package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type SSLInfo struct {
	Cert     string `json:"cert,omitempty"`
	Key      string `json:"key,omitempty"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`

	loadedCert string
	loadedKey  string
}

type DNS struct {
	Domain  string                 `json:"domain,omitempty"`
	Domains []string               `json:"domains,omitempty"`
	Plugin  string                 `json:"plugin,omitempty"`
	SSLInfo *SSLInfo               `json:"ssl,omitempty"`
	Other   map[string]interface{} `yaml:"-,remain"`

	dnsPlugin *plugins.Plugin
}

func (s *DNS) DNSPlugin() *plugins.Plugin {
	return s.dnsPlugin
}

func (s *DNS) Proto() *apiv1.DomainInfo {
	dnsPlugin := ""

	if s.dnsPlugin != nil {
		dnsPlugin = s.dnsPlugin.Name
	}

	return &apiv1.DomainInfo{
		Domains:   s.Domains,
		Cert:      s.SSLInfo.loadedCert,
		Key:       s.SSLInfo.loadedKey,
		DnsPlugin: dnsPlugin,
		Other:     plugin_util.MustNewStruct(s.Other),
	}
}

func (s *DNS) Normalize(i int, cfg *Project) error {
	if s.Domain != "" {
		s.Domains = append(s.Domains, s.Domain)
		for i, v := range s.Domains {
			s.Domains[i] = strings.ToLower(v)
		}
	}

	sort.Strings(s.Domains)

	if len(s.Domains) == 0 {
		return cfg.yamlError(fmt.Sprintf("$.dns[%d]", i), "at least one domain has to be specified")
	}

	if s.SSLInfo == nil {
		s.SSLInfo = &SSLInfo{}
	}

	s.SSLInfo.loadedCert = s.SSLInfo.Cert

	if s.SSLInfo.CertFile != "" {
		certBytes, err := os.ReadFile(filepath.Join(cfg.Dir, s.SSLInfo.CertFile))
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.dns[%d].ssl.cert_file", i), "cert file cannot be read")
		}

		s.SSLInfo.loadedCert = string(certBytes)
	}

	s.SSLInfo.loadedKey = s.SSLInfo.Key

	if s.SSLInfo.KeyFile != "" {
		keyBytes, err := os.ReadFile(filepath.Join(cfg.Dir, s.SSLInfo.KeyFile))
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.dns[%d].ssl.key_file", i), "private key file cannot be read")
		}

		s.SSLInfo.loadedKey = string(keyBytes)
	}

	if s.SSLInfo.loadedKey != "" && s.SSLInfo.loadedCert == "" {
		return cfg.yamlError(fmt.Sprintf("$.dns[%d].ssl", i), "private key defined but cert is missing")
	}

	if s.SSLInfo.loadedCert != "" && s.SSLInfo.loadedKey == "" {
		return cfg.yamlError(fmt.Sprintf("$.dns[%d].ssl", i), "cert defined but private key is missing")
	}

	return nil
}

func (s *DNS) Check(i int, cfg *Project) error {
	if s.Plugin != "" {
		s.dnsPlugin = cfg.FindLoadedPlugin(s.Plugin)
		if s.dnsPlugin == nil || s.dnsPlugin.HasAction(plugins.ActionDNS) {
			return cfg.yamlError(fmt.Sprintf("$.dns[%d].plugin", i), fmt.Sprintf("dns plugin '%s' not found", s.Plugin))
		}
	} else {
		s.dnsPlugin = cfg.FindLoadedPlugin(cfg.Defaults.DNS.Plugin)
	}

	return nil
}
