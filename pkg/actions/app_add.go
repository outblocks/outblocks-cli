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
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/templates"
	"github.com/outblocks/outblocks-plugin-go/types"
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
	App config.StaticApp
}

type serviceAppInfo struct {
	App config.ServiceApp
}

type AppAddOptions struct {
	Overwrite bool

	OutputDir    string
	Name         string
	Dir          string
	Type         string
	URL          string
	RunCommand   string
	RunPlugin    string
	DeployPlugin string

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
	_, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("can't get current working dir: %w", err)
	}

	appInfo, err := d.prompt()
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
	case *serviceAppInfo:
		tmpl = templates.ServiceAppYAMLTemplate()
		path = app.App.AppDir
		// TODO: add templates for function app
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

func validateAppAddOutputDir(cfg *config.Project) func(val interface{}) error {
	return func(val interface{}) error {
		if s, ok := val.(string); ok && !fileutil.IsRelativeSubdir(cfg.Dir, s) {
			return fmt.Errorf("output dir must be somewhere in current project config location tree")
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

func (d *AppAdd) promptBasic() error { // nolint: gocyclo
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

	// 2nd pass - get app dir.
	qs = []*survey.Question{}

	validateAppDir := validateAppAddDir(d.cfg)

	if d.opts.Dir == "" {
		defaultDir := filepath.Join(d.cfg.Dir, d.opts.Type, d.opts.Name)

		qs = append(qs, &survey.Question{
			Name:     "dir",
			Prompt:   &survey.Input{Message: "Application dir:", Default: defaultDir},
			Validate: validateAppDir,
		})
	} else {
		d.opts.Dir, _ = filepath.Abs(d.opts.Dir)

		err := validateAppDir(d.opts.Dir)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Application dir:"), pterm.Cyan(d.opts.Dir))
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

	// 3rd pass - get output dir, plugin info, URL.
	qs = []*survey.Question{}

	// Get output dir.
	validateOutputDir := validateAppAddOutputDir(d.cfg)

	if d.opts.OutputDir == "" {
		qs = append(qs, &survey.Question{
			Name:     "outputdir",
			Prompt:   &survey.Input{Message: "Dir to save application YAML:", Default: d.opts.Dir},
			Validate: validateOutputDir,
		})
	} else {
		d.opts.OutputDir, _ = filepath.Abs(d.opts.OutputDir)

		err := validateOutputDir(d.opts.OutputDir)
		if err != nil {
			return err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Dir to save application YAML:"), pterm.Cyan(d.opts.OutputDir))
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

	// Run info and plugins.
	if d.opts.RunCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "RunCommand",
			Prompt: &survey.Input{Message: "Run command of application to serve app during dev (optional, e.g. yarn dev):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Run command of application:"), pterm.Cyan(d.opts.RunCommand))
	}

	if d.opts.RunPlugin == "" {
		opts := []string{
			config.RunPluginDirect,
		}

		for _, p := range d.cfg.Plugins {
			if p.Loaded().HasAction(plugins.ActionRun) && p.Loaded().SupportsApp(d.opts.Type) {
				opts = append(opts, p.Name)
			}
		}

		qs = append(qs, &survey.Question{
			Name: "RunPlugin",
			Prompt: &survey.Select{
				Message: "Run plugin used for application:",
				Options: opts,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Run plugin used for application:"), pterm.Cyan(d.opts.RunPlugin))
	}

	if d.opts.DeployPlugin == "" {
		opts := []string{}

		for _, p := range d.cfg.Plugins {
			if p.Loaded().HasAction(plugins.ActionDeploy) && p.Loaded().SupportsApp(d.opts.Type) {
				opts = append(opts, p.Name)
			}
		}

		qs = append(qs, &survey.Question{
			Name: "DeployPlugin",
			Prompt: &survey.Select{
				Message: "Deploy plugin used for application:",
				Options: opts,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Deploy plugin used for application:"), pterm.Cyan(d.opts.DeployPlugin))
	}

	err = survey.Ask(qs, d.opts)
	if err == terminal.InterruptErr {
		return errAppAddCanceled
	}

	// Cleanup.
	if filepath.IsAbs(d.opts.Dir) {
		d.opts.Dir, _ = filepath.Rel(d.cfg.Dir, d.opts.Dir)
		d.opts.Dir = "./" + d.opts.Dir
	}

	return err
}

func (d *AppAdd) prompt() (interface{}, error) {
	err := d.promptBasic()
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(d.opts.OutputDir)
	if os.IsNotExist(err) {
		err = plugin_util.MkdirAll(d.opts.OutputDir, 0755)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if stat != nil && !stat.IsDir() {
		return nil, fmt.Errorf("output dir '%s' is not a directory", d.opts.OutputDir)
	}

	if !d.opts.Overwrite && fileutil.FindYAML(filepath.Join(d.opts.OutputDir, config.AppYAMLName)) != "" {
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
		return d.promptStatic()
	case config.AppTypeService:
		return d.promptService()
		// TODO: add adding app function
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
			return fmt.Errorf("build dir must be somewhere in current project config location tree")
		}

		if fileutil.IsRelativeSubdir(str, opts.OutputDir) {
			return fmt.Errorf("build dir cannot be a parent of output dir")
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

			if !fileutil.IsRelativeSubdir(cfg.Dir, path) || fileutil.IsRelativeSubdir(path, opts.OutputDir) {
				return nil
			}

			dirs = append(dirs, path)
			return nil
		})

		return dirs
	}
}

func (d *AppAdd) promptStatic() (*staticAppInfo, error) {
	var qs []*survey.Question

	staticBuildDirValidator := validateAppStaticBuildDir(d.cfg, d.opts)
	curDir, _ := os.Getwd()

	if d.opts.StaticBuildDir == "" {
		def := filepath.Join(curDir, d.opts.Dir, config.DefaultStaticAppBuildDir)

		qs = append(qs, &survey.Question{
			Name: "StaticBuildDir",
			Prompt: &survey.Input{
				Message: "Build directory of application:",
				Default: def,
				Suggest: suggestAppStaticBuildDir(d.cfg, d.opts),
			},
			Validate: staticBuildDirValidator,
		})
	} else {
		err := staticBuildDirValidator(d.opts.StaticBuildDir)
		if err != nil {
			return nil, err
		}

		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build directory of application:"), pterm.Cyan(d.opts.StaticBuildDir))
	}

	if d.opts.StaticBuildCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "StaticBuildCommand",
			Prompt: &survey.Input{Message: "Build command of application (optional, e.g. yarn build):"},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Build command of application:"), pterm.Cyan(d.opts.StaticBuildCommand))
	}

	if d.opts.StaticRouting == "" {
		qs = append(qs, &survey.Question{
			Name: "StaticRouting",
			Prompt: &survey.Select{
				Message: "Routing of application:",
				Options: config.StaticAppRoutings,
				Default: config.StaticAppRoutingReact,
			},
		})
	} else {
		d.log.Printf("%s %s\n", pterm.Bold.Sprint("Routing of application:"), pterm.Cyan(d.opts.StaticBuildCommand))
	}

	// Ask questions about static app.
	if len(qs) != 0 {
		err := survey.Ask(qs, d.opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, errAppAddCanceled
			}

			return nil, err
		}
	}

	// Cleanup.
	if filepath.IsAbs(d.opts.StaticBuildDir) {
		d.opts.StaticBuildDir, _ = filepath.Rel(filepath.Join(d.cfg.Dir, d.opts.Dir), d.opts.StaticBuildDir)
		d.opts.StaticBuildDir = "./" + d.opts.StaticBuildDir
	}

	return &staticAppInfo{
		App: config.StaticApp{
			BasicApp: config.BasicApp{
				AppName: d.opts.Name,
				AppType: config.AppTypeStatic,
				AppURL:  d.opts.URL,
				AppDir:  d.opts.Dir,
				AppDeploy: &config.AppDeploy{
					Plugin: d.opts.DeployPlugin,
				},
				AppRun: &config.AppRun{
					Plugin:  d.opts.RunPlugin,
					Command: d.opts.RunCommand,
				},
			},
			StaticAppProperties: types.StaticAppProperties{
				Build: &types.StaticAppBuild{
					Command: d.opts.StaticBuildCommand,
					Dir:     d.opts.StaticBuildDir,
				},
				Routing: d.opts.StaticRouting,
			},
		},
	}, nil
}

func (d *AppAdd) promptService() (*serviceAppInfo, error) { // nolint: unparam
	return &serviceAppInfo{
		App: config.ServiceApp{
			BasicApp: config.BasicApp{
				AppName: d.opts.Name,
				AppType: config.AppTypeService,
				AppURL:  d.opts.URL,
				AppDir:  d.opts.Dir,
				AppRun: &config.AppRun{
					Command: d.opts.RunCommand,
				},
			},
			ServiceAppProperties: types.ServiceAppProperties{
				Build: &types.ServiceAppBuild{
					Dockerfile:    "Dockerfile",
					DockerContext: ".",
				},
			},
		},
	}, nil
}
