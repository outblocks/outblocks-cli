package util

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type CmdInfo struct {
	cmd                    *exec.Cmd
	done                   chan struct{}
	err                    error
	stdoutPipe, stderrPipe io.ReadCloser
}

const (
	commandCleanupTimeout = 5 * time.Second
)

func NewCmdInfo(cmdStr, dir string, env []string) (*CmdInfo, error) {
	cmd := plugin_util.NewCmdAsUser(cmdStr)

	// Env.
	cmd.Env = append(os.Environ(),
		env...,
	)

	cmd.Dir = dir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	return &CmdInfo{
		done:       make(chan struct{}),
		stdoutPipe: stdoutPipe,
		stderrPipe: stderrPipe,
		cmd:        cmd,
	}, nil
}

func (i *CmdInfo) Run() error {
	err := i.cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := i.cmd.Wait()
		if err != nil {
			i.err = fmt.Errorf("exited: %s", err)
		}

		close(i.done)
	}()

	return nil
}

func (i *CmdInfo) IsRunning() bool {
	if i.cmd == nil || i.cmd.Process == nil {
		return false
	}

	select {
	case <-i.done:
		return false
	default:
		return true
	}
}

func (i *CmdInfo) Stdout() io.ReadCloser {
	return i.stdoutPipe
}

func (i *CmdInfo) Stderr() io.ReadCloser {
	return i.stderrPipe
}

func (i *CmdInfo) Wait() error {
	if i.cmd == nil || i.cmd.Process == nil {
		return nil
	}

	<-i.done

	return i.err
}

func (i *CmdInfo) WaitChannel() <-chan struct{} {
	return i.done
}

func (i *CmdInfo) Stop() error {
	if i.cmd == nil || i.cmd.Process == nil {
		return nil
	}

	_ = i.cmd.Process.Signal(syscall.SIGINT)

	go func() {
		time.Sleep(commandCleanupTimeout)

		if i.IsRunning() {
			_ = i.cmd.Process.Kill()
		}
	}()

	return i.Wait()
}
