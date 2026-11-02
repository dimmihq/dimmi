package process_test

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/process"
)

func shell(script string) process.Cmd {
	return process.Cmd{Name: "sh", Args: []string{"-c", script}}
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("returns both outputs of a successful program", func(t *testing.T) {
		t.Parallel()

		got, err := process.New().Run(t.Context(), shell("printf out; printf err >&2"))

		require.NoError(t, err)
		assert.Equal(t, process.Result{Stdout: []byte("out"), Stderr: []byte("err")}, got)
	})

	t.Run("feeds standard input to the program", func(t *testing.T) {
		t.Parallel()

		got, err := process.New().Run(t.Context(), process.Cmd{Name: "cat", Stdin: strings.NewReader("fed")})

		require.NoError(t, err)
		assert.Equal(t, "fed", string(got.Stdout))
	})

	t.Run("adds variables to the inherited environment", func(t *testing.T) {
		t.Parallel()
		cmd := shell(`printf "$GREETING:"; command -v cat >/dev/null && printf path`)
		cmd.Env = []string{"GREETING=hello"}

		got, err := process.New().Run(t.Context(), cmd)

		require.NoError(t, err)
		assert.Equal(t, "hello:path", string(got.Stdout))
	})

	t.Run("runs in the working directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		got, err := process.New().Run(t.Context(), process.Cmd{Name: "pwd", Dir: dir})

		require.NoError(t, err)
		assert.Equal(t, filepath.Base(dir), filepath.Base(strings.TrimSpace(string(got.Stdout))))
	})

	t.Run("passes arguments without a shell", func(t *testing.T) {
		t.Parallel()

		got, err := process.New().Run(t.Context(), process.Cmd{Name: "echo", Args: []string{"a; rm -rf /", "$HOME"}})

		require.NoError(t, err)
		assert.Equal(t, "a; rm -rf / $HOME\n", string(got.Stdout))
	})

	t.Run("reports a non-zero exit with the result", func(t *testing.T) {
		t.Parallel()

		got, err := process.New().Run(t.Context(), shell("printf boom >&2; exit 3"))

		var exitErr *process.ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 3, exitErr.Code)
		assert.Equal(t, "boom", string(exitErr.Stderr))
		assert.Contains(t, err.Error(), "sh exited with code 3: boom")
		assert.Equal(t, 3, got.ExitCode)
	})

	t.Run("reports a missing program", func(t *testing.T) {
		t.Parallel()

		_, err := process.New().Run(t.Context(), process.Cmd{Name: "no-such-program-anywhere"})

		require.ErrorIs(t, err, process.ErrNotFound)
	})

	t.Run("reports a working directory that does not exist", func(t *testing.T) {
		t.Parallel()

		_, err := process.New().Run(t.Context(), process.Cmd{Name: "pwd", Dir: "/no/such/dir"})

		require.Error(t, err)
		require.NotErrorIs(t, err, process.ErrNotFound)
		assert.Contains(t, err.Error(), "run pwd")
	})

	t.Run("kills a program when the context ends", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := process.New().Run(ctx, process.Cmd{Name: "sleep", Args: []string{"10"}})

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("streams standard output and collects errors on wait", func(t *testing.T) {
		t.Parallel()

		handle, err := process.New().Start(t.Context(), shell("printf streamed; printf e >&2"))
		require.NoError(t, err)

		out, err := io.ReadAll(handle.Stdout)
		require.NoError(t, err)
		got, err := handle.Wait()
		require.NoError(t, err)
		assert.Equal(t, "streamed", string(out))
		assert.Equal(t, process.Result{Stderr: []byte("e")}, got)
	})

	t.Run("reports a non-zero exit on wait", func(t *testing.T) {
		t.Parallel()

		handle, err := process.New().Start(t.Context(), shell("exit 2"))
		require.NoError(t, err)
		_, err = io.ReadAll(handle.Stdout)
		require.NoError(t, err)

		got, err := handle.Wait()

		var exitErr *process.ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 2, got.ExitCode)
	})

	t.Run("reports a missing program", func(t *testing.T) {
		t.Parallel()

		_, err := process.New().Start(t.Context(), process.Cmd{Name: "no-such-program-anywhere"})

		require.ErrorIs(t, err, process.ErrNotFound)
	})

	t.Run("refuses to start on an ended context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := process.New().Start(ctx, shell("true"))

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("kills a started program when the context ends", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())

		handle, err := process.New().Start(ctx, process.Cmd{Name: "sleep", Args: []string{"10"}})
		require.NoError(t, err)
		cancel()
		_, err = io.ReadAll(handle.Stdout)
		require.NoError(t, err)

		_, err = handle.Wait()

		require.ErrorIs(t, err, context.Canceled)
	})
}
