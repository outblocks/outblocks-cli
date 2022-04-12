package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/23doors/go-yaml/parser"
	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/pterm/pterm"
)

const (
	ProjectYAMLName = "project.outblocks"
	LockfileName    = "outblocks.lock"
	AppYAMLName     = "app.outblocks"
)

var (
	AppYAMLNames      = []string{AppYAMLName, "outblocks"}
	DefaultKnownTypes = map[string][]string{
		AppTypeFunction: {"functions"},
		AppTypeStatic:   {"statics"},
		AppTypeService:  {"services"},
	}

	essentialProjectKeys    = []string{"name", "dependencies", "plugins", "state"}
	essentialProjectKeysMap = util.StringArrayToSet(essentialProjectKeys)
	essentialAppKeys        = []string{"name", "type", "dir", "url", "deploy.plugin", "run.plugin", "dns.plugin", "defaults.deploy.plugin", "defaults.run.plugin", "defaults.dns.plugin"}
	essentialAppKeysMap     = util.StringArrayToSet(essentialAppKeys)
)

type LoadMode int

const (
	LoadModeFull LoadMode = iota
	LoadModeEssential
	LoadModeSkip
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
	vals             map[string]interface{}
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

func LoadProjectConfig(cfgPath string, vals map[string]interface{}, mode LoadMode, opts *ProjectOptions) (*Project, error) {
	if mode == LoadModeSkip {
		return nil, nil
	}

	if cfgPath == "" {
		return nil, ErrProjectConfigNotFound
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, merry.Errorf("cannot read project yaml: %w", err)
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

	var reqKeys map[string]bool

	if mode == LoadModeEssential {
		reqKeys = essentialProjectKeysMap
	}

	p, err := LoadProjectConfigData(cfgPath, data, vals, reqKeys, opts, lock)
	if err != nil {
		return nil, err
	}

	return p, err
}

func LoadProjectConfigData(path string, data []byte, vals map[string]interface{}, essentialKeys map[string]bool, opts *ProjectOptions, lock *lockfile.Lockfile) (*Project, error) {
	f, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, merry.Errorf("cannot read project yaml file: %s, cause: %w", path, err)
	}

	if len(f.Docs) != 1 {
		return nil, merry.Errorf("multi-document yamls are unsupported, file: %s", path)
	}

	n, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, merry.Errorf("project file %s yaml is invalid", path)
	}

	_, err = traverseYAMLMapping(n, path, opts.Env, vals, essentialKeys, nil)
	if err != nil {
		return nil, err
	}

	out := &Project{
		yamlPath: path,
		yamlData: data,
		env:      opts.Env,
		Dir:      filepath.Dir(path),
		lock:     lock,
		vals:     vals,
		State: &State{
			env: opts.Env,
		},
		Defaults: &Defaults{},
	}

	if err := util.YAMLNodeDecode(n, out); err != nil {
		return nil, merry.Errorf("load project config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	return out, nil
}

func (p *Project) LoadApps(mode LoadMode) error {
	if mode == LoadModeSkip {
		return nil
	}

	files := fileutil.FindYAMLFiles(p.Dir, AppYAMLNames...)

	if err := p.LoadAppFiles(files, mode); err != nil {
		return err
	}

	return nil
}

func (p *Project) LoadAppFiles(files []string, mode LoadMode) error {
	var reqKeys map[string]bool

	if mode == LoadModeEssential {
		reqKeys = essentialAppKeysMap
	}

	for _, f := range files {
		if err := p.LoadAppFile(f, reqKeys); err != nil {
			return err
		}
	}

	return nil
}

type fileType struct {
	Type string
}

func DetectAppType(n ast.Node) (string, error) {
	var f fileType

	if err := yaml.NodeToValue(n, &f); err != nil {
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

func (p *Project) LoadAppFile(file string, essentialKeys map[string]bool) error {
	f, err := parser.ParseFile(file, 0)
	if err != nil {
		return merry.Errorf("cannot read application yaml file: %s, cause: %w", file, err)
	}

	if len(f.Docs) != 1 {
		return merry.Errorf("multi-document yamls are unsupported, file: %s", file)
	}

	n, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return merry.Errorf("application file %s yaml is invalid", file)
	}

	_, err = traverseYAMLMapping(n, file, p.env, p.vals, essentialKeys, nil)
	if err != nil {
		return err
	}

	typ, err := DetectAppType(n)
	if err != nil {
		return err
	}

	if typ == "" {
		return merry.Errorf("unknown application file found.\nfile: %s", file)
	}

	typ = KnownType(typ)
	if typ == "" {
		return merry.Errorf("application type not supported: %s\nfile: %s", typ, file)
	}

	var app App

	switch typ {
	case AppTypeFunction:
		app, err = LoadFunctionAppData(p.Name, file, n)

	case AppTypeService:
		app, err = LoadServiceAppData(p.Name, file, n)

	case AppTypeStatic:
		app, err = LoadStaticAppData(p.Name, file, n)
	}

	if err != nil {
		return err
	}

	if !p.RegisterApp(app) {
		return merry.Errorf("application with name: '%s' of type: '%s' found more than once\nfile: %s", typ, app.Name(), file)
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

func (p *Project) YAMLPath() string {
	return p.yamlPath
}

func (p *Project) FindDNSPlugin(u *url.URL) *plugins.Plugin {
	for _, dns := range p.DNS {
		if dns.Match(u.Host) {
			dns.MarkAsUsed()

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

				return merry.Errorf("unable to load '%s' plugin: %w", plug.Name, err)
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
			return merry.Errorf("error starting plugin '%s': %w", plug.Name, err)
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
