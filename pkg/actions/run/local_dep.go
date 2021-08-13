package run

import (
	"fmt"

	"github.com/outblocks/outblocks-plugin-go/types"
)

type LocalDependency struct {
	*types.DependencyRun
}

type LocalDependencyRunInfo struct {
	*CmdInfo
	*LocalDependency
	StdoutCh chan string
	StderrCh chan string
}

func (d *LocalDependency) Run() (*LocalDependencyRunInfo, error) {
	return nil, fmt.Errorf("unimplemented")
}
