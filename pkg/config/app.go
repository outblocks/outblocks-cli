package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
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
	RunInfo() *AppRunInfo
	DeployInfo() *AppDeployInfo
	SupportsLocal() bool
	YAMLError(path, msg string) error

	DeployPlugin() *plugins.Plugin
	RunPlugin() *plugins.Plugin
	DNSPlugin() *plugins.Plugin
}

type AppRunInfo struct {
	Plugin  string                 `json:"plugin,omitempty"`
	Command string                 `json:"command,omitempty"`
	Port    int32                  `json:"port,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Other   map[string]interface{} `yaml:",remain"`
}

func (i *AppRunInfo) Proto() *apiv1.AppRunInfo {
	return &apiv1.AppRunInfo{
		Plugin:  i.Plugin,
		Command: i.Command,
		Port:    i.Port,
		Env:     i.Env,
		Other:   plugin_util.MustNewStruct(i.Other),
	}
}

type AppDeployInfo struct {
	Plugin string                 `json:"plugin,omitempty"`
	Env    map[string]string      `json:"env,omitempty"`
	Other  map[string]interface{} `yaml:",remain"`
}

func (i *AppDeployInfo) Proto() *apiv1.AppDeployInfo {
	return &apiv1.AppDeployInfo{
		Plugin: i.Plugin,
		Env:    i.Env,
		Other:  plugin_util.MustNewStruct(i.Other),
	}
}

type AppNeed struct {
	Other map[string]interface{} `yaml:"-,remain"`

	dep *Dependency
}

func (n *AppNeed) Normalize(name string, cfg *Project, data []byte) error {
	dep := cfg.DependencyByName(name)
	if dep == nil {
		return fileutil.YAMLError(fmt.Sprintf("$.needs.%s", name), "object not found in project dependencies", data)
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
	dnsPlugin    *plugins.Plugin
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

func (a *BasicApp) Normalize(cfg *Project) error {
	var err error

	if a.AppRun == nil {
		a.AppRun = &AppRunInfo{}
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

	if a.AppURL != "" {
		if !strings.HasPrefix(a.AppURL, "http") {
			a.AppURL = "https://" + a.AppURL
		}

		var err error

		a.url, err = url.Parse(a.AppURL)
		if err != nil {
			return a.YAMLError("$.url", "url is invalid")
		}

		if a.url.Path == "" {
			a.url.Path = "/"
		}
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
		return fmt.Errorf("%s config validation failed.\nfile: %s\n%s", a.Type(), a.yamlPath, err)
	}

	return nil
}

func (a *BasicApp) Check(cfg *Project) error {
	// Check deploy plugin.
	deployPlugin := a.AppDeploy.Plugin
	if deployPlugin == "" {
		deployPlugin = cfg.Defaults.Deploy.Plugin
	}

	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionDeploy) {
			continue
		}

		if (deployPlugin != "" && deployPlugin != plug.Name) || !plug.SupportsApp(a.Type()) {
			continue
		}

		a.deployPlugin = plug
		a.AppDeploy.Plugin = plug.Name

		break
	}

	if a.deployPlugin == nil {
		return fmt.Errorf("%s has no matching deployment plugin available.\nfile: %s", a.Type(), a.yamlPath)
	}

	// Check run plugin.
	runPlugin := a.AppRun.Plugin
	if runPlugin == "" {
		runPlugin = cfg.Defaults.Run.Plugin
	}

	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionRun) {
			continue
		}

		if (runPlugin != "" && runPlugin != plug.Name) || !plug.SupportsApp(a.Type()) {
			continue
		}

		a.runPlugin = plug
		a.AppRun.Plugin = plug.Name
	}

	if a.runPlugin == nil && !strings.EqualFold(RunPluginDirect, runPlugin) {
		return fmt.Errorf("%s has no matching run plugin available.\nfile: %s", a.Type(), a.yamlPath)
	}

	// Check dns plugin.
	if a.AppURL != "" {
		a.dnsPlugin = cfg.FindDNSPlugin(a.URL())
	}

	for k, need := range a.Needs {
		if need.dep.deployPlugin != a.deployPlugin {
			return a.YAMLError(fmt.Sprintf("$.needs[%s]", k), fmt.Sprintf("%s needs a dependency that uses different deployment plugin.", a.Type()))
		}
	}

	return nil
}

func (a *BasicApp) YAMLError(path, msg string) error {
	return fmt.Errorf("file: %s\n%s", a.yamlPath, fileutil.YAMLError(path, msg, a.yamlData))
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

	var dnsPluginName, deployPluginName, runPluginName string

	if a.DeployPlugin() != nil {
		deployPluginName = a.DeployPlugin().Name
	}

	if a.DNSPlugin() != nil {
		dnsPluginName = a.DNSPlugin().Name
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
		DnsPlugin:    dnsPluginName,
		RunPlugin:    runPluginName,
		Run:          a.AppRun.Proto(),
		Deploy:       a.AppDeploy.Proto(),
		Needs:        needs,
		Properties:   plugin_util.MustNewStruct(a.Other),
	}
}

func (a *BasicApp) DeployPlugin() *plugins.Plugin {
	return a.deployPlugin
}

func (a *BasicApp) DNSPlugin() *plugins.Plugin {
	return a.dnsPlugin
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
