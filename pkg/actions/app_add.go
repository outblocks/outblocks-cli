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
	"github.com/AlecAivazis/survey/v2/terminal"
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

type AppAddOptions struct {
	Overwrite bool

	OutputPath string
	Name       string
	Dir        string
	Type       string
	URL        string
	RunCommand string

	// Static App Options.
	StaticBuildCommand string
	StaticBuildDir     string
	StaticRouting      string
}

func (o *AppAddOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Type, validation.Required, validation.In(util.InterfaceSlice(config.ValidAppTypes)...)),
		validation.Field(&o.StaticRouting, validation.In(util.InterfaceSlice(config.StaticAppRoutings)...)),
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
		tmpl = templates.StaticAppYAMLTemplate()
		path = app.App.AppDir
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
		if s, ok := val.(string); ok && !fileutil.IsRelativeSubdir(cfg.Dir, s) {
			return fmt.Errorf("output path must be somewhere in current project config location tree")
		}

		return nil
	}
}

func validateAppAddDir(cfg *config.Project) func(val interface{}) error {
	return func(val interface{}) error {
		if s, ok := val.(string); ok && !fileutil.IsRelativeSubdir(cfg.Dir, s) {
			return fmt.Errorf("application dir must be somewhere in current project config location tree")
		}

		return nil
	}
}

func (d *AppAdd) promptBasic() error {
	var qs []*survey.Question

	// 1st pass - get app name and type.
	if d.opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Application name:"},
			Validate: validateAppAddName,
		})
	} else {
		err := validateAppAddName(d.opts.Name)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Application name:"), pterm.Cyan(d.opts.Name))
	}

	if d.opts.Type == "" {
		qs = append(qs, &survey.Question{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Application type:",
				Options: config.ValidAppTypes,
				Default: config.AppTypeStatic,
			},
		})
	} else {
		d.opts.Type = strings.ToLower(d.opts.Type)
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Application type:"), pterm.Cyan(d.opts.Type))
	}

	// Get basic info about app.
	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return errAppAddCanceled
			}

			return err
		}
	}

	err := d.opts.Validate()
	if err != nil {
		return err
	}

	// 2nd pass - get app path.
	qs = []*survey.Question{}

	validateAppDir := validateAppAddDir(d.cfg)

	if d.opts.Dir == "" {
		defaultDir := filepath.Join(d.cfg.Dir, d.opts.Type, d.opts.Name)

		qs = append(qs, &survey.Question{
			Name:     "dir",
			Prompt:   &survey.Input{Message: "Application path:", Default: defaultDir},
			Validate: validateAppDir,
		})
	} else {
		d.opts.Dir, _ = filepath.Abs(d.opts.Dir)

		err := validateAppDir(d.opts.Dir)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Application path:"), pterm.Cyan(d.opts.Dir))
	}

	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return errAppAddCanceled
			}

			return err
		}
	}

	// 3rd pass - get output path and app URL.
	qs = []*survey.Question{}

	// Get output path.
	validateOutputPath := validateAppAddOutputPath(d.cfg)

	if d.opts.OutputPath == "" {
		qs = append(qs, &survey.Question{
			Name:     "outputpath",
			Prompt:   &survey.Input{Message: "Path to save application YAML:", Default: d.opts.Dir},
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

	// Get app URL.
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

	err = survey.Ask(qs, d.opts)
	if err == terminal.InterruptErr {
		return errAppAddCanceled
	}

	return err
}

func (d *AppAdd) prompt(curDir string) (interface{}, error) {
	err := d.promptBasic()
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(d.opts.OutputPath)
	if os.IsNotExist(err) {
		err = plugin_util.MkdirAll(d.opts.OutputPath, 0755)
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
	case config.AppTypeStatic:
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

		if !fileutil.IsRelativeSubdir(cfg.Dir, str) {
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

		_ = filepath.WalkDir(opts.Dir, func(path string, entry fs.DirEntry, err error) error {
			if !entry.IsDir() {
				return nil
			}

			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
				return fs.SkipDir
			}

			if !fileutil.IsRelativeSubdir(cfg.Dir, path) || fileutil.IsRelativeSubdir(path, opts.OutputPath) {
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

	staticBuildDirValidator := validateAppStaticBuildDir(d.cfg, opts)

	if opts.StaticBuildDir == "" {
		def := filepath.Join(curDir, opts.Dir, config.DefaultStaticAppBuildDir)

		qs = append(qs, &survey.Question{
			Name: "StaticBuildDir",
			Prompt: &survey.Input{
				Message: "Build directory of application:",
				Default: def,
				Suggest: suggestAppStaticBuildDir(d.cfg, opts),
			},
			Validate: staticBuildDirValidator,
		})
	} else {
		err := staticBuildDirValidator(opts.StaticBuildDir)
		if err != nil {
			return nil, err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build directory of application:"), pterm.Cyan(opts.StaticBuildDir))
	}

	if opts.StaticBuildCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "StaticBuildCommand",
			Prompt: &survey.Input{Message: "Build command of application (optional, e.g. yarn build):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build command of application:"), pterm.Cyan(opts.StaticBuildCommand))
	}

	if opts.RunCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "RunCommand",
			Prompt: &survey.Input{Message: "Run command of application to serve app during dev (optional, e.g. yarn dev):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Run command of application:"), pterm.Cyan(opts.RunCommand))
	}

	if opts.StaticRouting == "" {
		qs = append(qs, &survey.Question{
			Name: "StaticRouting",
			Prompt: &survey.Select{
				Message: "Routing of application:",
				Options: config.StaticAppRoutings,
				Default: config.StaticAppRoutingReact,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Routing of application:"), pterm.Cyan(opts.StaticBuildCommand))
	}

	// Get info about static app.
	if len(qs) != 0 {
		err := survey.Ask(qs, opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, errAppAddCanceled
			}

			return nil, err
		}
	}

	// Cleanup.
	opts.StaticBuildDir, _ = filepath.Rel(opts.Dir, opts.StaticBuildDir)
	opts.StaticBuildDir = "./" + opts.StaticBuildDir

	return &staticAppInfo{
		App: config.StaticApp{
			BasicApp: config.BasicApp{
				AppName: opts.Name,
				AppURL:  opts.URL,
				AppDir:  opts.Dir,
				AppRun: &config.AppRun{
					Command: opts.RunCommand,
				},
			},
			Build: &config.StaticAppBuild{
				Command: opts.StaticBuildCommand,
				Dir:     opts.StaticBuildDir,
			},
			Routing: opts.StaticRouting,
		},

		URL:  opts.URL,
		Type: opts.Type,
	}, nil
}
