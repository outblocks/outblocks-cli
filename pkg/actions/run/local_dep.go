package run

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type LocalDependency struct {
	*types.DependencyRun
}

type LocalDependencyRunInfo struct {
	*util.CmdInfo
	*LocalDependency
	StdoutCh chan string
	StderrCh chan string
}

func (d *LocalDependency) Run() (*LocalDependencyRunInfo, error) {
	return nil, fmt.Errorf("unimplemented")
}
