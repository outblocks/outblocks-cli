package run

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

type LocalDependency struct {
	*apiv1.DependencyRun
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
