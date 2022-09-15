package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/pterm/pterm"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Logs struct {
	log  logger.Logger
	cfg  *config.Project
	opts *LogsOptions
}

type LogsOptions struct {
	Start                 time.Time
	End                   time.Time
	Targets               *util.TargetMatcher
	Contains, NotContains []string
	Filter                string
	Severity              apiv1.LogSeverity
	OnlyApps              bool
	Follow                bool
}

func NewLogs(cfg *config.Project, opts *LogsOptions) *Logs {
	return &Logs{
		log:  logger.NewLogger(),
		cfg:  cfg,
		opts: opts,
	}
}

func (l *Logs) logCallback(idMap map[string]string) func(lr *apiv1.LogsResponse) {
	normalizedMap := make(map[string]string, len(idMap))

	mLen := 0
	for _, v := range idMap {
		if mLen < len(v) {
			mLen = len(v)
		}
	}

	justify := fmt.Sprintf("%%-%ds", mLen)

	for k, v := range idMap {
		normalizedMap[k] = fmt.Sprintf(justify, v)
	}

	idMap = normalizedMap

	return func(lr *apiv1.LogsResponse) {
		logFunc := l.log.Infof

		switch lr.Severity {
		case apiv1.LogSeverity_LOG_SEVERITY_DEBUG:
			logFunc = l.log.Debugf
		case apiv1.LogSeverity_LOG_SEVERITY_NOTICE:
			logFunc = l.log.Noticef
		case apiv1.LogSeverity_LOG_SEVERITY_INFO, apiv1.LogSeverity_LOG_SEVERITY_UNSPECIFIED:
			logFunc = l.log.Infof
		case apiv1.LogSeverity_LOG_SEVERITY_WARN:
			logFunc = l.log.Warnf
		case apiv1.LogSeverity_LOG_SEVERITY_ERROR:
			logFunc = l.log.Errorf
		}

		parts := []string{pterm.Green(lr.Time.AsTime().Local().Format(time.StampMilli))}

		if len(idMap) > 1 {
			parts = append(parts, pterm.Yellow(idMap[lr.Source]))
		}

		if lr.Type == apiv1.LogsResponse_TYPE_STDERR {
			parts = append(parts, pterm.Red("stderr"))
		}

		if lr.Http != nil {
			httpStatusStr := strconv.Itoa(int(lr.Http.Status))
			if lr.Http.Status >= 400 {
				httpStatusStr = pterm.Style{pterm.Bold, pterm.FgRed, pterm.Underscore}.Sprint(httpStatusStr)
			}

			parts = append(parts, fmt.Sprintf("%s - %s %s %s %q %q", lr.Http.RemoteIp, lr.Http.RequestMethod, lr.Http.RequestUrl, httpStatusStr, lr.Http.Referer, lr.Http.UserAgent))
		}

		switch p := lr.Payload.(type) {
		case *apiv1.LogsResponse_Text:
			parts = append(parts, p.Text)
		case *apiv1.LogsResponse_Json:
			var fields []string

			for k, v := range p.Json.AsMap() {
				vv, _ := json.Marshal(v)
				fields = append(fields, fmt.Sprintf("%s=%s", k, vv))
			}

			sort.Strings(fields)

			parts = append(parts, fields...)
		}

		logFunc("%s\n", strings.Join(parts, " "))
	}
}

func (l *Logs) Run(ctx context.Context) error {
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   l.cfg.YAMLData(),
	}

	// Get state.
	state, _, err := getState(ctx, l.cfg.State, false, 0, true, yamlContext)
	if err != nil {
		return err
	}

	if len(state.Apps) == 0 && len(state.Dependencies) == 0 {
		return merry.Errorf("no apps and/or deployments found in state")
	}

	var (
		apps []*apiv1.App
		deps []*apiv1.Dependency
	)

	idMap := make(map[string]string)

	for _, app := range state.Apps {
		if l.opts.Targets.IsEmpty() || l.opts.Targets.Matches(app.App.Id) {
			apps = append(apps, app.App)
			idMap[app.App.Id] = fmt.Sprintf("APP:%s:%s", app.App.Type, app.App.Name)
		}
	}

	if !l.opts.OnlyApps {
		for _, dep := range state.Dependencies {
			if l.opts.Targets.IsEmpty() || l.opts.Targets.Matches(dep.Dependency.Id) {
				deps = append(deps, dep.Dependency)
				idMap[dep.Dependency.Id] = fmt.Sprintf("DEP:%s", dep.Dependency.Name)
			}
		}
	}

	for _, t := range l.opts.Targets.Unmatched() {
		return merry.Errorf("unknown target specified: '%s' is missing definition", t.Input())
	}

	// TODO: support multi-deployment plugins
	var deployPluginName string
	if len(apps) != 0 {
		deployPluginName = apps[0].DeployPlugin
	} else {
		deployPluginName = deps[0].DeployPlugin
	}

	req := &apiv1.LogsRequest{
		Apps:         apps,
		Dependencies: deps,
		State:        state.Plugins[deployPluginName].Proto(),
		Start:        timestamppb.New(l.opts.Start),
		End:          timestamppb.New(l.opts.End),
		Severity:     l.opts.Severity,
		Contains:     l.opts.Contains,
		NotContains:  l.opts.NotContains,
		Filter:       l.opts.Filter,
		Follow:       l.opts.Follow,
	}

	deployPlugin := l.cfg.FindLoadedPlugin(deployPluginName)
	if deployPlugin == nil {
		return merry.Errorf("deploy plugin '%s' not found", deployPluginName)
	}

	return deployPlugin.Client().Logs(ctx, req, l.logCallback(idMap))
}
