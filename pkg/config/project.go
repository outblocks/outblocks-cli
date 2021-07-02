package config

import (
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
	"github.com/outblocks/outblocks-cli/pkg/plugins"
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

	plugins  []*plugins.Plugin
	yamlPath string
	yamlData []byte
	lock     *lockfile.Lockfile
	vars     map[string]interface{}
}

func (p *Project) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.State, validation.Required),
		validation.Field(&p.Dependencies),
	)
}

func LoadProjectConfig(vars map[string]interface{}) (*Project, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot find current directory: %w", err)
	}

	f := fileutil.FindYAMLGoingUp(pwd, ProjectYAMLName)
	if f == "" {
		return nil, ErrProjectConfigNotFound
	}

	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read project yaml: %w", err)
	}

	// Process lockfile.
	var lock *lockfile.Lockfile

	lockPath := filepath.Join(filepath.Dir(f), LockfileName)
	if fileutil.FileExists(lockPath) {
		lock, err = lockfile.LoadLockfile(lockPath)
		if err != nil {
			return nil, err
		}
	}

	p, err := LoadProjectConfigData(f, data, vars, lock)
	if err != nil {
		return nil, err
	}

	return p, err
}

func LoadProjectConfigData(path string, data []byte, vars map[string]interface{}, lock *lockfile.Lockfile) (*Project, error) {
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

func deduceType(file string) string {
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

	typ := deduceType(file)
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
	case "function":
		app, err = LoadFunctionAppData(file, data)

	case "service":
		app, err = LoadServiceAppData(file, data)

	case "static":
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

func (p *Project) SetPlugins(plugs []*plugins.Plugin) {
	p.plugins = plugs
}

func (p *Project) LoadedPlugins() []*plugins.Plugin {
	return p.plugins
}

func (p *Project) PluginLock(plug *Plugin) *lockfile.Plugin {
	return p.lock.PluginByName(plug.Name)
}

func (p *Project) YAMLData() []byte {
	return p.yamlData
}

func (p *Project) FindDNSPlugin(url string) *plugins.Plugin {
	url = strings.SplitN(url, "/", 2)[0]

	for _, dns := range p.DNS {
		if strings.HasSuffix(url, dns.Domain) {
			return dns.plugin
		}
	}

	return nil
}

func (p *Project) FindLoadedPlugin(name string) *plugins.Plugin {
	for _, plug := range p.plugins {
		if plug.Name == name {
			return plug
		}
	}

	return nil
}
