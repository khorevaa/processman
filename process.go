// Copyright 2020 Burak Sezer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processman

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"syscall"
)

// Process denotes a child process in Processman
type Process struct {
	env     []string
	args    []string
	name    string
	stopped int32
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	cmd     *exec.Cmd
	errChan chan error
	ctx     context.Context
	cancel  context.CancelFunc
}

// Stdout returns a pipe that will be connected to the command's
// standard output when the command starts.
func (p *Process) Stdout() (io.ReadCloser, error) {
	select {
	case <-p.ctx.Done():
		return nil, fmt.Errorf("pid: %d is gone", p.Getpid())
	default:
	}

	return p.stdout, nil
}

// Stderr returns a pipe that will be connected to the command's
// standard error when the command starts.
func (p *Process) Stderr() (io.ReadCloser, error) {
	select {
	case <-p.ctx.Done():
		return nil, fmt.Errorf("pid: %d is gone", p.Getpid())
	default:
	}
	return p.stderr, nil
}

// ErrChan returns a chan which will deliver an error or nil by exec.Wait
func (p *Process) ErrChan() chan error {
	return p.errChan
}

// Stop sends SIGTERM to the child process
func (p *Process) Stop() error {
	atomic.StoreInt32(&p.stopped, 1)

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	<-p.ctx.Done()
	return nil
}

// Kill sends SIGKILL to the child process
func (p *Process) Kill() error {
	atomic.StoreInt32(&p.stopped, 1)

	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}

	<-p.ctx.Done()
	return nil
}

// Getpid returns pid of the child process.
func (p *Process) Getpid() int {
	return p.cmd.Process.Pid
}
