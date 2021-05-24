package cmd

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/spf13/cobra"
)

func (e *Executor) newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs stack locally",
		Long:  `Runs Outblocks stack and dependencies locally.`,
		Annotations: map[string]string{
			cmdGroupAnnotation: cmdGroupMain,
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if e.cfg == nil {
				return config.ErrProjectConfigNotFound
			}

			fmt.Println("RUN E")

			// spew.Dump(e.opts.cfg)

			return nil
		},
	}

	return cmd
}
