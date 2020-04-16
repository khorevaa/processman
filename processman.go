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

/*package processman implements a very simple process supervisor to run child processes in your Go program*/
package processman

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
)

const maximumInterval = 5000 // 5s in milliseconds

// ErrProcessmanGone denotes Shutdown function is called and this instance cannot be used anymore
var ErrProcessmanGone = errors.New("processman instance has been closed")

// Processman implements a very simple child process supervisor
type Processman struct {
	mtx       sync.RWMutex
	processes map[int]*Process

	logger *log.Logger
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// New returns a new Processman instance
func New(logger *log.Logger) *Processman {
	if logger == nil {
		logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pm := &Processman{
		logger:    logger,
		processes: make(map[int]*Process),
		ctx:       ctx,
		cancel:    cancel,
	}

	rand.Seed(time.Now().UnixNano())
	pm.wg.Add(1)
	go pm.waitForSigterm()
	return pm
}

func (pm *Processman) waitForSigterm() {
	defer pm.wg.Done()
	sigtermCh := make(chan os.Signal, 1)
	signal.Notify(sigtermCh, syscall.SIGTERM)

	select {
	case <-sigtermCh:
		if err := pm.StopAll(); err != nil {
			pm.logger.Printf("[ERROR] StopAll returned an error: %v", err)
		}
	case <-pm.ctx.Done():
		// don't wait indefinitely.
	}
}

func (pm *Processman) restartProcess(name string, args, env []string) {
	defer pm.wg.Done()

	interval := time.Duration(rand.Intn(maximumInterval)) * time.Millisecond
	pm.logger.Printf("[WARN] Trying restart %s %s, interval: %v",
		name,
		strings.Join(args, " "),
		interval,
	)

	select {
	case <-pm.ctx.Done():
	// processman is gone
	case <-time.After(interval):
		if _, err := pm.Command(name, args, env); err != nil {
			pm.logger.Printf("[ERROR] Failed to restart command: %s: %v", name, err)
			pm.wg.Add(1)
			go pm.restartProcess(name, args, env)
		}
	}
}

func (pm *Processman) callWait(p *Process) {
	defer pm.wg.Done()
	defer close(p.errChan)

	err := p.cmd.Wait()
	if err != nil {
		if atomic.LoadInt32(&p.stopped) != int32(1) {
			// Something went wrong for the child process, try to restart it.
			pm.wg.Add(1)
			go pm.restartProcess(p.name, p.args, p.env)

		}
	}

	p.errChan <- err
	p.cancel()

	pm.mtx.Lock()
	delete(pm.processes, p.cmd.Process.Pid)
	pm.mtx.Unlock()
}

// Command starts a new child process and creates a goroutine to wait for it.
func (pm *Processman) Command(name string, args, env []string) (*Process, error) {
	select {
	case <-pm.ctx.Done():
		return nil, ErrProcessmanGone
	default:
	}
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)

	ctx, cancel := context.WithCancel(context.Background())
	p := &Process{
		env:     env,
		name:    name,
		args:    args,
		errChan: make(chan error, 1),
		cmd:     cmd,
		ctx:     ctx,
		cancel:  cancel,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p.stdout = stdout

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	p.stderr = stderr

	err = p.cmd.Start()
	if err != nil {
		return nil, err
	}

	pm.mtx.Lock()
	pm.processes[p.cmd.Process.Pid] = p
	pm.mtx.Unlock()

	pm.wg.Add(1)
	go pm.callWait(p)

	return p, nil
}

// StopAll sends SIGTERM to all child processes of this Processman instance
func (pm *Processman) StopAll() error {
	pm.mtx.RLock()
	defer pm.mtx.RUnlock()

	var result error
	for _, process := range pm.processes {
		if err := process.Stop(); err != nil {
			result = multierror.Append(result, fmt.Errorf("stop: pid: %d: %v", process.Getpid(), err))
		}
	}
	return result
}

// KillAll sends SIGKILL to all child processes of this Processman instance
func (pm *Processman) KillAll() error {
	pm.mtx.RLock()
	defer pm.mtx.RUnlock()

	var result error
	for _, process := range pm.processes {
		if err := process.Kill(); err != nil {
			result = multierror.Append(result, fmt.Errorf("kill: pid: %d: %v", process.Getpid(), err))
		}
	}
	return result
}

// Processes returns a list of child processes
func (pm *Processman) Processes() map[int]*Process {
	pm.mtx.RLock()
	defer pm.mtx.RUnlock()

	res := make(map[int]*Process)
	for pid, process := range pm.processes {
		res[pid] = process
	}
	return res
}

// Shutdowns this Processman instance. Sends SIGTERM to all child processes. If ctx is cancelled for any reason,
// it sends SIGKILL.
func (pm *Processman) Shutdown(ctx context.Context) error {
	pm.cancel()

	var result error

	// Send SIGTERM to the child processes of this library
	err := pm.StopAll()
	if err != nil {
		result = multierror.Append(result, err)
	}

	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			result = multierror.Append(result, err)
		}

		// External context is closed. Send SIGKILL to the child processes of this library.
		if err = pm.KillAll(); err != nil {
			result = multierror.Append(result, err)
		}
	case <-done:
	}
	return result
}
