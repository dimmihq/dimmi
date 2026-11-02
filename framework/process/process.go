/*
|--------------------------------------------------------------------------
| Processes
|--------------------------------------------------------------------------
|
| A few jobs need a program the application cannot replace, like the
| database's own dump and restore tools. Running one is a boundary the
| application does not control, so every run goes through here, and
| nowhere else uses os/exec: the outcome comes back as a plain result,
| and a failed exit is an error that carries what the program printed.
|
| Arguments go to the program as they are, never through a shell, so no
| argument can inject another command. A canceled context kills the
| program, and the caller sees the context's error rather than the kill
| signal, so shutdowns and timeouts read as what they are.
|
*/

package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

/*
ErrNotFound means the program is not installed.
*/
var ErrNotFound = errors.New("process: program not found")

/*
Cmd describes one program run.

Name is the program, looked up on PATH unless it holds a slash. Env adds
NAME=value entries to the inherited environment. Dir is the working
directory; empty means the current one. Stdin feeds the program's
standard input; nil gives it none.
*/
type Cmd struct {
	Name  string
	Args  []string
	Env   []string
	Dir   string
	Stdin io.Reader
}

/*
Result is how a run ended: the exit code and everything the program
wrote. Stdout is empty for a started program, whose output was streamed.
*/
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

/*
ExitError reports a program that exited with a non-zero code, carrying
the code and what the program wrote to standard error.
*/
type ExitError struct {
	Name   string
	Code   int
	Stderr []byte
}

/*
Error names the program, its exit code and its error output.
*/
func (e *ExitError) Error() string {
	return fmt.Sprintf("process: %s exited with code %d: %s", e.Name, e.Code, strings.TrimSpace(string(e.Stderr)))
}

/*
Handle is a started program. Read Stdout to the end, then call Wait.
*/
type Handle struct {
	Stdout io.Reader
	wait   func() (Result, error)
}

/*
Wait waits for the program to exit and returns how it ended. A non-zero
exit returns the result together with an ExitError.
*/
func (h *Handle) Wait() (Result, error) {
	return h.wait()
}

/*
Process runs programs. The zero value is ready to use.
*/
type Process struct{}

/*
New returns a process runner.
*/
func New() *Process {
	return &Process{}
}

/*
Run runs the program to completion and collects both of its outputs. A
non-zero exit returns the result together with an ExitError.
*/
func (*Process) Run(ctx context.Context, c Cmd) (Result, error) {
	var stdout, stderr bytes.Buffer

	cmd := command(ctx, c)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return finish(ctx, c.Name, Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, err)
}

/*
Start starts the program and returns a handle that streams its standard
output; its error output is collected for Wait.
*/
func (*Process) Start(ctx context.Context, c Cmd) (*Handle, error) {
	var stderr bytes.Buffer

	cmd := command(ctx, c)
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("pipe %s output: %w", c.Name, err)
	}

	if err := cmd.Start(); err != nil {
		_, err = finish(ctx, c.Name, Result{}, err)

		return nil, err
	}

	return &Handle{Stdout: stdout, wait: func() (Result, error) {
		err := cmd.Wait()

		return finish(ctx, c.Name, Result{Stderr: stderr.Bytes()}, err)
	}}, nil
}

func command(ctx context.Context, c Cmd) *exec.Cmd {
	cmd := program(ctx, c.Name)
	cmd.Args = append(cmd.Args, c.Args...)
	cmd.Dir = c.Dir
	cmd.Stdin = c.Stdin

	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.Env...)
	}

	return cmd
}

func program(ctx context.Context, name string) *exec.Cmd {
	return exec.CommandContext(ctx, name)
}

/*
finish maps how exec ended a run: a canceled context wins over the kill
it caused, a missing program is ErrNotFound, and a non-zero exit becomes
an ExitError.
*/
func finish(ctx context.Context, name string, result Result, err error) (Result, error) {
	if err == nil {
		return result, nil
	}

	if ctx.Err() != nil {
		return result, fmt.Errorf("run %s: %w", name, ctx.Err())
	}

	if errors.Is(err, exec.ErrNotFound) {
		return result, fmt.Errorf("run %s: %w", name, ErrNotFound)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return result, fmt.Errorf("run %s: %w", name, err)
	}

	result.ExitCode = exitErr.ExitCode()

	return result, &ExitError{Name: name, Code: result.ExitCode, Stderr: result.Stderr}
}
