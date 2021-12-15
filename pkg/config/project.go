package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/pterm/pterm"
)

const (
	ProjectYAMLName = "project.outblocks"
	AppYAMLName     = "outblocks"
	LockfileName    = "outblocks.lock"
)

var (
	DefaultKnownTypes = map[string][]string{
		AppTypeFunction: {"functions"},
		AppTypeStatic:   {"statics"},
		AppTypeService:  {"services"},
	}
)

type DefaultsRun struct {
	Plugin string                 `json:"plugin,omitempty"`
	Env    map[string]string      `json:"env,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

type DefaultsDeploy struct {
	Plugin string                 `json:"plugin,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

type DefaultsDNS struct {
	Plugin string                 `json:"plugin,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

type Defaults struct {
	Run    DefaultsRun    `json:"run,omitempty"`
	Deploy DefaultsDeploy `json:"deploy,omitempty"`
	DNS    DefaultsDNS    `json:"dns,omitempty"`
}

type Project struct {
	Name         string                 `json:"name,omitempty"`
	Dir          string                 `json:"-"`
	State        *State                 `json:"state,omitempty"`
	Apps         []App                  `json:"-"`
	Dependencies map[string]*Dependency `json:"dependencies,omitempty"`
	Plugins      []*Plugin              `json:"plugins,omitempty"`
	DNS          []*DNS                 `json:"dns,omitempty"`
	Defaults     *Defaults              `json:"defaults,omitempty"`

	appsIDMap        map[string]App
	dependencyIDMap  map[string]*Dependency
	env              string
	loadedPlugins    []*plugins.Plugin
	loadedPluginsMap map[string]*plugins.Plugin
	yamlPath         string
	yamlData         []byte
	lock             *lockfile.Lockfile
	vars             map[string]interface{}
}

func (p *Project) ID() string {
	return fmt.Sprintf("%s_%s", p.Name, p.env)
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
	if plugin_util.FileExists(lockPath) {
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
		env:      opts.Env,
		Dir:      filepath.Dir(path),
		yamlData: data,
		lock:     lock,
		vars:     vars,
		State: &State{
			env: opts.Env,
		},
		Defaults: &Defaults{},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load project config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	return out, nil
}

func (p *Project) LoadApps() error {
	files := fileutil.FindYAMLFiles(p.Dir, AppYAMLName)

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

type fileType struct {
	Type string
}

func DetectAppType(data []byte) (string, error) {
	var f fileType
	if err := yaml.Unmarshal(data, &f); err != nil {
		return "", err
	}

	return f.Type, nil
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

	typ, err := DetectAppType(data)
	if err != nil {
		return err
	}

	if typ == "" {
		return fmt.Errorf("unknown application file found.\nfile: %s", file)
	}

	typ = KnownType(typ)
	if typ == "" {
		return fmt.Errorf("application type not supported: %s\nfile: %s", typ, file)
	}

	var app App

	switch typ {
	case AppTypeFunction:
		app, err = LoadFunctionAppData(file, data)

	case AppTypeService:
		app, err = LoadServiceAppData(file, data)

	case AppTypeStatic:
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
	if p.appsIDMap == nil {
		p.appsIDMap = make(map[string]App)
	}

	id := app.ID()
	if _, ok := p.appsIDMap[id]; ok {
		return false
	}

	p.appsIDMap[id] = app
	p.Apps = append(p.Apps, app)

	return true
}

func (p *Project) DependencyByID(n string) *Dependency {
	return p.dependencyIDMap[n]
}

func (p *Project) DependencyByName(n string) *Dependency {
	return p.dependencyIDMap[ComputeDependencyID(n)]
}

func (p *Project) AppByID(n string) App {
	return p.appsIDMap[n]
}

func (p *Project) SetLoadedPlugins(plugs []*plugins.Plugin) {
	p.loadedPlugins = plugs
	p.loadedPluginsMap = make(map[string]*plugins.Plugin)

	for _, plug := range plugs {
		p.loadedPluginsMap[plug.Name] = plug
	}
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

func (p *Project) FindDNSPlugin(u *url.URL) *plugins.Plugin {
	for _, dns := range p.DNS {
		if dns.Match(u.Host) {
			return dns.plugin
		}
	}

	return nil
}

func (p *Project) FindLoadedPlugin(name string) *plugins.Plugin {
	return p.loadedPluginsMap[name]
}

func (p *Project) LoadPlugins(ctx context.Context, log logger.Logger, loader *plugins.Loader, hostAddr string) error {
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
				prog.Stop()

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

		if err := plug.Prepare(ctx, log, p.env, p.ID(), p.Name, p.Dir, hostAddr, plugConfig.Other, prefix, p.YAMLData()); err != nil {
			return fmt.Errorf("error starting plugin '%s': %w", plug.Name, err)
		}
	}

	p.SetLoadedPlugins(plugs)

	return nil
}

func (p *Project) DomainInfoProto() []*apiv1.DomainInfo {
	var ret []*apiv1.DomainInfo

	for _, d := range p.DNS {
		if d.SSLInfo.loadedCert == "" {
			continue
		}

		ret = append(ret, &apiv1.DomainInfo{
			Domains: d.Domains,
			Cert:    d.SSLInfo.loadedCert,
			Key:     d.SSLInfo.loadedKey,
		})
	}

	return ret
}

func (p *Project) Env() string {
	return p.env
}
