package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/version"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	cmdGroupAnnotation = "cmd_group"
	cmdGroupDelimiter  = "-"

	defaultValuesYAML = "values.yaml"
)

// Command groups.
const (
	cmdGroupMain   = "1-Main"
	cmdGroupOthers = "5-Other"
)

// Inspired by similar approach in: https://github.com/hitzhangjie/godbg (Apache 2.0 License).
func helpCommandsGrouped(cmd *cobra.Command) string {
	groups := map[string][]string{}

	for _, c := range cmd.Commands() {
		groupName, ok := c.Annotations[cmdGroupAnnotation]
		if !ok {
			groupName = cmdGroupOthers
		}

		groupCmds := groups[groupName]
		cmdName := c.Name()
		rightPad := strings.Repeat(" ", 16-len(cmdName))
		groupCmds = append(groupCmds, fmt.Sprintf("  %s%s%s", pterm.Green(cmdName), rightPad, c.Short))
		sort.Strings(groupCmds)

		groups[groupName] = groupCmds
	}

	groupNames := []string{}

	for k := range groups {
		groupNames = append(groupNames, k)
	}

	sort.Strings(groupNames)

	buf := bytes.Buffer{}

	for _, groupName := range groupNames {
		commands := groups[groupName]

		group := strings.Split(groupName, cmdGroupDelimiter)[1]
		buf.WriteString(pterm.Yellow(strings.ToUpper(group), " COMMANDS:\n"))

		for _, cmd := range commands {
			buf.WriteString(fmt.Sprintf("%s\n", cmd))
		}

		buf.WriteString("\n")
	}

	if buf.Len() > 0 {
		buf.Truncate(buf.Len() - 1)
	}

	return buf.String()
}

func helpCommands(cmd *cobra.Command) string {
	buf := bytes.Buffer{}

	buf.WriteString(pterm.Yellow("COMMANDS:\n"))

	for _, c := range cmd.Commands() {
		cmdName := c.Name()
		rightPad := strings.Repeat(" ", 16-len(cmdName))
		c := fmt.Sprintf("  %s%s%s", pterm.Green(cmdName), rightPad, c.Short)

		buf.WriteString(c)
		buf.WriteString("\n")
	}

	return buf.String()
}

func rootCmdHelpFunc(log logger.Logger, cmd *cobra.Command, _ []string) {
	long := cmd.Long

	log.Println(long)
	log.Println()

	log.Println(pterm.FgYellow.Sprint("USAGE:"))

	if cmd.Runnable() {
		log.Printf("  %s\n", cmd.UseLine())
	}

	if cmd.HasAvailableSubCommands() {
		log.Printf("  %s [command]\n", pterm.Green(cmd.CommandPath()))
	}

	log.Println()

	var usage string
	if cmd.Root() == cmd {
		usage = helpCommandsGrouped(cmd)
	} else {
		usage = helpCommands(cmd)
	}

	log.Printf(usage)

	if len(cmd.Aliases) != 0 {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("ALIASES:"))
		log.Printf("  %s\n", cmd.NameAndAliases())
	}

	if cmd.HasExample() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("EXAMPLES:"))
		log.Printf(cmd.Example)
	}

	if cmd.HasAvailableLocalFlags() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("FLAGS:"))
		log.Printf(cmd.LocalFlags().FlagUsages())
	}

	if cmd.HasAvailableInheritedFlags() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("GLOBAL FLAGS:"))
		log.Printf(cmd.InheritedFlags().FlagUsages())
	}

	if cmd.HasHelpSubCommands() {
		log.Println(pterm.FgYellow.Sprint("ADDITIONAL HELP TOPICS:"))

		for _, c := range cmd.Commands() {
			if c.IsAdditionalHelpTopicCommand() {
				log.Printf("  %-16s%s\n", c.CommandPath(), c.Short)
			}
		}
	}

	if cmd.HasAvailableSubCommands() {
		log.Println()
		log.Printf(`Use "%s [command] --help" for more information about a command.`, cmd.CommandPath())
		log.Println()
	}
}

func rootCmdUsageFunc(log logger.Logger, cmd *cobra.Command) error {
	short := cmd.Short

	log.Println(short)
	log.Println()

	log.Println(pterm.FgYellow.Sprint("USAGE:"))

	if cmd.Runnable() {
		log.Printf("  %s\n", cmd.UseLine())
	}

	if cmd.HasAvailableSubCommands() {
		log.Printf("  %s [command]\n", pterm.Green(cmd.CommandPath()))
	}

	log.Println()

	var usage string
	if cmd.Root() == cmd {
		usage = helpCommandsGrouped(cmd)
	} else {
		usage = helpCommands(cmd)
	}

	log.Printf(usage)

	if len(cmd.Aliases) != 0 {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("ALIASES:"))
		log.Printf("  %s\n", cmd.NameAndAliases())
	}

	if cmd.HasExample() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("EXAMPLES:"))
		log.Printf(cmd.Example)
	}

	if cmd.HasAvailableLocalFlags() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("FLAGS:"))
		log.Printf(cmd.LocalFlags().FlagUsages())
	}

	if cmd.HasAvailableInheritedFlags() {
		log.Println()
		log.Println(pterm.FgYellow.Sprint("GLOBAL FLAGS:"))
		log.Printf(cmd.InheritedFlags().FlagUsages())
	}

	if cmd.HasAvailableSubCommands() {
		log.Println()
		log.Printf(`Use "%s [command] --help" for more information about a command.`, cmd.CommandPath())
		log.Println()
	}

	return nil
}

func (e *Executor) rootLongHelp() string {
	buf := bytes.Buffer{}

	h := pterm.Sprintf("%s - %s\n\n", pterm.Bold.Sprintf("ok"), pterm.Italic.Sprint(version.Version()))

	buf.WriteString(h)

	data := [][]string{
		{"Environment variables", "Description"},
		{pterm.ThemeDefault.TableSeparatorStyle.Sprint(strings.Repeat("-", 30)), pterm.ThemeDefault.TableSeparatorStyle.Sprint(strings.Repeat("-", 48))},
	}
	data = append(data, e.env.Info()...)

	s, _ := e.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Srender()

	buf.WriteString(s)
	buf.WriteString("\n")

	return buf.String()
}

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{defaultValuesYAML}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
}

func (e *Executor) newRoot() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	cmd := &cobra.Command{
		Use:           "ok",
		Short:         pterm.Sprintf("%s - %s", pterm.Bold.Sprintf("ok"), pterm.Italic.Sprint(version.Version())),
		Long:          e.rootLongHelp(),
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load values.
			v, err := e.opts.valueOpts.MergeValues(cmd.Context(), getter.All())
			if err != nil && (len(e.opts.valueOpts.ValueFiles) != 1 || e.opts.valueOpts.ValueFiles[0] != defaultValuesYAML) {
				return err
			}

			// Load config file.
			if err := e.loadProjectConfig(cmd.Context(), map[string]interface{}{"var": v}); err != nil && !errors.Is(err, config.ErrProjectConfigNotFound) {
				return err
			}

			if e.cfg != nil {
				if err := e.saveLockfile(); err != nil {
					_ = e.cleanupProject()
					return err
				}
			}

			return nil
		},
	}

	f := cmd.PersistentFlags()
	f.Bool("help", false, "help")

	addValueOptionsFlags(f, e.opts.valueOpts)

	f.Lookup("help").Hidden = true

	cmd.SetUsageFunc(func(c *cobra.Command) error { return rootCmdUsageFunc(e.log, c) })
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) { rootCmdHelpFunc(e.log, c, args) })

	cmd.AddCommand(
		e.newCompletionCmd(),
		e.newRunCmd(),
		e.newDeployCmd(),
		e.newForceUnlockCmd(),
	)

	return cmd
}
