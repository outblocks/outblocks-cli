package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

var (
	ValidURLRegex   = regexp.MustCompile(`^(https?://)?([a-zA-Z][a-zA-Z0-9-]*)((\.)([a-zA-Z][a-zA-Z0-9-]*)){1,}(/[a-zA-Z0-9-_]+)*(/)?$`)
	ValidNameRegex  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,30}$`)
	ValidAppTypes   = []string{AppTypeStatic, AppTypeFunction, AppTypeService}
	RunPluginDirect = "direct"
)

type App interface {
	ID() string
	Name() string
	Dir() string
	URL() *url.URL
	Env() map[string]string
	PathRedirect() string
	Normalize(cfg *Project) error
	Check(cfg *Project) error
	Type() string
	Proto() *apiv1.App
	BuildProto() *apiv1.AppBuild
	RunInfo() *AppRunInfo
	DeployInfo() *AppDeployInfo
	SupportsLocal() bool
	YAMLError(path, msg string) error

	DeployPlugin() *plugins.Plugin
	RunPlugin() *plugins.Plugin
}

type AppRunInfo struct {
	Plugin  string                 `json:"plugin,omitempty"`
	Command *command.StringCommand `json:"command,omitempty"`
	Port    int                    `json:"port,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Other   map[string]interface{} `yaml:",remain" json:"other,omitempty"`
}

type AppDeployInfo struct {
	Plugin string                 `json:"plugin,omitempty"`
	Env    map[string]string      `json:"env,omitempty"`
	Other  map[string]interface{} `yaml:",remain"`
}

func (i *AppDeployInfo) Proto() *apiv1.AppDeployInfo {
	return &apiv1.AppDeployInfo{
		Plugin:     i.Plugin,
		Env:        i.Env,
		Properties: plugin_util.MustNewStruct(i.Other),
	}
}

type AppNeed struct {
	Dep   string                 `yaml:"dependency,omitempty"`
	Other map[string]interface{} `yaml:"-,remain"`

	dep *Dependency
}

func (n *AppNeed) Normalize(name string, cfg *Project, data []byte) error {
	if n.Dep != "" {
		name = n.Dep
	}

	dep := cfg.DependencyByName(name)
	if dep == nil {
		return fileutil.YAMLError(fmt.Sprintf("$.needs.%s", name), fmt.Sprintf("'%s' not found in project dependencies", name), data)
	}

	n.dep = dep

	return nil
}

func (n *AppNeed) Dependency() *Dependency {
	return n.dep
}

func (n *AppNeed) Proto() *apiv1.AppNeed {
	return &apiv1.AppNeed{
		Dependency: n.dep.Name,
		Properties: plugin_util.MustNewStruct(n.Other),
	}
}

type BasicApp struct {
	AppName         string                 `json:"name"`
	AppType         string                 `json:"type"`
	AppURL          string                 `json:"url"`
	AppPathRedirect string                 `json:"path_redirect"`
	AppEnv          map[string]string      `json:"env"`
	AppDir          string                 `json:"dir"`
	AppRun          *AppRunInfo            `json:"run"`
	AppDeploy       *AppDeployInfo         `json:"deploy"`
	Needs           map[string]*AppNeed    `json:"needs"`
	Other           map[string]interface{} `yaml:"-,remain"`

	url          *url.URL
	yamlPath     string
	yamlData     []byte
	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
}

func NewBasicApp() *BasicApp {
	return &BasicApp{
		AppRun:    &AppRunInfo{},
		AppDeploy: &AppDeployInfo{},
	}
}

func (a *BasicApp) Validate() error {
	return validation.ValidateStruct(a,
		validation.Field(&a.AppName, validation.Required, validation.Match(ValidNameRegex)),
		validation.Field(&a.AppURL, validation.Match(ValidURLRegex)),
	)
}

func ParseURL(u string, normalize bool) (*url.URL, error) {
	if u == "" {
		return nil, nil
	}

	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	if normalize {
		if parsed.Path == "" {
			parsed.Path = "/"
		}
	}

	return parsed, nil
}

func (a *BasicApp) Normalize(cfg *Project) error {
	var err error

	if a.AppRun == nil {
		a.AppRun = &AppRunInfo{}
	}

	if a.AppEnv == nil {
		a.AppEnv = make(map[string]string)
	}

	if a.AppDir == "" {
		a.AppDir = filepath.Dir(a.yamlPath)
	} else {
		a.AppDir, err = filepath.Abs(a.AppDir)
		if err != nil {
			return a.YAMLError("$.dir", "dir is invalid")
		}
	}

	if !fileutil.IsRelativeSubdir(cfg.Dir, a.AppDir) {
		return a.YAMLError("$.dir", "main config dir must be a parent of App.Dir")
	}

	a.AppDir, _ = filepath.Rel(cfg.Dir, a.AppDir)
	a.AppDir = "./" + a.AppDir

	if a.AppDeploy == nil {
		a.AppDeploy = &AppDeployInfo{}
	}

	if a.AppPathRedirect == "" {
		a.AppPathRedirect = "/"
	}

	a.AppDeploy.Plugin = strings.ToLower(a.AppDeploy.Plugin)
	a.AppRun.Plugin = strings.ToLower(a.AppRun.Plugin)
	a.AppURL = strings.ToLower(a.AppURL)

	a.url, err = ParseURL(a.AppURL, false)
	if err != nil {
		return a.YAMLError("$.url", "url is invalid")
	}

	err = func() error {
		for name, n := range a.Needs {
			if n == nil {
				a.Needs[name] = &AppNeed{}
			}
		}

		for name, n := range a.Needs {
			if err := n.Normalize(name, cfg, a.yamlData); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return merry.Errorf("%s config validation failed.\nfile: %s\n%s", a.Type(), a.yamlPath, err)
	}

	return nil
}

func (a *BasicApp) Check(cfg *Project) error {
	// Check deploy plugin.
	deployPlugin := a.AppDeploy.Plugin
	if deployPlugin == "" {
		deployPlugin = cfg.Defaults.Deploy.Plugin
	}

	plug := cfg.FindLoadedPlugin(deployPlugin)

	if plug != nil && (!plug.HasAction(plugins.ActionDeploy) || !plug.SupportsApp(a.Type())) {
		for _, p := range cfg.LoadedPlugins() {
			if !p.HasAction(plugins.ActionDeploy) || !p.SupportsApp(a.Type()) {
				continue
			}

			plug = p

			break
		}
	}

	if plug == nil {
		return merry.Errorf("%s has no matching deployment plugin available.\nfile: %s", a.Type(), a.yamlPath)
	}

	a.deployPlugin = plug
	a.AppDeploy.Plugin = plug.Name

	// Check run plugin.
	runPlugin := a.AppRun.Plugin
	if runPlugin == "" {
		runPlugin = cfg.Defaults.Run.Plugin
	}

	plug = cfg.FindLoadedPlugin(runPlugin)

	if plug != nil && (!plug.HasAction(plugins.ActionDeploy) || !plug.SupportsApp(a.Type())) {
		for _, p := range cfg.LoadedPlugins() {
			if !p.HasAction(plugins.ActionDeploy) || !p.SupportsApp(a.Type()) {
				continue
			}

			plug = p

			break
		}
	}

	if plug == nil && !strings.EqualFold(RunPluginDirect, runPlugin) {
		return merry.Errorf("%s has no matching run plugin available.\nfile: %s", a.Type(), a.yamlPath)
	}

	a.runPlugin = plug
	a.AppRun.Plugin = runPlugin

	for k, need := range a.Needs {
		if need.dep.deployPlugin != a.deployPlugin {
			return a.YAMLError(fmt.Sprintf("$.needs[%s]", k), fmt.Sprintf("%s needs a dependency that uses different deployment plugin", a.Type()))
		}
	}

	return nil
}

func (a *BasicApp) YAMLError(path, msg string) error {
	return merry.Errorf("file: %s\n%s", a.yamlPath, fileutil.YAMLError(path, msg, a.yamlData))
}

func (a *BasicApp) Type() string {
	return a.AppType
}

func (a *BasicApp) Dir() string {
	return a.AppDir
}

func (a *BasicApp) Proto() *apiv1.App {
	needs := make(map[string]*apiv1.AppNeed, len(a.Needs))

	for k, n := range a.Needs {
		needs[k] = n.Proto()
	}

	var appURL string
	if a.url != nil {
		appURL = a.url.String()
	}

	var deployPluginName, runPluginName string

	if a.DeployPlugin() != nil {
		deployPluginName = a.DeployPlugin().Name
	}

	if a.RunPlugin() != nil {
		runPluginName = a.RunPlugin().Name
	}

	return &apiv1.App{
		Id:           a.ID(),
		Name:         a.AppName,
		Type:         a.Type(),
		Dir:          a.Dir(),
		Url:          appURL,
		PathRedirect: a.AppPathRedirect,
		Env:          a.Env(),
		DeployPlugin: deployPluginName,
		RunPlugin:    runPluginName,
		Deploy:       a.AppDeploy.Proto(),
		Needs:        needs,
		Properties:   plugin_util.MustNewStruct(a.Other),
	}
}

func (a *BasicApp) DeployPlugin() *plugins.Plugin {
	return a.deployPlugin
}

func (a *BasicApp) RunPlugin() *plugins.Plugin {
	return a.runPlugin
}

func (a *BasicApp) Name() string {
	return a.AppName
}

func (a *BasicApp) URL() *url.URL {
	return a.url
}

func (a *BasicApp) ID() string {
	return ComputeAppID(a.AppType, a.AppName)
}

func ComputeAppID(typ, name string) string {
	return fmt.Sprintf("app_%s_%s", typ, name)
}

func (a *BasicApp) SupportsLocal() bool {
	return false
}

func (a *BasicApp) RunInfo() *AppRunInfo {
	return a.AppRun
}

func (a *BasicApp) DeployInfo() *AppDeployInfo {
	return a.AppDeploy
}

func (a *BasicApp) PathRedirect() string {
	return a.AppPathRedirect
}

func (a *BasicApp) Env() map[string]string {
	return a.AppEnv
}
