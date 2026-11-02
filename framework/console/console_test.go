package console_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
greet is a command exercising arguments, flags and every kind of output.
*/
type greet struct{}

func (greet) Name() string {
	return "greet:user"
}

func (greet) Description() string {
	return "Greet a user"
}

func (greet) Flags(flags *console.Flags) {
	flags.Argument("name", "who to greet")
	flags.OptionalArgument("title", "how to address them")
	flags.String("punctuation", "!", "what ends the greeting")
	flags.Int("times", 1, "how often to greet")
	flags.Bool("shout", "greet in capitals")
}

func (greet) Handle(_ context.Context, in *console.Input, out *console.Output) error {
	line := strings.TrimSpace(in.Argument("title")+" "+in.Argument("name")) + in.String("punctuation")
	if in.Bool("shout") {
		line = strings.ToUpper(line)
	}

	for range in.Int("times") {
		out.Line("Hello %s", line)
	}

	out.Info("greeted")
	out.Warn("interactive=%t undeclared=%q", in.Interactive(), in.String("missing"))

	return nil
}

/*
failing is a command whose run returns the error it holds.
*/
type failing struct{ err error }

func (failing) Name() string {
	return "fail"
}

func (failing) Description() string {
	return "Fail on purpose"
}

func (failing) Flags(*console.Flags) {}

func (f failing) Handle(context.Context, *console.Input, *console.Output) error {
	return f.err
}

/*
run executes the arguments on a console with the commands and returns
the exit code with everything written to each stream.
*/
func run(t *testing.T, streams console.IO, args []string, commands ...console.Command) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	streams.Out, streams.Err = &stdout, &stderr
	if streams.In == nil {
		streams.In = strings.NewReader("")
	}

	app := console.New("app", "1.2.3")
	app.Register(commands...)

	code := app.Run(t.Context(), args, streams)

	return code, stdout.String(), stderr.String()
}

func TestConsole(t *testing.T) {
	t.Parallel()

	t.Run("runs a command with its arguments and flags", func(t *testing.T) {
		t.Parallel()

		code, stdout, stderr := run(t, console.IO{},
			[]string{"greet:user", "ada", "dr", "--punctuation=.", "--times", "2", "--shout"}, greet{})

		assert.Equal(t, console.ExitSuccess, code)
		assert.Equal(t, "Hello DR ADA.\nHello DR ADA.\ngreeted\ninteractive=false undeclared=\"\"\n", stdout)
		assert.Empty(t, stderr)
	})

	t.Run("prints nothing but errors when quiet", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, console.IO{}, []string{"greet:user", "ada", "-q"}, greet{})

		assert.Equal(t, console.ExitSuccess, code)
		assert.Empty(t, stdout)
	})

	t.Run("is interactive only when someone can answer and no-interaction is off", func(t *testing.T) {
		t.Parallel()

		_, on, _ := run(t, console.IO{Interactive: true}, []string{"greet:user", "ada"}, greet{})
		_, off, _ := run(t, console.IO{Interactive: true}, []string{"greet:user", "ada", "-n"}, greet{})

		assert.Contains(t, on, "interactive=true")
		assert.Contains(t, off, "interactive=false")
	})

	t.Run("lists the commands when called without one", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, console.IO{}, nil, greet{})

		assert.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "greet:user")
		assert.Contains(t, stdout, "Greet a user")
	})

	t.Run("describes a command's arguments in its help", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, console.IO{}, []string{"greet:user", "--help"}, greet{})

		assert.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "greet:user <name> [title]")
		assert.Contains(t, stdout, "who to greet")
		assert.Contains(t, stdout, "--no-interaction")
	})

	t.Run("prints its version", func(t *testing.T) {
		t.Parallel()

		_, stdout, _ := run(t, console.IO{}, []string{"--version"})

		assert.Contains(t, stdout, "1.2.3")
	})

	t.Run("replaces a command registered again under its name", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, console.IO{}, []string{"fail"},
			failing{err: errors.New("first")}, failing{err: errors.New("second")})

		assert.Equal(t, console.ExitFailure, code)
		assert.Equal(t, "second\n", stderr)
	})
}

func TestExitCodes(t *testing.T) {
	t.Parallel()

	usage := map[string][]string{
		"an unknown command":             {"gret:user"},
		"an unknown flag":                {"greet:user", "ada", "--loud"},
		"a missing argument":             {"greet:user"},
		"too many arguments":             {"greet:user", "ada", "dr", "extra"},
		"a flag value of the wrong type": {"greet:user", "ada", "--times", "many"},
	}

	for name, args := range usage {
		t.Run("exits with a usage error on "+name, func(t *testing.T) {
			t.Parallel()

			code, _, stderr := run(t, console.IO{}, args, greet{})

			assert.Equal(t, console.ExitUsage, code)
			assert.Contains(t, stderr, `run "app help" for usage`)
		})
	}

	t.Run("suggests a command close to an unknown one", func(t *testing.T) {
		t.Parallel()

		_, _, stderr := run(t, console.IO{}, []string{"greet:usr"}, greet{})

		assert.Contains(t, stderr, "did you mean greet:user?")
	})

	t.Run("exits with failure and prints the error", func(t *testing.T) {
		t.Parallel()

		code, stdout, stderr := run(t, console.IO{}, []string{"fail", "-q"}, failing{err: errors.New("disk full")})

		assert.Equal(t, console.ExitFailure, code)
		assert.Empty(t, stdout)
		assert.Equal(t, "disk full\n", stderr)
	})

	t.Run("exits with the code a command chooses", func(t *testing.T) {
		t.Parallel()

		withError, _, stderr := run(t, console.IO{}, []string{"fail"},
			failing{err: &console.ExitError{Code: 3, Err: errors.New("checks failed")}})
		silent, _, quiet := run(t, console.IO{}, []string{"fail"}, failing{err: &console.ExitError{Code: 4}})

		assert.Equal(t, 3, withError)
		assert.Equal(t, "checks failed\n", stderr)
		assert.Equal(t, 4, silent)
		assert.Empty(t, quiet)
	})

	t.Run("exits as interrupted when the run is canceled", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, console.IO{}, []string{"fail"}, failing{err: context.Canceled})

		assert.Equal(t, console.ExitInterrupted, code)
		assert.Equal(t, "interrupted\n", stderr)
	})
}

func TestExitError(t *testing.T) {
	t.Parallel()

	t.Run("reads as its error or its code", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("checks failed")

		require.ErrorIs(t, &console.ExitError{Code: 1, Err: cause}, cause)
		assert.Equal(t, "checks failed", (&console.ExitError{Code: 1, Err: cause}).Error())
		assert.Equal(t, "exit code 4", (&console.ExitError{Code: 4}).Error())
	})
}

func TestStdIO(t *testing.T) {
	t.Parallel()

	t.Run("uses the process streams", func(t *testing.T) {
		t.Parallel()

		streams := console.StdIO()

		assert.NotNil(t, streams.In)
		assert.NotNil(t, streams.Out)
		assert.NotNil(t, streams.Err)
	})
}
