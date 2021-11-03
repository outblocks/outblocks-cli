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
	"github.com/outblocks/outblocks-plugin-go/types"
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
	PluginType() *types.App
	RunInfo() *AppRun
	DeployInfo() *AppDeploy
	SupportsLocal() bool
	YAMLError(path, msg string) error

	DeployPlugin() *plugins.Plugin
	RunPlugin() *plugins.Plugin
	DNSPlugin() *plugins.Plugin
}

type BasicApp struct {
	AppName         string                 `json:"name"`
	AppType         string                 `json:"type"`
	AppURL          string                 `json:"url"`
	AppPathRedirect string                 `json:"pathRedirect"`
	AppEnv          map[string]string      `json:"env"`
	AppDir          string                 `json:"dir"`
	AppRun          *AppRun                `json:"run"`
	AppDeploy       *AppDeploy             `json:"deploy"`
	Needs           map[string]*AppNeed    `json:"needs"`
	Other           map[string]interface{} `yaml:"-,remain"`

	url          *url.URL
	yamlPath     string
	yamlData     []byte
	deployPlugin *plugins.Plugin
	dnsPlugin    *plugins.Plugin
	runPlugin    *plugins.Plugin
}

type AppRun struct {
	Plugin  string                 `json:"plugin,omitempty"`
	Command string                 `json:"command,omitempty"`
	Port    int                    `json:"port,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Other   map[string]interface{} `yaml:"-,remain"`
}

type AppDeploy struct {
	Plugin string                 `json:"plugin,omitempty"`
	Env    map[string]string      `json:"env,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

func (a *BasicApp) Validate() error {
	return validation.ValidateStruct(a,
		validation.Field(&a.AppURL, validation.Match(ValidURLRegex)),
	)
}

func (a *BasicApp) Normalize(cfg *Project) error {
	var err error

	if a.AppRun == nil {
		a.AppRun = &AppRun{}
	}

	if a.AppDir == "" {
		a.AppDir = filepath.Dir(a.yamlPath)
	} else {
		a.AppDir, err = filepath.Abs(a.AppDir)
		if err != nil {
			return a.YAMLError("$.dir", "App.Dir is invalid")
		}
	}

	if a.AppDeploy == nil {
		a.AppDeploy = &AppDeploy{}
	}

	if a.AppName == "" {
		a.AppName = filepath.Base(a.AppDir)
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
			return a.YAMLError("$.url", "App.URL is invalid")
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
		a.dnsPlugin = cfg.FindDNSPlugin(a.AppURL)
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

func (a *BasicApp) PluginType() *types.App {
	needs := make(map[string]*types.AppNeed, len(a.Needs))

	for k, n := range a.Needs {
		needs[k] = n.PluginType()
	}

	var appURL string
	if a.url != nil {
		appURL = a.url.String()
	}

	var dnsPluginName, deployPluginName string

	if a.DeployPlugin() != nil {
		deployPluginName = a.DeployPlugin().Name
	}

	if a.DNSPlugin() != nil {
		dnsPluginName = a.DNSPlugin().Name
	}

	return &types.App{
		ID:           a.ID(),
		DeployPlugin: deployPluginName,
		DNSPlugin:    dnsPluginName,
		Env:          a.Env(),
		Dir:          a.Dir(),
		Name:         a.AppName,
		Type:         a.Type(),
		URL:          appURL,
		PathRedirect: a.AppPathRedirect,
		Needs:        needs,
		Properties:   a.Other,
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

func (a *BasicApp) RunInfo() *AppRun {
	return a.AppRun
}

func (a *BasicApp) DeployInfo() *AppDeploy {
	return a.AppDeploy
}

func (a *BasicApp) PathRedirect() string {
	return a.AppPathRedirect
}

func (a *BasicApp) Env() map[string]string {
	return a.AppEnv
}
