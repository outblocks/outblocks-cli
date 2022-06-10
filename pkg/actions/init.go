package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/templating"
	"github.com/outblocks/outblocks-cli/templates"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/pterm/pterm"
	"google.golang.org/grpc/codes"
)

var (
	errInitCanceled = errors.New("init canceled")
)

type Init struct {
	log            logger.Logger
	pluginCacheDir string
	hostAddr       string
	opts           *InitOptions
	input          map[string]interface{}

	template *templating.Template
}

type InitOptions struct {
	Overwrite bool
	Path      string

	Name              string
	DeployPlugin      string
	RunPlugin         string
	DNSPlugin         string
	DNSDomain         string
	Template          string
	TemplateValueOpts *values.Options

	GCP struct {
		Project string
		Region  string
	}

	defaultRunPlugin    string
	defaultDeployPlugin string
	defaultDNSPlugin    string
	statePlugin         string
}

func (o *InitOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Name, validation.Required, validation.By(validateInitName)),
	)
}

func NewInit(log logger.Logger, pluginCacheDir, hostAddr string, opts *InitOptions) *Init {
	return &Init{
		log:            log,
		pluginCacheDir: pluginCacheDir,
		hostAddr:       hostAddr,
		opts:           opts,
	}
}

type projectInit struct {
	*config.Project

	DNSTemplate []*config.DNS
}

type valuesInit struct {
	*projectInit

	DNSDomain      string
	TemplateValues []byte
	PluginValues   map[string]interface{}
}

func (d *Init) promptEnv(ctx context.Context, cfg *projectInit, env string, input map[string]interface{}) error {
	d.log.Section().Printf("%s environment configuration", util.Title(env))

	vals := &valuesInit{
		projectInit:  cfg,
		PluginValues: make(map[string]interface{}),
	}

	var (
		pluginOpts map[string]map[string]interface{}
		err        error
	)

	if d.template != nil {
		vals.TemplateValues, err = d.processValuesTemplate(input)
		if err != nil {
			return err
		}
	}

	if env == "dev" {
		pluginOpts = map[string]map[string]interface{}{
			"gcp": {
				"project": d.opts.GCP.Project,
				"region":  d.opts.GCP.Region,
			},
		}

		vals.DNSDomain = d.opts.DNSDomain
	}

	if len(cfg.DNSTemplate) == 0 {
		if vals.DNSDomain == "" {
			err := survey.AskOne(&survey.Input{
				Message: "Main domain you plan to use for deployments:",
				Default: "example.com",
			}, &vals.DNSDomain)
			if err != nil {
				return err
			}
		} else {
			d.log.Printf("%s %s\n", pterm.Bold.Sprint("Main domain you plan to use for deployments"), pterm.Cyan(vals.DNSDomain))
		}
	}

	for _, plug := range cfg.Plugins {
		initRes, err := plug.Loaded().Client().ProjectInit(ctx, cfg.Name, []string{d.opts.DeployPlugin}, []string{d.opts.RunPlugin}, d.opts.DNSPlugin, pluginOpts[plug.Name])
		if err != nil {
			if st, ok := util.StatusFromError(err); ok && st.Code() == codes.Aborted {
				return errInitCanceled
			}

			return err
		}

		if initRes != nil {
			plug.Other = initRes.Properties.AsMap()

			for k, v := range plug.Other {
				valueKey := fmt.Sprintf("%s_%s", plug.Name, k)
				vals.PluginValues[valueKey] = v
				plug.Other[k] = fmt.Sprintf("${var.%s}", valueKey)
			}
		}
	}

	// Generate Values.YAML
	tmpl := templates.ValuesYAMLTemplate()

	var valuesYAML bytes.Buffer

	err = tmpl.Execute(&valuesYAML, vals)
	if err != nil {
		return err
	}

	err = fileutil.WriteFile(filepath.Join(d.opts.Path, fmt.Sprintf("%s.values.yaml", env)), valuesYAML.Bytes(), 0o644)

	return err
}

func (d *Init) installTemplate() error {
	d.log.Printf("Downloading template %s...\n", d.opts.Template)

	inst, err := templating.NewInstaller(d.opts.Template, "")
	if err != nil {
		return merry.Errorf("failed to create template installer for %s: %w", d.opts.Template, err)
	}

	err = inst.Download()
	if err != nil {
		return merry.Errorf("failed to download template %s: %w", d.opts.Template, err)
	}

	d.log.Println("Copying template...")

	err = inst.CopyTo(d.opts.Path)
	if err != nil {
		return merry.Errorf("failed to copy template to %s: %w", d.opts.Path, err)
	}

	t, err := templating.LoadTemplate(d.opts.Path)
	if err != nil {
		return err
	}

	d.template = t

	return nil
}

func (d *Init) processProjectTemplate(cfg *projectInit, input map[string]interface{}) error {
	if d.template.HasProjectPrompt() {
		d.log.Println("Parsing project template...")
		d.log.Println()
	}

	err := d.template.ParseProjectTemplate(d.opts.Name, input)
	if err != nil {
		return err
	}

	if len(d.template.Project.DNS) > 0 {
		cfg.DNSTemplate = d.template.Project.DNS
	}

	if len(d.template.Project.Dependencies) > 0 {
		cfg.Dependencies = d.template.Project.Dependencies
	}

	if len(d.template.Project.Plugins) > 0 {
		cfg.Plugins = d.template.Project.Plugins
	}

	return nil
}

func (d *Init) processValuesTemplate(input map[string]interface{}) ([]byte, error) {
	if d.template.HasValuesPrompt() {
		d.log.Println("Parsing values template...")
		d.log.Println()
	}

	return d.template.ParseValuesTemplate(input)
}

func (d *Init) promptBasicInfo() error {
	var qs []*survey.Question

	d.log.Section().Printf("Project setup")

	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Name of project:", Default: filepath.Base(d.opts.Path)},
			Validate: validateInitName,
		})
	} else {
		err := validateInitName(d.opts.Name)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Name of project:"), pterm.Cyan(d.opts.Name))
	}

	err := survey.Ask(qs, d.opts)

	return err
}

func (d *Init) runPrompt(ctx context.Context, cfg *projectInit) error { // nolint: gocyclo
	if d.opts.Template != "" && plugin_util.DirExists(d.opts.Path) && !d.opts.Overwrite {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Destination directory already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			return errInitCanceled
		}
	}

	projectFile := fileutil.FindYAML(filepath.Join(d.opts.Path, config.ProjectYAMLName))

	if !d.opts.Overwrite && projectFile != "" {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Project config already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			return errInitCanceled
		}

		_ = os.Remove(projectFile)
	}

	if d.opts.Template != "" {
		err := d.installTemplate()
		if err != nil {
			return err
		}
	}

	err := d.promptBasicInfo()
	if err != nil {
		return err
	}

	if d.template != nil {
		err := d.processProjectTemplate(cfg, util.MapLookupPath(d.input, "project"))
		if err != nil {
			return err
		}
	}

	if !plugin_util.DirExists(d.opts.Path) {
		err := fileutil.MkdirAll(d.opts.Path, 0o755)
		if err != nil {
			return merry.Errorf("failed to create dir %s: %w", d.opts.Path, err)
		}
	}

	err = d.prompt(ctx, cfg)
	if err != nil {
		return err
	}

	vals := util.MapLookupPath(d.input, "values")

	for k := range vals {
		err = d.promptEnv(ctx, cfg, k, util.MapLookupPath(vals, k))
		if err != nil {
			return err
		}
	}

	if len(vals) == 0 {
		err = d.promptEnv(ctx, cfg, "dev", nil)
		if err != nil {
			return err
		}

		var addProd bool

		_ = survey.AskOne(&survey.Confirm{
			Message: "Do you want to add production config as well? You can add it later on by creating production.values.yaml based on dev.values.yaml.",
		}, addProd)

		if addProd {
			err = d.promptEnv(ctx, cfg, "production", nil)
		}
	}

	return err
}

func (d *Init) Run(ctx context.Context) error {
	curDir, err := os.Getwd()
	if err != nil {
		return merry.Errorf("can't get current working dir: %w", err)
	}

	if d.opts.Path == "" {
		d.opts.Path = curDir
	}

	d.opts.Path, err = filepath.Abs(d.opts.Path)
	if err != nil {
		return err
	}

	initCfg := &projectInit{
		Project: &config.Project{},
	}

	v, err := d.opts.TemplateValueOpts.MergeValues(ctx, curDir, getter.All())
	if err != nil {
		return merry.Errorf("error getting template values: %w", err)
	}

	d.input = v

	d.opts.Name = getMapStringVal(d.input, d.opts.Name, "project", "name")
	d.opts.DeployPlugin = getMapStringVal(d.input, d.opts.DeployPlugin, "project", "deploy_plugin")
	d.opts.RunPlugin = getMapStringVal(d.input, d.opts.RunPlugin, "project", "run_plugin")
	d.opts.defaultDeployPlugin = getMapStringVal(d.input, d.opts.defaultDeployPlugin, "project", "default_deploy_plugin")
	d.opts.defaultRunPlugin = getMapStringVal(d.input, d.opts.defaultRunPlugin, "project", "default_run_plugin")
	d.opts.defaultDNSPlugin = getMapStringVal(d.input, d.opts.defaultDNSPlugin, "project", "default_dns_plugin")
	d.opts.statePlugin = getMapStringVal(d.input, d.opts.statePlugin, "project", "state_plugin")

	d.opts.GCP.Project = getMapStringVal(d.input, d.opts.GCP.Project, "project", "gcp", "project")
	d.opts.GCP.Region = getMapStringVal(d.input, d.opts.GCP.Region, "project", "gcp", "region")

	err = d.runPrompt(ctx, initCfg)
	if errors.Is(err, errInitCanceled) || errors.Is(err, terminal.InterruptErr) {
		d.log.Println("Init canceled.")
		return nil
	}

	if err != nil {
		return err
	}

	// Generate Project.YAML
	tmpl := templates.ProjectYAMLTemplate()

	var projectYAML bytes.Buffer

	err = tmpl.Execute(&projectYAML, initCfg)
	if err != nil {
		return err
	}

	err = fileutil.WriteFile(filepath.Join(d.opts.Path, config.ProjectYAMLName+".yaml"), projectYAML.Bytes(), 0o644)
	if err != nil {
		return err
	}

	return err
}

func validateInitName(val interface{}) error {
	return util.RegexValidator(config.ValidNameRegex, "must start with a letter and consist only of letters, numbers, underscore or hyphens")(val)
}

func (d *Init) promptPlugins(cfg *projectInit) error {
	var qs []*survey.Question

	if d.opts.DeployPlugin == "" {
		qs = append(qs, &survey.Question{
			Name: "deployPlugin",
			Prompt: &survey.Select{
				Message: "Deployment plugin to be used:",
				Options: []string{"gcp"},
				Default: "gcp",
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Deployment plugin to be used:"), pterm.Cyan(d.opts.DeployPlugin))
	}

	if d.opts.RunPlugin == "" {
		qs = append(qs, &survey.Question{
			Name: "runPlugin",
			Prompt: &survey.Select{
				Message: "Run plugin to be used:",
				Options: []string{"docker"},
				Default: "docker",
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Run plugin to be used:"), pterm.Cyan(d.opts.RunPlugin))
	}

	if d.opts.DNSPlugin == "" {
		qs = append(qs, &survey.Question{
			Name: "dnsPlugin",
			Prompt: &survey.Select{
				Message: "DNS plugin to be used:",
				Options: []string{"none", "cloudflare"},
				Default: "cloudflare",
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("DNS plugin to be used:"), pterm.Cyan(d.opts.DNSPlugin))
	}

	// Ask questions.
	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			return err
		}
	}

	cfg.Plugins = []*config.Plugin{
		{Name: d.opts.DeployPlugin, Version: ""},
		{Name: d.opts.RunPlugin, Version: ""},
	}

	if d.opts.DNSPlugin != "" && d.opts.DNSPlugin != "none" {
		cfg.Plugins = append(cfg.Plugins, &config.Plugin{Name: d.opts.DNSPlugin, Version: ""})
	}

	return nil
}

func (d *Init) prompt(ctx context.Context, cfg *projectInit) error { // nolint: gocyclo
	// Setup config object.
	cfg.Name = d.opts.Name

	// Setup plugins.
	if len(cfg.Plugins) == 0 {
		err := d.promptPlugins(cfg)
		if err != nil {
			return err
		}
	}

	loader := plugins.NewLoader(d.opts.Path, d.pluginCacheDir)

	for _, plug := range cfg.Plugins {
		if plug.Version != "" {
			continue
		}

		_, latestVersion, err := loader.MatchingVersion(ctx, plug.Name, plug.Source, nil)
		if err != nil {
			return merry.Errorf("error retrieving latest version of plugin '%s': %w", plug.Name, err)
		}

		plug.Version = fmt.Sprintf("^%s", latestVersion.String())
	}

	// Normalize plugins.
	for i, plug := range cfg.Plugins {
		err := plug.Normalize(i, cfg.Project)
		if err != nil {
			return err
		}
	}

	err := cfg.LoadPlugins(ctx, d.log, loader, d.hostAddr)
	if err != nil {
		return err
	}

	// Proceed to questions about defaults.
	var (
		deployPlugins []string
		runPlugins    []string
		dnsPlugins    []string
		statePlugins  []string
	)

	for _, plug := range cfg.LoadedPlugins() {
		if plug.HasAction(plugins.ActionRun) {
			runPlugins = append(runPlugins, plug.Name)
		}

		if plug.HasAction(plugins.ActionDeploy) {
			deployPlugins = append(deployPlugins, plug.Name)
		}

		if plug.HasAction(plugins.ActionDNS) {
			dnsPlugins = append(dnsPlugins, plug.Name)
		}

		if plug.HasAction(plugins.ActionState) {
			statePlugins = append(statePlugins, plug.Name)
		}
	}

	if len(deployPlugins) > 1 {
		err = survey.AskOne(&survey.Select{
			Message: "Default deploy plugin:",
			Options: deployPlugins,
		}, &d.opts.defaultDeployPlugin)
		if err != nil {
			return err
		}
	} else if len(deployPlugins) == 1 {
		d.opts.defaultDeployPlugin = deployPlugins[0]
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Default deploy plugin:"), pterm.Cyan(d.opts.defaultDeployPlugin))
	}

	runPlugins = append(runPlugins, config.RunPluginDirect)

	if len(runPlugins) > 1 {
		err = survey.AskOne(&survey.Select{
			Message: "Default run plugin:",
			Options: runPlugins,
			Default: config.RunPluginDirect,
		}, &d.opts.defaultRunPlugin)
		if err != nil {
			return err
		}
	} else {
		d.opts.defaultRunPlugin = runPlugins[0]
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Default run plugin:"), pterm.Cyan(d.opts.defaultRunPlugin))
	}

	if len(dnsPlugins) > 1 {
		err = survey.AskOne(&survey.Select{
			Message: "Default DNS plugin:",
			Options: dnsPlugins,
		}, &d.opts.defaultDNSPlugin)
		if err != nil {
			return err
		}
	} else if len(dnsPlugins) == 1 {
		d.opts.defaultDNSPlugin = deployPlugins[0]
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Default DNS plugin:"), pterm.Cyan(d.opts.defaultDNSPlugin))
	}

	cfg.Defaults = &config.Defaults{
		Run: config.DefaultsRun{
			Plugin: d.opts.defaultRunPlugin,
		},
		Deploy: config.DefaultsDeploy{
			Plugin: d.opts.defaultDeployPlugin,
		},
		DNS: config.DefaultsDNS{
			Plugin: d.opts.defaultDNSPlugin,
		},
	}

	// Process state.
	if len(statePlugins) > 1 {
		err = survey.AskOne(&survey.Select{
			Message: "State plugin:",
			Options: statePlugins,
		}, &d.opts.statePlugin)
		if err != nil {
			return err
		}
	} else if len(statePlugins) == 1 {
		d.opts.statePlugin = statePlugins[0]
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("State plugin:"), pterm.Cyan(d.opts.statePlugin))
	}

	cfg.State = &config.State{
		Type: d.opts.statePlugin,
	}

	// Add DNS.
	if len(cfg.DNSTemplate) == 0 {
		cfg.DNSTemplate = append(cfg.DNSTemplate, &config.DNS{
			Domains: []string{
				"*.${var.base_url}",
				"${var.base_url}",
			},
		})
	}

	return nil
}

func getMapStringVal(m map[string]interface{}, def string, keys ...string) string {
	if len(keys) < 1 {
		return ""
	}

	if len(keys) > 1 {
		m = util.MapLookupPath(m, keys[:len(keys)-1]...)
	}

	v, ok := m[keys[len(keys)-1]]
	if !ok {
		return def
	}

	vs, ok := v.(string)
	if !ok {
		return def
	}

	return vs
}
