# processman

A thin layer around [os/exec](https://golang.org/pkg/os/exec/) to run processes and manage their life cycle. 

processman is a library. It provides a simple API to run child processes in your Golang application. 

It runs on macOS and Linux. It requires Go 1.14 at least.

## Status

processman is in early stages of development. See [TODO](#todo) list. 

Please don't hesitate to open an issue or PR to improve processman.

## Package API

Processman:

* [Command](#command)
* [StopAll](#stopall)
* [KillAll](#killall)
* [Processes](#processes)
* [Shutdown](#shutdown)

Process:

* [Stop](#stop) 
* [Kill](#kill)
* [Getpid](#getpid)
* [ErrChan](#errchan)
* [Stdout](#stdout)
* [Stderr](#stderr)

## Usage

With a properly configured Golang environment:

```
go get -u github.com/buraksezer/processman
```

### Initialization

You just need to call `New` method to create a new Processman instance:

```go
import "github.com/buraksezer/processman"
...
pm := processman.New(nil)
...
```

You can pass your own `*log.Logger` to processman instead of `nil`

## API Methods

### Command

Command runs a new child process.

```go
p, err := pm.Command("prog", []string{"--param"}, []string{"ENV_VAR=VAL"})
```  

Command returns a `Process` and an `error`. `Process` has its own API methods to manage process life-cycle. 
See [Process](#process) section for further details.

### StopAll

StopAll sends `SIGTERM` signal to all child processes of the Processman instance.

```go
err := pm.StopAll()
```

### KillAll

KillAll sends `SIGKILL` signal to all child processes of the Processman instance.

```go
err := pm.KillAll()
```

### Processes

Processes returns currently running child processes. 

```go
processes := pm.Processes()
```

It returns `map[int]*Process`. This is a copy of an internal data structure. So you can modify this list after 
the function returned.

### Shutdown

Shutdown calls `StopAll` and frees allocated resources. A Processman instance cannot be used again after calling Shutdown.
This function calls `KillAll` if the given context is closed.

```go
err := pm.Shutdown(context.Background())
```

## Process

Process is a structure that defines a running child process. 

### Stop

Sends `SIGTERM` to the child process. 

```go
err := p.Stop()
```

### Kill

Sends `SIGKILL` to the child process. 

```go
err := p.Kill()
```

### Getpid

Returns pid of the child process.

```go
err := p.Getpid()
```

### ErrChan

Returns result of the running child process. It is a blocking call.

```go
err := <- p.ErrChan()
```

### Stdout

Returns an `io.ReadCloser` to read `stdout` of the running child process.

```go
stdout, err := p.Stdout()
```

### StdErr

Returns an `io.ReadCloser` to read `stderr` of the running child process.

```go
stderr, err := p.Stderr()
```

## Life-cycle management

Processman restarts crashed child processes. There is no retry limit for this but it waits some time before forking a 
new child processes for the crashed one. The interval is 5 seconds at maximum and it's selected randomly between 
0 and 5 seconds. 

If you stop or kill the child process programmatically, Processman doesn't try to restart it. 

Processman calls `StopAll` on its child processes if you send `SIGTERM` to the parent process.

## Running tests

```go
go test -v -cover
```

Current test coverage is around 84%. 

## Sample code

```go
pm := New(nil)
defer pm.Shutdown(context.Background())

p, err := pm.Command("uname", []string{"-a"}, nil)
if err != nil {
    // handle error
}

r, err := p.Stdout()
if err != nil {
    // handle error
}
data, err := ioutil.ReadAll(r)
if err != nil {
    // handle error
}
fmt.Println("uname -a:", string(data))
```

## TODO

* Exponential backoff based restart algorithm for crashed processes,
* Sample CLI tool for demonstration,
* Improve tests, 
* Ability to control `syscall.SysProcAttr` at API level.

## Contributions

Please don't hesitate to fork the project and send a pull request or just e-mail me to ask questions and share ideas.

## License

The Apache License, Version 2.0 - see LICENSE for more details.