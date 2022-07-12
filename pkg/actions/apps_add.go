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
	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/templates"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/outblocks/outblocks-plugin-go/util/command"
	"github.com/pterm/pterm"
)

var (
	errAppAddCanceled = errors.New("adding app canceled")
	validURLRegex     = regexp.MustCompile(`^(https?://)?[a-zA-Z0-9{}\-_.]+$`)
)

type staticAppInfo struct {
	App config.StaticApp
}

type serviceAppInfo struct {
	App config.ServiceApp
}

type functionAppInfo struct {
	App config.FunctionApp
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

	// Function App Options.
	FunctionRuntime    string
	FunctionEntrypoint string
}

func (o *AppAddOptions) Validate() error {
	return validation.ValidateStruct(o,
		validation.Field(&o.Type, validation.Required, validation.In(util.InterfaceSlice(config.ValidAppTypes)...)),
		validation.Field(&o.StaticRouting, validation.In(util.InterfaceSlice(config.StaticAppRoutings)...)),
	)
}

func (m *AppManager) Add(ctx context.Context, opts *AppAddOptions) error {
	_, err := os.Getwd()
	if err != nil {
		return merry.Errorf("can't get current working dir: %w", err)
	}

	appInfo, err := m.promptAdd(opts)
	if errors.Is(err, errAppAddCanceled) {
		m.log.Println("Adding application canceled.")
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
	case *functionAppInfo:
		tmpl = templates.FunctionAppYAMLTemplate()
		path = app.App.AppDir
	default:
		return merry.Errorf("unsupported app type")
	}

	var appYAML bytes.Buffer

	err = tmpl.Execute(&appYAML, appInfo)
	if err != nil {
		return err
	}

	destFile := filepath.Join(path, config.AppYAMLName+".yaml")

	err = fileutil.WriteFile(destFile, appYAML.Bytes(), 0o644)
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
			return merry.Errorf("output dir must be somewhere in current project config location tree")
		}

		return nil
	}
}

func validateAppAddDir(cfg *config.Project) func(val interface{}) error {
	return func(val interface{}) error {
		if s, ok := val.(string); ok && !fileutil.IsRelativeSubdir(cfg.Dir, s) {
			return merry.Errorf("application dir must be somewhere in current project config location tree")
		}

		return nil
	}
}

func (m *AppManager) promptAddBasic(opts *AppAddOptions) error { // nolint: gocyclo
	var qs []*survey.Question

	// 1st pass - get app name and type.
	if opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Application name:"},
			Validate: validateAppAddName,
		})
	} else {
		err := validateAppAddName(opts.Name)
		if err != nil {
			return err
		}

		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Application name:"), pterm.Cyan(opts.Name))
	}

	if opts.Type == "" {
		qs = append(qs, &survey.Question{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Application type:",
				Options: config.ValidAppTypes,
				Default: config.AppTypeStatic,
			},
		})
	} else {
		opts.Type = strings.ToLower(opts.Type)
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Application type:"), pterm.Cyan(opts.Type))
	}

	// Get basic info about app.
	if len(qs) != 0 {
		err := survey.Ask(qs, opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return errAppAddCanceled
			}

			return err
		}
	}

	err := opts.Validate()
	if err != nil {
		return err
	}

	// 2nd pass - get app dir.
	qs = []*survey.Question{}

	validateAppDir := validateAppAddDir(m.cfg)

	if opts.Dir == "" {
		defaultDir := filepath.Join(m.cfg.Dir, opts.Type, opts.Name)

		qs = append(qs, &survey.Question{
			Name:     "dir",
			Prompt:   &survey.Input{Message: "Application dir:", Default: defaultDir},
			Validate: validateAppDir,
		})
	} else {
		opts.Dir, _ = filepath.Abs(opts.Dir)

		err := validateAppDir(opts.Dir)
		if err != nil {
			return err
		}

		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Application dir:"), pterm.Cyan(opts.Dir))
	}

	if len(qs) != 0 {
		err := survey.Ask(qs, opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return errAppAddCanceled
			}

			return err
		}
	}

	opts.Dir, _ = filepath.Abs(opts.Dir)

	// 3rd pass - get output dir, plugin info, URL.
	qs = []*survey.Question{}

	// Get output dir.
	validateOutputDir := validateAppAddOutputDir(m.cfg)

	if opts.OutputDir == "" {
		qs = append(qs, &survey.Question{
			Name:     "outputdir",
			Prompt:   &survey.Input{Message: "Dir to save application YAML:", Default: opts.Dir},
			Validate: validateOutputDir,
		})
	} else {
		opts.OutputDir, _ = filepath.Abs(opts.OutputDir)

		err := validateOutputDir(opts.OutputDir)
		if err != nil {
			return err
		}

		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Dir to save application YAML:"), pterm.Cyan(opts.OutputDir))
	}

	// Get app URL.
	if opts.URL == "" {
		defaultURL := ""

		if len(m.cfg.DNS) > 0 {
			defaultURL = m.cfg.DNS[0].Domain
		}

		qs = append(qs, &survey.Question{
			Name:     "url",
			Prompt:   &survey.Input{Message: "URL of application:", Default: defaultURL},
			Validate: validateAppAddURL,
		})
	} else {
		err := validateAppAddURL(opts.URL)
		if err != nil {
			return err
		}

		opts.URL = strings.ToLower(opts.URL)
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("URL of application:"), pterm.Cyan(opts.URL))
	}

	// Run info and plugins.
	if opts.RunCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "RunCommand",
			Prompt: &survey.Input{Message: "Run command of application to serve app during dev (optional, e.g. yarn dev):"},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Run command of application:"), pterm.Cyan(opts.RunCommand))
	}

	if opts.RunPlugin == "" {
		pluginOpts := []string{
			config.RunPluginDirect,
		}

		for _, p := range m.cfg.Plugins {
			if p.Loaded().HasAction(plugins.ActionRun) && p.Loaded().SupportsApp(opts.Type) {
				pluginOpts = append(pluginOpts, p.Name)
			}
		}

		qs = append(qs, &survey.Question{
			Name: "RunPlugin",
			Prompt: &survey.Select{
				Message: "Run plugin used for application:",
				Options: pluginOpts,
			},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Run plugin used for application:"), pterm.Cyan(opts.RunPlugin))
	}

	if opts.DeployPlugin == "" {
		pluginOpts := []string{}

		for _, p := range m.cfg.Plugins {
			if p.Loaded().HasAction(plugins.ActionDeploy) && p.Loaded().SupportsApp(opts.Type) {
				pluginOpts = append(pluginOpts, p.Name)
			}
		}

		qs = append(qs, &survey.Question{
			Name: "DeployPlugin",
			Prompt: &survey.Select{
				Message: "Deploy plugin used for application:",
				Options: pluginOpts,
			},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Deploy plugin used for application:"), pterm.Cyan(opts.DeployPlugin))
	}

	err = survey.Ask(qs, opts)
	if err == terminal.InterruptErr {
		return errAppAddCanceled
	}

	opts.OutputDir, _ = filepath.Abs(opts.OutputDir)

	// Cleanup.
	if filepath.IsAbs(opts.Dir) {
		opts.Dir, _ = filepath.Rel(m.cfg.Dir, opts.Dir)
		opts.Dir = "./" + opts.Dir
	}

	return err
}

func (m *AppManager) promptAdd(opts *AppAddOptions) (interface{}, error) {
	err := m.promptAddBasic(opts)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(opts.OutputDir)
	if os.IsNotExist(err) {
		err = fileutil.MkdirAll(opts.OutputDir, 0o755)
		if err != nil {
			return nil, merry.Errorf("failed to create dir %s: %w", opts.OutputDir, err)
		}
	}

	if err != nil {
		return nil, err
	}

	if stat != nil && !stat.IsDir() {
		return nil, merry.Errorf("output dir '%s' is not a directory", opts.OutputDir)
	}

	if !opts.Overwrite && fileutil.FindYAML(filepath.Join(opts.OutputDir, config.AppYAMLName)) != "" {
		proceed := false
		prompt := &survey.Confirm{
			Message: "Application config already exists! Do you want to overwrite it?",
		}

		_ = survey.AskOne(prompt, &proceed)

		if !proceed {
			return nil, errAppAddCanceled
		}
	}

	switch opts.Type {
	case config.AppTypeStatic:
		return m.promptAddStatic(opts)
	case config.AppTypeService:
		return m.promptAddService(opts)
	case config.AppTypeFunction:
		return m.promptAddFunction(opts)
	default:
		return nil, merry.Errorf("unsupported app type")
	}
}

func validateAppStaticBuildDir(cfg *config.Project, opts *AppAddOptions) func(val interface{}) error {
	return func(val interface{}) error {
		str, ok := val.(string)
		if !ok {
			return nil
		}

		if !fileutil.IsRelativeSubdir(cfg.Dir, str) {
			return merry.Errorf("build dir must be somewhere in current project config location tree")
		}

		fmt.Println(str, "--", opts.OutputDir, "--", fileutil.IsRelativeSubdir(str, opts.OutputDir))

		if fileutil.IsRelativeSubdir(str, opts.OutputDir) {
			return merry.Errorf("build dir cannot be a parent of output dir")
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

func (m *AppManager) promptAddStatic(opts *AppAddOptions) (*staticAppInfo, error) {
	var qs []*survey.Question

	staticBuildDirValidator := validateAppStaticBuildDir(m.cfg, opts)
	curDir, _ := os.Getwd()

	if opts.StaticBuildDir == "" {
		def := filepath.Join(curDir, opts.Dir, config.DefaultStaticAppBuildDir)

		qs = append(qs, &survey.Question{
			Name: "StaticBuildDir",
			Prompt: &survey.Input{
				Message: "Build directory of application:",
				Default: def,
				Suggest: suggestAppStaticBuildDir(m.cfg, opts),
			},
			Validate: staticBuildDirValidator,
		})
	} else {
		err := staticBuildDirValidator(opts.StaticBuildDir)
		if err != nil {
			return nil, err
		}

		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Build directory of application:"), pterm.Cyan(opts.StaticBuildDir))
	}

	if opts.StaticBuildCommand == "" {
		qs = append(qs, &survey.Question{
			Name:   "StaticBuildCommand",
			Prompt: &survey.Input{Message: "Build command of application (optional, e.g. yarn build):"},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Build command of application:"), pterm.Cyan(opts.StaticBuildCommand))
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
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Routing of application:"), pterm.Cyan(opts.StaticBuildCommand))
	}

	// Ask questions about static app.
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
	if filepath.IsAbs(opts.StaticBuildDir) {
		opts.StaticBuildDir, _ = filepath.Rel(filepath.Join(m.cfg.Dir, opts.Dir), opts.StaticBuildDir)
		opts.StaticBuildDir = "./" + opts.StaticBuildDir
	}

	return &staticAppInfo{
		App: config.StaticApp{
			BasicApp: config.BasicApp{
				AppName: opts.Name,
				AppType: config.AppTypeStatic,
				AppURL:  opts.URL,
				AppDir:  opts.Dir,
				AppDeploy: &config.AppDeployInfo{
					Plugin: opts.DeployPlugin,
				},
				AppRun: &config.AppRunInfo{
					Plugin:  opts.RunPlugin,
					Command: command.NewStringCommandFromString(opts.RunCommand),
				},
			},
			StaticAppProperties: types.StaticAppProperties{
				Build: &types.StaticAppBuild{
					Command: command.NewStringCommandFromString(opts.StaticBuildCommand),
					Dir:     opts.StaticBuildDir,
				},
				Routing: opts.StaticRouting,
			},
		},
	}, nil
}

func (m *AppManager) promptAddService(opts *AppAddOptions) (*serviceAppInfo, error) { // nolint: unparam
	return &serviceAppInfo{
		App: config.ServiceApp{
			BasicApp: config.BasicApp{
				AppName: opts.Name,
				AppType: config.AppTypeService,
				AppURL:  opts.URL,
				AppDir:  opts.Dir,
				AppRun: &config.AppRunInfo{
					Command: command.NewStringCommandFromString(opts.RunCommand),
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

func (m *AppManager) promptAddFunction(opts *AppAddOptions) (*functionAppInfo, error) { // nolint: unparam
	var qs []*survey.Question

	if opts.FunctionRuntime == "" {
		qs = append(qs, &survey.Question{
			Name:   "FunctionRuntime",
			Prompt: &survey.Input{Message: "Function runtime (the runtime in which the function is going to run):"},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Function runtime:"), pterm.Cyan(opts.FunctionRuntime))
	}

	if opts.FunctionEntrypoint == "" {
		qs = append(qs, &survey.Question{
			Name: "FunctionEntrypoint",
			Prompt: &survey.Input{
				Message: "Function entrypoint (name of the function that will be executed when the function is triggered):",
				Default: opts.Name,
			},
		})
	} else {
		m.log.Printf("%s %s\n", pterm.Bold.Sprint("Function entrypoint:"), pterm.Cyan(opts.FunctionEntrypoint))
	}

	if len(qs) != 0 {
		err := survey.Ask(qs, opts)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, errAppAddCanceled
			}

			return nil, err
		}
	}

	return &functionAppInfo{
		App: config.FunctionApp{
			BasicApp: config.BasicApp{
				AppName: opts.Name,
				AppType: config.AppTypeService,
				AppURL:  opts.URL,
				AppDir:  opts.Dir,
				AppRun: &config.AppRunInfo{
					Command: command.NewStringCommandFromString(opts.RunCommand),
				},
			},
			FunctionAppProperties: types.FunctionAppProperties{
				Runtime:    opts.FunctionRuntime,
				Entrypoint: opts.FunctionEntrypoint,
			},
		},
	}, nil
}
