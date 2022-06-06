package cmd

import (
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/araddon/dateparse"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"github.com/outblocks/outblocks-cli/pkg/config"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/spf13/cobra"
)

func parseTimeOrDuration(in string) (time.Time, error) {
	if in == "" {
		return time.Time{}, nil
	}

	dur, err := time.ParseDuration(in)
	if err == nil {
		return time.Now().Add(-dur), nil
	}

	t, err := dateparse.ParseLocal(in)
	if err != nil {
		return time.Time{}, merry.Errorf("invalid time format")
	}

	return t, nil
}

func (e *Executor) newLogsCmd() *cobra.Command {
	opts := &actions.LogsOptions{}

	var (
		target     []string
		severity   string
		start, end string
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream logs",
		Long:  `Filter and stream logs of specific apps and dependencies.`,
		Annotations: map[string]string{
			cmdGroupAnnotation:           cmdGroupMain,
			cmdProjectLoadModeAnnotation: cmdLoadModeEssential,
			cmdAppsLoadModeAnnotation:    cmdLoadModeSkip,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, t := range target {
				tsplit := strings.SplitN(t, ".", 2)
				if len(tsplit) != 2 {
					return merry.Errorf("wrong format for target '%s': specify in a form of <app type>.<name> or dep.<dep name>, e.g.: static.website", t)
				}

				if tsplit[0] == "dep" {
					opts.Dependencies = append(opts.Dependencies, config.ComputeDependencyID(tsplit[1]))
				} else {
					opts.Apps = append(opts.Apps, config.ComputeAppID(tsplit[0], tsplit[1]))
				}
			}

			var level apiv1.LogSeverity

			switch strings.ToLower(severity) {
			case "debug":
				level = apiv1.LogSeverity_LOG_SEVERITY_DEBUG
			case "notice":
				level = apiv1.LogSeverity_LOG_SEVERITY_NOTICE
			case "info":
				level = apiv1.LogSeverity_LOG_SEVERITY_INFO
			case "warn":
				level = apiv1.LogSeverity_LOG_SEVERITY_WARN
			case "error":
				level = apiv1.LogSeverity_LOG_SEVERITY_ERROR
			case "":
				level = apiv1.LogSeverity_LOG_SEVERITY_UNSPECIFIED
			default:
				return merry.Errorf("unknown log level specified")
			}

			opts.Severity = level

			var err error
			opts.Start, err = parseTimeOrDuration(start)
			if err != nil {
				return merry.Errorf("wrong format for start time '%s': %w", start, err)
			}

			opts.End, err = parseTimeOrDuration(end)
			if err != nil {
				return merry.Errorf("wrong format for end time '%s': %w", start, err)
			}

			if !opts.End.IsZero() && opts.Follow {
				return merry.Errorf("while stream logs end time cannot be specified")
			}

			return actions.NewLogs(e.Log(), e.cfg, opts).Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.StringSliceVarP(&target, "target", "t", nil, "target only specified apps or dependencies, can specify multiple or separate values with comma in a form of <app type>.<name> or dep.<dep name>, e.g.: static.website,service.api,dep.database")
	f.BoolVarP(&opts.OnlyApps, "only-apps", "a", false, "target only apps, skip all dependencies")
	f.StringVarP(&start, "start", "s", "5m", "start time")
	f.StringVarP(&end, "end", "d", "", "end time")
	f.StringVarP(&severity, "severity", "l", "", "minimum severity level (options: debug, notice, info, warn, error)")
	f.StringSliceVarP(&opts.Contains, "contains", "c", nil, "filter logs containing specific words")
	f.StringSliceVarP(&opts.NotContains, "not-contains", "x", nil, "filter logs not containing specific words")
	f.StringVarP(&opts.Filter, "filter", "q", "", "pass raw filter to logs, refer to cloud provider docs for possible options")
	f.BoolVarP(&opts.Follow, "follow", "w", false, "stream logs (end has to be unspecified)")

	return cmd
}
