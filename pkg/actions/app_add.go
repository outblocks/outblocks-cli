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
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/sprig"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/templates"
	"github.com/pterm/pterm"
)

var (
	errAppAddCanceled = errors.New("adding app canceled")
	validValueRegex   = regexp.MustCompile(`^[a-zA-Z0-9{}\-_.]+$`)
)

type AppAdd struct {
	log  logger.Logger
	opts *AppAddOptions
}

type staticAppInfo struct {
	App  config.StaticApp
	URL  string
	Type string
}

type AppStaticOptions struct {
	BuildCommand string
	BuildDir     string
	Routing      string
}

func (o *AppStaticOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Routing, validation.In(util.InterfaceSlice(config.StaticAppRoutings)...)),
	)
}

type AppAddOptions struct {
	Overwrite bool

	OutputPath string
	Name       string
	Type       string
	URL        string

	Static AppStaticOptions
}

func (o *AppAddOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Name, validation.Required, validation.Match(config.ValidNameRegex)),
		validation.Field(&o.Type, validation.Required, validation.In(util.InterfaceSlice(config.ValidAppTypes)...)),
		validation.Field(&o.URL, validation.Required, validation.Match(validValueRegex)),
		validation.Field(&o.Static),
	)
}

func NewAppAdd(log logger.Logger, opts *AppAddOptions) *AppAdd {
	return &AppAdd{
		log:  log,
		opts: opts,
	}
}

func (d *AppAdd) Run(ctx context.Context, cfg *config.Project) error {
	appInfo, err := d.prompt(ctx, cfg)
	if errors.Is(err, errAppAddCanceled) {
		d.log.Println("Adding application canceled.")
		return nil
	}

	if err != nil {
		return err
	}

	// Generate Application.YAML
	var (
		tmpl *template.Template
		path string
	)

	switch app := appInfo.(type) {
	case *staticAppInfo:
		tmpl = template.Must(template.New("static_app").Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Parse(templates.StaticAppYAML))
		path = app.App.AppPath
	default:
		return fmt.Errorf("unsupported app type (WIP)")
	}

	var appYAML bytes.Buffer

	err = tmpl.Execute(&appYAML, appInfo)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(path, config.AppYAMLName+".yaml"), appYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (d *AppAdd) prompt(_ context.Context, cfg *config.Project) (interface{}, error) {
	var qs []*survey.Question

	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Name of application:"},
			Validate: util.RegexValidator(config.ValidNameRegex, "must start with a letter and consist only of letters, numbers, underscore or hyphens"),
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Name of application:"), pterm.Cyan(d.opts.Name))
	}

	if d.opts.Type == "" {
		qs = append(qs, &survey.Question{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Type of application:",
				Options: config.ValidAppTypes,
				Default: config.TypeStatic,
			},
		})
	} else {
		d.opts.Type = strings.ToLower(d.opts.Type)
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Type of application:"), pterm.Cyan(d.opts.Type))
	}

	if d.opts.URL == "" {
		defaultURL := ""

		if len(cfg.DNS) > 0 {
			defaultURL = cfg.DNS[0].Domain
		}

		qs = append(qs, &survey.Question{
			Name:     "url",
			Prompt:   &survey.Input{Message: "URL of application:", Default: defaultURL},
			Validate: util.RegexValidator(validValueRegex, "invalid URL, example example.com/some_path/run or using vars: ${var.base_url}/some_path/run"),
		})
	} else {
		d.opts.Type = strings.ToLower(d.opts.URL)
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("URL of application:"), pterm.Cyan(d.opts.URL))
	}

	answers := *d.opts

	// Get basic info about app.
	if len(qs) != 0 {
		err := survey.Ask(qs, &answers)
		if err != nil {
			return nil, errAppAddCanceled
		}
	}

	err := answers.Validate()
	if err != nil {
		return nil, err
	}

	if answers.OutputPath == "" {
		answers.OutputPath = filepath.Join(cfg.Path, answers.Type, answers.Name)
	}

	stat, err := os.Stat(answers.OutputPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(answers.OutputPath, 0755)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if stat != nil && !stat.IsDir() {
		return nil, fmt.Errorf("output path '%s' is not a directory", answers.OutputPath)
	}

	if !d.opts.Overwrite && fileutil.FindYAML(filepath.Join(answers.OutputPath, config.AppYAMLName)) != "" {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Application config already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			return nil, errAppAddCanceled
		}
	}

	switch answers.Type {
	case config.TypeStatic:
		return d.promptStatic(&answers)
	default:
		return nil, fmt.Errorf("unsupported app type (WIP)")
	}
}

func (d *AppAdd) promptStatic(answers *AppAddOptions) (*staticAppInfo, error) {
	var qs []*survey.Question

	if answers.Static.BuildDir == "" {
		qs = append(qs, &survey.Question{
			Name:   "builddir",
			Prompt: &survey.Input{Message: "Build directory of application:", Default: config.DefaultStaticAppBuildDir},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build directory of application:"), pterm.Cyan(answers.Static.BuildDir))
	}

	if answers.Static.BuildCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "buildcommand",
			Prompt: &survey.Input{Message: "Build command of application (optional):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build command of application:"), pterm.Cyan(answers.Static.BuildCommand))
	}

	if answers.Static.Routing == "" {
		qs = append(qs, &survey.Question{
			Name: "routing",
			Prompt: &survey.Select{
				Message: "Routing of application:",
				Options: config.StaticAppRoutings,
				Default: config.StaticAppRoutingReact,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Routing of application:"), pterm.Cyan(answers.Static.BuildCommand))
	}

	// Get info about static app.
	if len(qs) != 0 {
		err := survey.Ask(qs, &answers.Static)
		if err != nil {
			return nil, errAppAddCanceled
		}
	}

	// Skip "type" if it can be deduced from path.
	if config.KnownType(config.DetectAppType(answers.OutputPath)) != "" {
		answers.Type = ""
	}

	return &staticAppInfo{
		App: config.StaticApp{
			BasicApp: config.BasicApp{
				AppName: answers.Name,
				AppURL:  answers.URL,
				AppPath: answers.OutputPath,
			},
			Build: &config.StaticAppBuild{
				Command: answers.Static.BuildCommand,
				Dir:     answers.Static.BuildDir,
			},
			Routing: answers.Static.Routing,
		},

		URL:  answers.URL,
		Type: answers.Type,
	}, nil
}
