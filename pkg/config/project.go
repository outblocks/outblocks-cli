package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

// TODO: add state support

const (
	ProjectYAMLName = "project.outblocks"
	AppYAMLName     = "outblocks"
	LockfileName    = "outblocks.lock"
)

var (
	DefaultKnownTypes = map[string][]string{
		"function": {"functions"},
		"static":   {"statics"},
		"service":  {"services"},
	}
)

type ProjectConfig struct {
	State        *ProjectState                 `json:"state,omitempty"`
	Dependencies map[string]*ProjectDependency `json:"dependencies,omitempty"`
	Plugins      []*ProjectPlugin              `json:"plugins,omitempty"`

	functions []*FunctionConfig
	services  []*ServiceConfig
	static    []*StaticConfig

	plugins []*plugins.Plugin

	Path    string `json:"-"`
	BaseDir string `json:"-"`
	data    []byte
	lock    *lockfile.Lockfile
}

type ProjectDependency struct {
	Type   string                 `json:"type"`
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`

	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
}

func LoadProjectConfig(vars map[string]interface{}) (*ProjectConfig, error) {
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

	p, err := LoadProjectConfigData(f, data, lock)
	if err != nil {
		return nil, err
	}

	return p, err
}

func LoadProjectConfigData(path string, data []byte, lock *lockfile.Lockfile) (*ProjectConfig, error) {
	out := &ProjectConfig{
		Path:    path,
		BaseDir: filepath.Dir(path),
		data:    data,
		lock:    lock,
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("load project config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	return out, nil
}

func (p *ProjectConfig) LoadApps() error {
	base := filepath.Dir(p.Path)
	files := fileutil.FindYAMLFiles(base, AppYAMLName)

	if err := p.LoadFiles(files); err != nil {
		return err
	}

	return nil
}

func (p *ProjectConfig) LoadFiles(files []string) error {
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

func (p *ProjectConfig) KnownType(typ string) string {
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

func (p *ProjectConfig) LoadFile(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read yaml: %w", err)
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

	typ = p.KnownType(typ)
	if typ == "" {
		return fmt.Errorf("application type not supported: %s\nfile: %s", typ, file)
	}

	switch typ {
	case "function":
		f, err := LoadFunctionConfigData(file, data)
		if err != nil {
			return err
		}

		p.functions = append(p.functions, f)

	case "service":
		f, err := LoadServiceConfigData(file, data)
		if err != nil {
			return err
		}

		p.services = append(p.services, f)

	case "static":
		f, err := LoadStaticConfigData(file, data)
		if err != nil {
			return err
		}

		p.static = append(p.static, f)
	}

	return nil
}
