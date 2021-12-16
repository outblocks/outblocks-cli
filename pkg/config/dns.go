package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
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
	Domain  string   `json:"domain"`
	Domains []string `json:"domains"`
	Plugin  string   `json:"plugin,omitempty"`
	SSLInfo *SSLInfo `json:"ssl,omitempty"`

	domainsRegex []*regexp.Regexp

	plugin *plugins.Plugin
}

func (s *DNS) Match(d string) bool {
	for _, r := range s.domainsRegex {
		if r.MatchString(d) {
			return true
		}
	}

	return false
}

func (s *DNS) Normalize(i int, cfg *Project) error {
	s.Domains = append(s.Domains, s.Domain)
	for i, v := range s.Domains {
		s.Domains[i] = strings.ToLower(v)
	}

	if len(s.Domains) == 0 {
		return cfg.yamlError(fmt.Sprintf("$.dns[%d]", i), "at least one domain has to be specified")
	}

	s.domainsRegex = make([]*regexp.Regexp, len(s.Domains))

	for _, v := range s.Domains {
		s.domainsRegex[i] = plugin_util.DomainRegex(v)
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
		s.plugin = cfg.FindLoadedPlugin(s.Plugin)
	} else {
		for _, plug := range cfg.LoadedPlugins() {
			if plug.HasAction(plugins.ActionDNS) {
				s.plugin = plug

				break
			}
		}
	}

	return nil
}
