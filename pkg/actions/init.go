package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Masterminds/sprig"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/templates"
	"github.com/pterm/pterm"
)

var (
	validNameRegex  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	errInitCanceled = errors.New("init canceled")
)

type Init struct {
	log            logger.Logger
	loader         *plugins.Loader
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

func NewInit(log logger.Logger, loader *plugins.Loader, pluginCacheDir string, opts *InitOptions) *Init {
	return &Init{
		log:            log,
		loader:         loader,
		pluginCacheDir: pluginCacheDir,
		opts:           opts,
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"toYaml": toYaml,
	}
}

func (d *Init) Run(ctx context.Context, cfg *config.Project) error {
	curDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working dir error: %w", err)
	}

	if cfg == nil {
		cfg = &config.Project{}

		d.loader = plugins.NewLoader(curDir, d.pluginCacheDir)
	} else if !d.opts.Overwrite {
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

	cfg, err = d.prompt(ctx, cfg, curDir)
	if errors.Is(err, errInitCanceled) {
		d.log.Println("Init canceled.")
		return nil
	}

	if err != nil {
		return err
	}

	err = cfg.LoadPlugins(ctx, d.log, d.loader)
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

	err = ioutil.WriteFile(config.ProjectYAMLName+".yaml", projectYAML.Bytes(), 0644)
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

	err = ioutil.WriteFile("dev.values.yaml", valuesYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	// TODO: Apps adding

	return err
}

func (d *Init) prompt(ctx context.Context, cfg *config.Project, curDir string) (*config.Project, error) {
	var qs []*survey.Question

	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:   "name",
			Prompt: &survey.Input{Message: "Name of project?", Default: filepath.Base(curDir)},
			Validate: func(val interface{}) error {
				// since we are validating an Input, the assertion will always succeed
				if str, ok := val.(string); !ok || !validNameRegex.MatchString(str) {
					return errors.New("must start with a letter and consist only of letters, numbers, underscore or hyphens")
				}
				return nil
			},
		})
	} else {
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

	answers := *d.opts

	if len(qs) != 0 {
		err := survey.Ask(qs, &answers)
		if err != nil {
			return nil, errInitCanceled
		}
	}

	cfg.Name = answers.Name

	_, latestDeployVersion, err := d.loader.MatchingVersion(ctx, answers.DeployPlugin, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest version of plugin '%s': %w", answers.RunPlugin, err)
	}

	_, latestRunVersion, err := d.loader.MatchingVersion(ctx, answers.RunPlugin, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest version of plugin '%s': %w", answers.RunPlugin, err)
	}

	cfg.Plugins = []*config.Plugin{
		{Name: answers.DeployPlugin, Version: fmt.Sprintf("^%s", latestDeployVersion.String())},
		{Name: answers.RunPlugin, Version: fmt.Sprintf("^%s", latestRunVersion.String())},
	}

	cfg.State = &config.State{
		Type: answers.DeployPlugin,
	}

	cfg.DNS = append(cfg.DNS, &config.DNS{
		Domain: answers.DNSDomain,
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
