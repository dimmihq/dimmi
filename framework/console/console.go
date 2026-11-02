/*
|--------------------------------------------------------------------------
| Console Kernel
|--------------------------------------------------------------------------
|
| The binary's command line is a list of registered commands under one
| name chosen by the application. The kernel parses each run, hands the
| command its input and output, and turns the outcome into an exit code
| a shell script can act on:
|
|   0    the command succeeded
|   1    the command failed
|   2    the command line was wrong: unknown command, flag or argument
|   130  the run was interrupted
|
| Two flags work on every command: --no-interaction (-n) answers every
| question with its default, and --quiet (-q) prints nothing but errors.
|
*/

package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

/*
The exit codes a run ends with, described in the kernel's banner.
*/
const (
	ExitSuccess     = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitInterrupted = 130
)

/*
ExitError ends a run with a chosen exit code. Err, when set, is printed
like any other error.
*/
type ExitError struct {
	Code int
	Err  error
}

/*
Error returns the message of the wrapped error.
*/
func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}

	return e.Err.Error()
}

/*
Unwrap returns the wrapped error.
*/
func (e *ExitError) Unwrap() error {
	return e.Err
}

/*
usageError is a command line that cannot be run as written.
*/
type usageError struct {
	err error
}

/*
Error returns what is wrong with the command line.
*/
func (e usageError) Error() string {
	return e.err.Error()
}

/*
IO holds the streams a run reads and writes. Interactive says whether
someone is there to answer questions on In.
*/
type IO struct {
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Interactive bool
}

/*
StdIO returns the process's standard streams, interactive when standard
input is a terminal.
*/
func StdIO() IO {
	return IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Interactive: isTerminal(os.Stdin)}
}

/*
Console is the application's command line.
*/
type Console struct {
	name     string
	version  string
	commands []Command
}

/*
New returns a command line for the binary with the name and version.
*/
func New(name, version string) *Console {
	return &Console{name: name, version: version}
}

/*
Register adds commands. A command registered under a name already in
use replaces the earlier one.
*/
func (c *Console) Register(commands ...Command) {
	for _, command := range commands {
		c.commands = removeNamed(c.commands, command.Name())
		c.commands = append(c.commands, command)
	}
}

/*
Run runs the command named by the arguments, which exclude the binary's
own name, and returns the exit code. Without arguments it prints the
command list.
*/
func (c *Console) Run(ctx context.Context, args []string, streams IO) int {
	root := c.root(ctx, streams)
	root.SetArgs(args)

	err := root.ExecuteContext(ctx)

	return report(err, newOutput(streams, false, false), c.name)
}

func (c *Console) root(ctx context.Context, streams IO) *cobra.Command {
	root := &cobra.Command{
		Use:           c.name,
		Version:       c.version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          unknownCommand,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err: err} })
	root.CompletionOptions.DisableDefaultCmd = true
	root.SuggestionsMinimumDistance = 2
	root.PersistentFlags().BoolP("no-interaction", "n", false, "never ask a question; take every default")
	root.PersistentFlags().BoolP("quiet", "q", false, "print nothing but errors")

	for _, command := range c.commands {
		root.AddCommand(wrap(ctx, command, streams))
	}

	return root
}

/*
wrap turns a command into its cobra form, binding its arguments and
building its input and output for the run.
*/
func wrap(ctx context.Context, command Command, streams IO) *cobra.Command {
	flags := &Flags{set: pflag.NewFlagSet(command.Name(), pflag.ContinueOnError)}
	command.Flags(flags)

	cmd := &cobra.Command{
		Use:   flags.use(command.Name()),
		Short: command.Description(),
		Long:  strings.TrimSpace(command.Description() + "\n\n" + flags.help()),
		Args: func(_ *cobra.Command, words []string) error {
			if _, err := flags.bind(words); err != nil {
				return usageError{err: err}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, words []string) error {
			values, err := flags.bind(words)
			if err != nil {
				return usageError{err: err}
			}

			set := cmd.Flags()
			quiet, _ := set.GetBool("quiet")
			off, _ := set.GetBool("no-interaction")
			interactive := streams.Interactive && !off

			in := &Input{flags: set, args: values, interactive: interactive}

			return command.Handle(ctx, in, newOutput(streams, quiet, interactive))
		},
	}
	cmd.Flags().AddFlagSet(flags.set)

	return cmd
}

/*
unknownCommand refuses words given to the binary without a command,
suggesting commands with similar names.
*/
func unknownCommand(cmd *cobra.Command, words []string) error {
	if len(words) == 0 {
		return nil
	}

	err := fmt.Errorf("unknown command %q", words[0])
	if suggestions := cmd.SuggestionsFor(words[0]); len(suggestions) > 0 {
		err = fmt.Errorf("%w; did you mean %s?", err, strings.Join(suggestions, " or "))
	}

	return usageError{err: err}
}

/*
report prints a failed run's error and returns its exit code.
*/
func report(err error, out *Output, name string) int {
	var (
		exit  *ExitError
		usage usageError
	)

	switch {
	case err == nil:
		return ExitSuccess
	case errors.As(err, &exit):
		if exit.Err != nil {
			out.Error("%v", exit.Err)
		}

		return exit.Code
	case errors.As(err, &usage):
		out.Error("%v", usage.err)
		out.Error("run %q for usage", name+" help")

		return ExitUsage
	case errors.Is(err, context.Canceled), errors.Is(err, huh.ErrUserAborted):
		out.Error("interrupted")

		return ExitInterrupted
	default:
		out.Error("%v", err)

		return ExitFailure
	}
}

func removeNamed(commands []Command, name string) []Command {
	kept := commands[:0]
	for _, command := range commands {
		if command.Name() != name {
			kept = append(kept, command)
		}
	}

	return kept
}
