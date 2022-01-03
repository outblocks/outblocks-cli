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
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/templates"
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
}

type InitOptions struct {
	Overwrite bool

	Name             string
	DeployPlugin     string
	RunPlugin        string
	DefaultRunPlugin string
	DNSDomain        string

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

	Values map[string]interface{}
}

func (d *Init) Run(ctx context.Context) error {
	curDir, err := os.Getwd()
	if err != nil {
		return merry.Errorf("can't get current working dir: %w", err)
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

	err = cfg.LoadPlugins(ctx, d.log, loader, d.hostAddr)
	if err != nil {
		return err
	}

	pluginOpts := map[string]map[string]interface{}{
		"gcp": {
			"project": d.opts.GCP.Project,
			"region":  d.opts.GCP.Region,
		},
	}

	initCfg := &projectInit{
		Project: cfg,
		Values:  make(map[string]interface{}),
	}

	for _, plug := range cfg.Plugins {
		initRes, err := plug.Loaded().Client().ProjectInit(ctx, cfg.Name, []string{d.opts.DeployPlugin}, []string{d.opts.RunPlugin}, pluginOpts[plug.Name])
		if err != nil {
			if st, ok := util.StatusFromError(err); ok && st.Code() == codes.Aborted {
				d.log.Println("Init canceled.")
				return nil
			}

			return err
		}

		if initRes != nil {
			plug.Other = initRes.Properties.AsMap()

			for k, v := range plug.Other {
				valueKey := fmt.Sprintf("%s_%s", plug.Name, k)
				initCfg.Values[valueKey] = v
				plug.Other[k] = fmt.Sprintf("${var.%s}", valueKey)
			}
		}
	}

	// Generate Project.YAML
	tmpl := templates.ProjectYAMLTemplate()

	var projectYAML bytes.Buffer

	err = tmpl.Execute(&projectYAML, initCfg)
	if err != nil {
		return err
	}

	err = fileutil.WriteFile(config.ProjectYAMLName+".yaml", projectYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	// Generate Values.YAML
	tmpl = templates.ValuesYAMLTemplate()

	var valuesYAML bytes.Buffer

	err = tmpl.Execute(&valuesYAML, initCfg)
	if err != nil {
		return err
	}

	err = fileutil.WriteFile("dev.values.yaml", valuesYAML.Bytes(), 0644)
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

	// Proceed to questions about defaults.
	qs = []*survey.Question{}

	if d.opts.DefaultRunPlugin == "" {
		qs = append(qs, &survey.Question{
			Name: "DefaultRunPlugin",
			Prompt: &survey.Select{
				Message: "Default run plugin:",
				Options: []string{
					config.RunPluginDirect,
					d.opts.RunPlugin,
				},
				Default: config.RunPluginDirect,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Default run plugin:"), pterm.Cyan(d.opts.DefaultRunPlugin))
	}

	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, errInitCanceled
			}

			return nil, err
		}
	}

	// Setup config object.
	cfg.Name = d.opts.Name

	_, latestDeployVersion, err := loader.MatchingVersion(ctx, d.opts.DeployPlugin, "", nil)
	if err != nil {
		return nil, merry.Errorf("error retrieving latest version of plugin '%s': %w", d.opts.RunPlugin, err)
	}

	_, latestRunVersion, err := loader.MatchingVersion(ctx, d.opts.RunPlugin, "", nil)
	if err != nil {
		return nil, merry.Errorf("error retrieving latest version of plugin '%s': %w", d.opts.RunPlugin, err)
	}

	cfg.Plugins = []*config.Plugin{
		{Name: d.opts.DeployPlugin, Version: fmt.Sprintf("^%s", latestDeployVersion.String())},
		{Name: d.opts.RunPlugin, Version: fmt.Sprintf("^%s", latestRunVersion.String())},
	}

	cfg.Defaults = &config.Defaults{
		Run: config.DefaultsRun{
			Plugin: d.opts.DefaultRunPlugin,
		},
		Deploy: config.DefaultsDeploy{
			Plugin: d.opts.DeployPlugin,
		},
	}

	cfg.State = &config.State{
		Type: d.opts.DeployPlugin,
	}

	cfg.DNS = append(cfg.DNS, &config.DNS{
		Domain: d.opts.DNSDomain,
	})

	return cfg, nil
}
