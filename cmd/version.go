package cmd

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/version"
	"github.com/spf13/cobra"
)

func (e *Executor) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Long:  `Show current build version.`,
		Annotations: map[string]string{
			cmdSkipLoadConfigAnnotation: "1",
		},
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%#v\n", version.Get())
		},
	}

	return cmd
}
