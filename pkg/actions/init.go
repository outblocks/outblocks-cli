package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Masterminds/sprig"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/templates"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/pterm/pterm"
)

var (
	errInitCanceled = errors.New("init canceled")
)

type Init struct {
	log            logger.Logger
	pluginCacheDir string
	opts           *InitOptions
}

type InitOptions struct {
	Overwrite bool

	Name         string
	DeployPlugin string
	RunPlugin    string
	DNSDomain    string

	GCP struct {
		Project string
		Region  string
	}
}

func (o *InitOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Name, validation.Required, validation.By(validateInitName)),
	)
}

func NewInit(log logger.Logger, pluginCacheDir string, opts *InitOptions) *Init {
	return &Init{
		log:            log,
		pluginCacheDir: pluginCacheDir,
		opts:           opts,
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"toYaml": toYaml,
	}
}

func (d *Init) Run(ctx context.Context) error {
	curDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("can't get current working dir: %w", err)
	}

	cfg := &config.Project{}
	loader := plugins.NewLoader(curDir, d.pluginCacheDir)

	if !d.opts.Overwrite && fileutil.FindYAML(filepath.Join(curDir, config.ProjectYAMLName)) != "" {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Project config already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			d.log.Println("Init canceled.")
			return nil
		}
	}

	cfg, err = d.prompt(ctx, cfg, loader, curDir)
	if errors.Is(err, errInitCanceled) {
		d.log.Println("Init canceled.")
		return nil
	}

	if err != nil {
		return err
	}

	// Normalize plugins.
	for i, plug := range cfg.Plugins {
		err = plug.Normalize(i, cfg)
		if err != nil {
			return err
		}
	}

	err = cfg.LoadPlugins(ctx, d.log, loader)
	if err != nil {
		return err
	}

	pluginOpts := map[string]map[string]interface{}{
		"gcp": {
			"project": d.opts.GCP.Project,
			"region":  d.opts.GCP.Region,
		},
	}

	for _, plug := range cfg.Plugins {
		initRes, err := plug.Loaded().Client().Init(ctx, cfg.Name, []string{d.opts.DeployPlugin}, []string{d.opts.RunPlugin}, pluginOpts[plug.Name])
		if err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				d.log.Println("Init canceled.")
				return nil
			}

			return err
		}

		if initRes != nil {
			plug.Other = initRes.Properties
		}
	}

	// Generate Project.YAML
	tmpl := template.Must(template.New("project").Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Parse(templates.ProjectYAML))

	var projectYAML bytes.Buffer

	err = tmpl.Execute(&projectYAML, cfg)
	if err != nil {
		return err
	}

	err = plugin_util.WriteFile(config.ProjectYAMLName+".yaml", projectYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	// Generate Values.YAML
	tmpl = template.Must(template.New("values").Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Parse(templates.ValuesYAML))

	var valuesYAML bytes.Buffer

	err = tmpl.Execute(&valuesYAML, cfg)
	if err != nil {
		return err
	}

	err = plugin_util.WriteFile("dev.values.yaml", valuesYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	return err
}

func validateInitName(val interface{}) error {
	return util.RegexValidator(config.ValidNameRegex, "must start with a letter and consist only of letters, numbers, underscore or hyphens")(val)
}

func (d *Init) prompt(ctx context.Context, cfg *config.Project, loader *plugins.Loader, curDir string) (*config.Project, error) {
	var qs []*survey.Question

	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Name of project:", Default: filepath.Base(curDir)},
			Validate: validateInitName,
		})
	} else {
		err := validateInitName(d.opts.Name)
		if err != nil {
			return nil, err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Name of project:"), pterm.Cyan(d.opts.Name))
	}

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

	if d.opts.DNSDomain == "" {
		qs = append(qs, &survey.Question{
			Name: "dnsDomain",
			Prompt: &survey.Input{
				Message: "Main domain you plan to use for deployments:",
				Default: "example.com",
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Main domain you plan to use for deployments"), pterm.Cyan(d.opts.DNSDomain))
	}

	// Ask questions.
	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, errInitCanceled
			}

			return nil, err
		}
	}

	cfg.Name = d.opts.Name

	_, latestDeployVersion, err := loader.MatchingVersion(ctx, d.opts.DeployPlugin, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest version of plugin '%s': %w", d.opts.RunPlugin, err)
	}

	_, latestRunVersion, err := loader.MatchingVersion(ctx, d.opts.RunPlugin, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest version of plugin '%s': %w", d.opts.RunPlugin, err)
	}

	cfg.Plugins = []*config.Plugin{
		{Name: d.opts.DeployPlugin, Version: fmt.Sprintf("^%s", latestDeployVersion.String())},
		{Name: d.opts.RunPlugin, Version: fmt.Sprintf("^%s", latestRunVersion.String())},
	}

	cfg.State = &config.State{
		Type: d.opts.DeployPlugin,
	}

	cfg.DNS = append(cfg.DNS, &config.DNS{
		Domain: d.opts.DNSDomain,
	})

	return cfg, nil
}

func toYaml(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}

	return string(data)
}
