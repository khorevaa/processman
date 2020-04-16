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
	"io/ioutil"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcess_Start(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("uname", []string{"-a"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	r, err := p.Stdout()
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	if data == nil {
		t.Fatalf("Some data expected from uname -a, Got: nil")
	}
}

func TestProcess_Stop(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sleep", []string{"500"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	err = <-p.ErrChan()
	_, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected *exec.ExitError (signal: interrupt), Got: %v", err)
	}
}

func TestProcess_Restart(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sleep", []string{"500"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	err = p.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	<-p.ErrChan()

	sleepExists := func(res map[int]*Process) bool {
		for _, process := range res {
			if process.name == "sleep" {
				return true
			}
		}
		return false
	}

	var retry int
	for retry <= 5 {
		res := pm.Processes()
		if len(res) != 0 {
			if sleepExists(res) {
				return
			}
			break
		}
		retry++
		<-time.After(time.Second)
	}
	t.Fatalf("sleep command cannot be restarted after SIGKILL")
}

func TestProcess_StopAll(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sleep", []string{"500"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	if err := pm.StopAll(); err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	err = <-p.ErrChan()
	_, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected *exec.ExitError (signal: interrupt), Got: %v", err)
	}

	res := pm.Processes()
	if len(res) != 0 {
		t.Fatalf("there are alive child processes: %v", res)
	}
}

func TestProcess_KillAll(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sleep", []string{"500"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	if err := pm.KillAll(); err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	err = <-p.ErrChan()
	_, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected *exec.ExitError (signal: interrupt), Got: %v", err)
	}

	res := pm.Processes()
	if len(res) != 0 {
		t.Fatalf("there are alive child processes: %v", res)
	}
}

func TestProcess_Stderr(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sh", []string{"-c", "1>&2 echo linux"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	r, err := p.Stderr()
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	// Trim new line char, probably the OS adds it.
	keyword := strings.Trim(string(data), "\n")
	if keyword != "linux" {
		t.Fatalf("Expected keyword: linux, Got: %v", keyword)
	}
}

func TestProcess_Stdout(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sh", []string{"-c", "echo linux"}, nil)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	r, err := p.Stdout()
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	// Trim new line char, probably the OS adds it.
	keyword := strings.Trim(string(data), "\n")
	if keyword != "linux" {
		t.Fatalf("Expected keyword: linux, Got: %v", keyword)
	}
}

func TestProcess_Env(t *testing.T) {
	pm := New(nil)
	defer pm.Shutdown(context.Background())

	p, err := pm.Command("sh",
		[]string{"-c", "echo $MY_ENV_VAR"},
		[]string{"MY_ENV_VAR=LINUX"},
	)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	r, err := p.Stdout()
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("Expected nil, Got: %v", err)
	}

	// Trim new line char, probably the OS adds it.
	keyword := strings.Trim(string(data), "\n")
	if keyword != "LINUX" {
		t.Fatalf("Expected keyword: LINUX, Got: %v", keyword)
	}
}
