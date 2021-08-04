package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

const (
	ProjectYAMLName = "project.outblocks"
	AppYAMLName     = "outblocks"
	LockfileName    = "outblocks.lock"
)

var (
	DefaultKnownTypes = map[string][]string{
		TypeFunction: {"functions"},
		TypeStatic:   {"statics"},
		TypeService:  {"services"},
	}
)

type Project struct {
	Name         string                 `json:"name,omitempty"`
	State        *State                 `json:"state,omitempty"`
	Dependencies map[string]*Dependency `json:"dependencies,omitempty"`
	Plugins      []*Plugin              `json:"plugins,omitempty"`
	DNS          []*DNS                 `json:"dns,omitempty"`

	Apps   []App          `json:"-"`
	AppMap map[string]App `json:"-"`
	Path   string         `json:"-"`

	loadedPlugins []*plugins.Plugin
	yamlPath      string
	yamlData      []byte
	lock          *lockfile.Lockfile
	vars          map[string]interface{}
}

func (p *Project) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.State, validation.Required),
		validation.Field(&p.Dependencies),
	)
}

type ProjectOptions struct {
	Env string
}

func LoadProjectConfig(cfgPath string, vars map[string]interface{}, opts *ProjectOptions) (*Project, error) {
	if cfgPath == "" {
		return nil, ErrProjectConfigNotFound
	}

	data, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read project yaml: %w", err)
	}

	// Process lockfile.
	var lock *lockfile.Lockfile

	lockPath := filepath.Join(filepath.Dir(cfgPath), LockfileName)
	if fileutil.FileExists(lockPath) {
		lock, err = lockfile.LoadLockfile(lockPath)
		if err != nil {
			return nil, err
		}
	}

	p, err := LoadProjectConfigData(cfgPath, data, vars, opts, lock)
	if err != nil {
		return nil, err
	}

	return p, err
}

func LoadProjectConfigData(path string, data []byte, vars map[string]interface{}, opts *ProjectOptions, lock *lockfile.Lockfile) (*Project, error) {
	data, err := NewYAMLEvaluator(vars).Expand(data)
	if err != nil {
		return nil, fmt.Errorf("load project config %s error: \n%w", path, err)
	}

	out := &Project{
		yamlPath: path,
		Path:     filepath.Dir(path),
		yamlData: data,
		lock:     lock,
		vars:     vars,
		State: &State{
			Env: opts.Env,
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load project config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	return out, nil
}

func (p *Project) LoadApps() error {
	files := fileutil.FindYAMLFiles(p.Path, AppYAMLName)

	if err := p.LoadFiles(files); err != nil {
		return err
	}

	return nil
}

func (p *Project) LoadFiles(files []string) error {
	for _, f := range files {
		if err := p.LoadFile(f); err != nil {
			return err
		}
	}

	return nil
}

func DetectAppType(file string) string {
	f, err := os.Stat(file)
	if err == nil && f.IsDir() {
		return filepath.Base(filepath.Dir(file))
	}

	return filepath.Base(filepath.Dir(filepath.Dir(file)))
}

type fileType struct {
	Type string
}

func KnownType(typ string) string {
	typ = strings.TrimSpace(strings.ToLower(typ))

	if _, ok := DefaultKnownTypes[typ]; ok {
		return typ
	}

	// check aliases
	for k, v := range DefaultKnownTypes {
		for _, alias := range v {
			if alias == typ {
				return k
			}
		}
	}

	return ""
}

func (p *Project) LoadFile(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read yaml: %w", err)
	}

	data, err = NewYAMLEvaluator(p.vars).Expand(data)
	if err != nil {
		return fmt.Errorf("load application file %s error: \n%w", file, err)
	}

	typ := DetectAppType(file)
	if typ == "" {
		var f fileType
		if err := yaml.Unmarshal(data, &f); err != nil {
			return err
		}

		if f.Type == "" {
			return fmt.Errorf("unknown application file found.\nfile: %s", file)
		}
	}

	typ = KnownType(typ)
	if typ == "" {
		return fmt.Errorf("application type not supported: %s\nfile: %s", typ, file)
	}

	var app App

	switch typ {
	case TypeFunction:
		app, err = LoadFunctionAppData(file, data)

	case TypeService:
		app, err = LoadServiceAppData(file, data)

	case TypeStatic:
		app, err = LoadStaticAppData(file, data)
	}

	if err != nil {
		return err
	}

	if !p.RegisterApp(app) {
		return fmt.Errorf("application with name: '%s' of type: '%s' found more than once\nfile: %s", typ, app.Name(), file)
	}

	return nil
}

func (p *Project) RegisterApp(app App) bool {
	if p.AppMap == nil {
		p.AppMap = make(map[string]App)
	}

	id := app.ID()
	if _, ok := p.AppMap[id]; ok {
		return false
	}

	p.AppMap[id] = app
	p.Apps = append(p.Apps, app)

	return true
}

func (p *Project) FindDependency(n string) *Dependency {
	for name, dep := range p.Dependencies {
		if name == n {
			return dep
		}
	}

	return nil
}

func (p *Project) SetLoadedPlugins(plugs []*plugins.Plugin) {
	p.loadedPlugins = plugs
}

func (p *Project) LoadedPlugins() []*plugins.Plugin {
	return p.loadedPlugins
}

func (p *Project) PluginLock(plug *Plugin) *lockfile.Plugin {
	return p.lock.PluginByName(plug.Name)
}

func (p *Project) YAMLData() []byte {
	return p.yamlData
}

func (p *Project) FindDNSPlugin(url string) *plugins.Plugin {
	host := strings.SplitN(url, "/", 2)[0]

	for _, dns := range p.DNS {
		if strings.HasSuffix(host, dns.Domain) {
			return dns.plugin
		}
	}

	return nil
}

func (p *Project) FindLoadedPlugin(name string) *plugins.Plugin {
	for _, plug := range p.loadedPlugins {
		if plug.Name == name {
			return plug
		}
	}

	return nil
}

func (p *Project) LoadPlugins(ctx context.Context, log logger.Logger, loader *plugins.Loader) error {
	plugs := make([]*plugins.Plugin, len(p.Plugins))
	pluginsToDownload := make(map[int]*Plugin)

	for i, plug := range p.Plugins {
		plugin, err := loader.LoadPlugin(ctx, plug.Name, plug.Source, plug.VerConstr(), p.PluginLock(plug))
		if err != nil {
			if err != plugins.ErrPluginNotFound {
				return err
			}

			pluginsToDownload[i] = plug

			continue
		}

		plugs[i] = plugin

		plug.SetLoaded(plugin)
	}

	if len(pluginsToDownload) != 0 {
		prog, _ := log.ProgressBar().WithTotal(len(pluginsToDownload)).WithTitle("Downloading plugins...").Start()

		for i, plug := range pluginsToDownload {
			title := fmt.Sprintf("Downloading plugin '%s'", plug.Name)
			if plug.Version != "" {
				title += fmt.Sprintf(" with version: %s", plug.Version)
			}

			prog.UpdateTitle(title)

			plugin, err := loader.DownloadPlugin(ctx, plug.Name, plug.VerConstr(), plug.Source, p.PluginLock(plug))
			plugs[i] = plugin

			plug.SetLoaded(plugin)

			if err != nil {
				_, _ = prog.Stop()

				return fmt.Errorf("unable to load '%s' plugin: %w", plug.Name, err)
			}

			prog.Increment()
			pterm.Success.Printf("Downloaded plugin '%s' at version: %s\n", plug.Name, plugin.Version)
		}
	}

	// Normalize and start plugins.
	for _, plug := range plugs {
		if err := plug.Normalize(); err != nil {
			return err
		}
	}

	for i, plug := range plugs {
		plug := plug
		plugConfig := p.Plugins[i]
		prefix := fmt.Sprintf("$.plugins[%d]", i)

		if err := plug.Prepare(ctx, log, p.Name, p.Path, plugConfig.Other, prefix, p.YAMLData()); err != nil {
			return fmt.Errorf("error starting plugin '%s': %w", plug.Name, err)
		}
	}

	p.loadedPlugins = plugs

	return nil
}
