package config

import (
	"fmt"
	"path/filepath"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

// Initial first pass validation.
func (p *ProjectConfig) Normalize() error {
	if p.Name == "" {
		p.Name = filepath.Base(p.Path)
	}

	err := func() error {
		for key, dep := range p.Dependencies {
			dep.Name = key
			if err := dep.Normalize(key, p); err != nil {
				return err
			}
		}

		for i, plugin := range p.Plugins {
			if err := plugin.Normalize(i, p); err != nil {
				return err
			}
		}

		for i, dns := range p.DNS {
			if err := dns.Normalize(i, p); err != nil {
				return err
			}
		}

		// Default to local statefile.
		if p.State == nil {
			p.State = &State{
				Type: StateLocal,
			}
		} else if p.State.Type == "" {
			p.State.Type = StateLocal
		}

		if err := p.State.Normalize(p); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config validation failed.\nfile: %s\n%s", p.yamlPath, err)
	}

	err = func() error {
		for _, app := range p.Apps {
			if err := app.Normalize(p); err != nil {
				return err
			}
		}

		return nil
	}()

	return err
}

// Logic validation after everything is loaded, e.g. check for supported types.
func (p *ProjectConfig) FullCheck() error {
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

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config check failed.\nfile: %s\n%s", p.yamlPath, err)
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

func (p *ProjectConfig) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, p.yamlData)
}
