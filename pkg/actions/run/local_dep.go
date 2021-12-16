package run

import (
	"fmt"

	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

type LocalDependency struct {
	*apiv1.DependencyRun
}

type LocalDependencyRunInfo struct {
	*command.Cmd
	*LocalDependency
	StdoutCh chan string
	StderrCh chan string
}

func (d *LocalDependency) Run() (*LocalDependencyRunInfo, error) {
	return nil, fmt.Errorf("unimplemented")
}
