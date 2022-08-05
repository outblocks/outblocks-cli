package config

import (
	"fmt"
	"path/filepath"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

func (p *Project) normalizeDNS() error {
	domains := make(map[string]struct{})

	for i, dns := range p.DNS {
		if err := dns.Normalize(i, p); err != nil {
			return err
		}

		for _, d := range dns.Domains {
			if _, ok := domains[d]; ok {
				return p.yamlError(fmt.Sprintf("$.dns[%d]", i), "domain '%s' is duplicated")
			}

			domains[d] = struct{}{}
		}
	}

	return nil
}

// Initial first pass validation.
func (p *Project) Normalize() error {
	if p.Name == "" {
		p.Name = filepath.Base(p.Dir)
	}

	p.dependencyIDMap = make(map[string]*Dependency, len(p.Dependencies))

	err := func() error {
		for key, dep := range p.Dependencies {
			dep.cfg = p

			dep.Name = key
			if err := dep.Normalize(key, p); err != nil {
				return err
			}

			p.dependencyIDMap[dep.ID()] = dep
		}

		for i, plugin := range p.Plugins {
			if err := plugin.Normalize(i, p); err != nil {
				return err
			}
		}

		if err := p.normalizeDNS(); err != nil {
			return err
		}

		// Default to local statefile.
		if p.State == nil {
			p.State = &State{}
		}

		if err := p.State.Normalize(p); err != nil {
			return err
		}

		if p.Secrets == nil {
			p.Secrets = &Secrets{}
		}

		if err := p.Secrets.Normalize(p); err != nil {
			return err
		}

		if p.Monitoring == nil {
			p.Monitoring = &Monitoring{}
		}

		if err := p.Monitoring.Normalize(p); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		return merry.Errorf("project config validation failed.\nfile: %s\n%s", p.yamlPath, err)
	}

	// URL uniqueness check.
	urlMap := make(map[string]App)

	err = func() error {
		for _, app := range p.Apps {
			if err := app.Normalize(p); err != nil {
				return err
			}

			if app.URL() != nil {
				url := app.URL().String()
				if cur, ok := urlMap[url]; ok {
					return merry.Errorf("same URL '%s' used in more than 1 app: '%s' and '%s'", url, app.Name(), cur.Name())
				}

				urlMap[url] = app
			}
		}

		return nil
	}()

	return err
}

// Logic validation after everything is loaded, e.g. check for supported types.
func (p *Project) FullCheck() error {
	err := func() error {
		for key, dep := range p.Dependencies {
			if err := dep.Check(key, p); err != nil {
				return err
			}
		}

		for i, dns := range p.DNS {
			if err := dns.Check(i, p); err != nil {
				return err
			}
		}

		if err := p.State.Check(p); err != nil {
			return err
		}

		if err := p.Secrets.Check(p); err != nil {
			return err
		}

		if err := p.Monitoring.Check(p); err != nil {
			return err
		}

		for _, plug := range p.LoadedPlugins() {
			if plug.HasAction(plugins.ActionDeploy) && p.Defaults.Deploy.Plugin == "" {
				p.Defaults.Deploy.Plugin = plug.Name
			}

			if plug.HasAction(plugins.ActionDNS) && p.Defaults.DNS.Plugin == "" {
				p.Defaults.DNS.Plugin = plug.Name
			}

			if plug.HasAction(plugins.ActionRun) && p.Defaults.Run.Plugin == "" {
				p.Defaults.Run.Plugin = plug.Name
			}
		}

		return nil
	}()

	if err != nil {
		return merry.Errorf("project config check failed.\nfile: %s\n%s", p.yamlPath, err)
	}

	err = func() error {
		for _, app := range p.Apps {
			if err := app.Check(p); err != nil {
				return err
			}
		}

		return nil
	}()

	return err
}

func (p *Project) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, p.yamlData)
}
