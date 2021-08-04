package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/pterm/pterm"
)

var (
	errAppAddCanceled = errors.New("adding app canceled")
	validURLRegex     = regexp.MustCompile(`^(https?://)?[a-zA-Z0-9{}\-_.]+$`)
)

type AppAdd struct {
	log  logger.Logger
	cfg  *config.Project
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
	DevCommand   string
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
		validation.Field(&o.Type, validation.Required, validation.In(util.InterfaceSlice(config.ValidAppTypes)...)),
		validation.Field(&o.Static),
	)
}

func NewAppAdd(log logger.Logger, cfg *config.Project, opts *AppAddOptions) *AppAdd {
	return &AppAdd{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *AppAdd) Run(ctx context.Context) error {
	curDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("can't get current working dir: %w", err)
	}

	appInfo, err := d.prompt(curDir)
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

	destFile := filepath.Join(path, config.AppYAMLName+".yaml")

	err = plugin_util.WriteFile(destFile, appYAML.Bytes(), 0644)
	if err != nil {
		return err
	}

	return nil
}

func validateAppAddName(val interface{}) error {
	return util.RegexValidator(config.ValidNameRegex, "must start with a letter and consist only of letters, numbers, underscore or hyphens")(val)
}

func validateAppAddURL(val interface{}) error {
	return util.RegexValidator(validURLRegex, "invalid URL, example example.com/some_path/run or using vars: ${var.base_url}/some_path/run")(val)
}

func validateAppAddOutputPath(cfg *config.Project) func(val interface{}) error {
	return func(val interface{}) error {
		if s, ok := val.(string); ok && !fileutil.IsRelativeSubdir(cfg.Path, s) {
			return fmt.Errorf("output path must be somewhere in current project config location tree")
		}

		return nil
	}
}

func (d *AppAdd) promptBasic() error {
	var qs []*survey.Question

	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Name of application:"},
			Validate: validateAppAddName,
		})
	} else {
		err := validateAppAddName(d.opts.Name)
		if err != nil {
			return err
		}

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

		if len(d.cfg.DNS) > 0 {
			defaultURL = d.cfg.DNS[0].Domain
		}

		qs = append(qs, &survey.Question{
			Name:     "url",
			Prompt:   &survey.Input{Message: "URL of application:", Default: defaultURL},
			Validate: validateAppAddURL,
		})
	} else {
		err := validateAppAddURL(d.opts.URL)
		if err != nil {
			return err
		}

		d.opts.URL = strings.ToLower(d.opts.URL)
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("URL of application:"), pterm.Cyan(d.opts.URL))
	}

	// Get basic info about app.
	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			return errAppAddCanceled
		}
	}

	err := d.opts.Validate()
	if err != nil {
		return err
	}

	qs = []*survey.Question{}

	// Get output path.
	validateOutputPath := validateAppAddOutputPath(d.cfg)

	if d.opts.OutputPath == "" {
		defaultOutputPath := filepath.Join(d.cfg.Path, d.opts.Type, d.opts.Name)

		qs = append(qs, &survey.Question{
			Name:     "outputpath",
			Prompt:   &survey.Input{Message: "Path to save application YAML:", Default: defaultOutputPath},
			Validate: validateOutputPath,
		})
	} else {
		d.opts.OutputPath, _ = filepath.Abs(d.opts.OutputPath)

		err := validateOutputPath(d.opts.OutputPath)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Path to save application YAML:"), pterm.Cyan(d.opts.OutputPath))
	}

	err = survey.Ask(qs, d.opts)
	if err != nil {
		return errAppAddCanceled
	}

	return nil
}

func (d *AppAdd) prompt(curDir string) (interface{}, error) {
	err := d.promptBasic()
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(d.opts.OutputPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(d.opts.OutputPath, 0755)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if stat != nil && !stat.IsDir() {
		return nil, fmt.Errorf("output path '%s' is not a directory", d.opts.OutputPath)
	}

	if !d.opts.Overwrite && fileutil.FindYAML(filepath.Join(d.opts.OutputPath, config.AppYAMLName)) != "" {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Application config already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			return nil, errAppAddCanceled
		}
	}

	switch d.opts.Type {
	case config.TypeStatic:
		return d.promptStatic(curDir, d.opts)
	default:
		return nil, fmt.Errorf("unsupported app type (WIP)")
	}
}

func validateAppStaticBuildDir(cfg *config.Project, opts *AppAddOptions) func(val interface{}) error {
	return func(val interface{}) error {
		str, ok := val.(string)
		if !ok {
			return nil
		}

		if !fileutil.IsRelativeSubdir(cfg.Path, str) {
			return fmt.Errorf("build dir path must be somewhere in current project config location tree")
		}

		if fileutil.IsRelativeSubdir(str, opts.OutputPath) {
			return fmt.Errorf("build dir path cannot be a parent of output path")
		}

		return nil
	}
}

func suggestAppStaticBuildDir(cfg *config.Project, opts *AppAddOptions) func(toComplete string) []string {
	return func(toComplete string) []string {
		var dirs []string

		_ = filepath.WalkDir(cfg.Path, func(path string, entry fs.DirEntry, err error) error {
			if !entry.IsDir() {
				return nil
			}

			if strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}

			if !fileutil.IsRelativeSubdir(cfg.Path, path) || fileutil.IsRelativeSubdir(path, opts.OutputPath) {
				return nil
			}

			dirs = append(dirs, path)
			return nil
		})

		return dirs
	}
}

func (d *AppAdd) promptStatic(curDir string, opts *AppAddOptions) (*staticAppInfo, error) {
	var qs []*survey.Question

	buildDirValidator := validateAppStaticBuildDir(d.cfg, opts)

	if opts.Static.BuildDir == "" {
		def, _ := filepath.Rel(curDir, filepath.Join(opts.OutputPath, config.DefaultStaticAppBuildDir))

		qs = append(qs, &survey.Question{
			Name: "builddir",
			Prompt: &survey.Input{
				Message: "Build directory of application:",
				Default: "./" + def,
				Suggest: suggestAppStaticBuildDir(d.cfg, opts),
			},
			Validate: buildDirValidator,
		})
	} else {
		err := buildDirValidator(opts.Static.BuildDir)
		if err != nil {
			return nil, err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build directory of application:"), pterm.Cyan(opts.Static.BuildDir))
	}

	if opts.Static.BuildCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "buildcommand",
			Prompt: &survey.Input{Message: "Build command of application (optional, e.g. yarn build):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build command of application:"), pterm.Cyan(opts.Static.BuildCommand))
	}

	if opts.Static.DevCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "devcommand",
			Prompt: &survey.Input{Message: "Dev command of application (optional, e.g. yarn dev):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Dev command of application:"), pterm.Cyan(opts.Static.DevCommand))
	}

	if opts.Static.Routing == "" {
		qs = append(qs, &survey.Question{
			Name: "routing",
			Prompt: &survey.Select{
				Message: "Routing of application:",
				Options: config.StaticAppRoutings,
				Default: config.StaticAppRoutingReact,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Routing of application:"), pterm.Cyan(opts.Static.BuildCommand))
	}

	// Get info about static app.
	if len(qs) != 0 {
		err := survey.Ask(qs, &opts.Static)
		if err != nil {
			return nil, errAppAddCanceled
		}
	}

	// Cleanup.
	opts.Static.BuildDir, _ = filepath.Rel(opts.OutputPath, opts.Static.BuildDir)
	opts.Static.BuildDir = "./" + opts.Static.BuildDir

	return &staticAppInfo{
		App: config.StaticApp{
			BasicApp: config.BasicApp{
				AppName: opts.Name,
				AppURL:  opts.URL,
				AppPath: opts.OutputPath,
			},
			Build: &config.StaticAppBuild{
				Command: opts.Static.BuildCommand,
				Dir:     opts.Static.BuildDir,
			},
			Dev: &config.StaticAppDev{
				Command: opts.Static.DevCommand,
			},
			Routing: opts.Static.Routing,
		},

		URL:  opts.URL,
		Type: opts.Type,
	}, nil
}
