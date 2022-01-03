package run

import (
	"github.com/ansel1/merry/v2"
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
	return nil, merry.New("unimplemented")
}
