package cmd

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/version"
	"github.com/outblocks/outblocks-cli/pkg/cli/values"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	cmdGroupAnnotation = "cmd_group"
	cmdGroupDelimiter  = "-"
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

func rootCmdHelpFunc(cmd *cobra.Command, args []string) {
	long := cmd.Long

	if !pterm.PrintColor {
		long = pterm.RemoveColorFromString(long)
	}

	fmt.Println(long)
	fmt.Println()

	pterm.FgYellow.Println("USAGE:")

	if cmd.Runnable() {
		fmt.Printf("  %s\n", cmd.UseLine())
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Printf("  %s [command]\n", pterm.Green(cmd.CommandPath()))
	}

	fmt.Println()

	var usage string
	if cmd.Root() == cmd {
		usage = helpCommandsGrouped(cmd)
	} else {
		usage = helpCommands(cmd)
	}

	fmt.Print(usage)

	if len(cmd.Aliases) != 0 {
		fmt.Println()
		pterm.FgYellow.Println("ALIASES:")
		fmt.Printf("  %s\n", cmd.NameAndAliases())
	}

	if cmd.HasExample() {
		fmt.Println()
		pterm.FgYellow.Println("EXAMPLES:")
		fmt.Printf(cmd.Example)
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Println()
		pterm.FgYellow.Println("FLAGS:")
		fmt.Print(cmd.LocalFlags().FlagUsages())
	}

	if cmd.HasAvailableInheritedFlags() {
		fmt.Println()
		pterm.FgYellow.Println("GLOBAL FLAGS:")
		fmt.Print(cmd.InheritedFlags().FlagUsages())
	}

	if cmd.HasHelpSubCommands() {
		pterm.FgYellow.Println("ADDITIONAL HELP TOPICS:")

		for _, c := range cmd.Commands() {
			if c.IsAdditionalHelpTopicCommand() {
				fmt.Printf("  %-16s%s\n", c.CommandPath(), c.Short)
			}
		}
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Println()
		fmt.Printf(`Use "%s [command] --help" for more information about a command.`, cmd.CommandPath())
		fmt.Println()
	}
}

func rootCmdUsageFunc(cmd *cobra.Command) error {
	short := cmd.Short

	if !pterm.PrintColor {
		short = pterm.RemoveColorFromString(short)
	}

	fmt.Println(short)
	fmt.Println()

	pterm.FgYellow.Println("USAGE:")

	if cmd.Runnable() {
		fmt.Printf("  %s\n", cmd.UseLine())
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Printf("  %s [command]\n", pterm.Green(cmd.CommandPath()))
	}

	fmt.Println()

	var usage string
	if cmd.Root() == cmd {
		usage = helpCommandsGrouped(cmd)
	} else {
		usage = helpCommands(cmd)
	}

	fmt.Print(usage)

	if len(cmd.Aliases) != 0 {
		fmt.Println()
		pterm.FgYellow.Println("ALIASES:")
		fmt.Printf("  %s\n", cmd.NameAndAliases())
	}

	if cmd.HasExample() {
		fmt.Println()
		pterm.FgYellow.Println("EXAMPLES:")
		fmt.Printf(cmd.Example)
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Println()
		pterm.FgYellow.Println("FLAGS:")
		fmt.Print(cmd.LocalFlags().FlagUsages())
	}

	if cmd.HasAvailableInheritedFlags() {
		fmt.Println()
		pterm.FgYellow.Println("GLOBAL FLAGS:")
		fmt.Print(cmd.InheritedFlags().FlagUsages())
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Println()
		fmt.Printf(`Use "%s [command] --help" for more information about a command.`, cmd.CommandPath())
		fmt.Println()
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

	s, _ := pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData(data)).Srender()

	buf.WriteString(s)
	buf.WriteString("\n")

	return buf.String()
}

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{"values.yaml"}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
}

func (e *Executor) newRoot() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	cmd := &cobra.Command{
		Use:           "ok",
		Short:         pterm.Sprintf("%s - %s", pterm.Bold.Sprintf("ok"), pterm.Italic.Sprint(version.Version())),
		Long:          e.rootLongHelp(),
		SilenceErrors: true,
	}

	f := cmd.PersistentFlags()
	f.Bool("help", false, "help")

	addValueOptionsFlags(f, e.opts.valueOpts)

	f.Lookup("help").Hidden = true

	cmd.SetUsageFunc(rootCmdUsageFunc)
	cmd.SetHelpFunc(rootCmdHelpFunc)

	cmd.AddCommand(
		e.newCompletionCmd(),
		e.newRunCmd(),
	)

	return cmd
}
