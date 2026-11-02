/*
|--------------------------------------------------------------------------
| Commands
|--------------------------------------------------------------------------
|
| A command is one thing an operator can do from the terminal, named in
| the colon style (make:job, db:seed). It declares its arguments and
| flags, then handles a run with the parsed input and an output to write
| to. Commands never parse os.Args or write to os.Stdout themselves, so
| every command runs the same in a terminal, a pipe and a test.
|
*/

package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

/*
Command is one console command.
*/
type Command interface {
	/*
		Name is what the command is called with, such as "make:job".
	*/
	Name() string

	/*
		Description is the one line shown in the command list.
	*/
	Description() string

	/*
		Flags declares the command's arguments and flags.
	*/
	Flags(flags *Flags)

	/*
		Handle runs the command. A returned error fails the run; ExitError
		chooses the exit code.
	*/
	Handle(ctx context.Context, in *Input, out *Output) error
}

/*
argument is a positional argument a command declares.
*/
type argument struct {
	name     string
	usage    string
	optional bool
}

/*
Flags collects the arguments and flags a command declares. Arguments
are matched to positional words in the order they are declared;
required arguments come before optional ones.
*/
type Flags struct {
	set  *pflag.FlagSet
	args []argument
}

/*
String declares a text flag with its default value.
*/
func (f *Flags) String(name, value, usage string) {
	f.set.String(name, value, usage)
}

/*
Bool declares a switch, off unless given.
*/
func (f *Flags) Bool(name, usage string) {
	f.set.Bool(name, false, usage)
}

/*
Int declares a whole-number flag with its default value.
*/
func (f *Flags) Int(name string, value int, usage string) {
	f.set.Int(name, value, usage)
}

/*
Argument declares a required positional argument.
*/
func (f *Flags) Argument(name, usage string) {
	f.args = append(f.args, argument{name: name, usage: usage})
}

/*
OptionalArgument declares a positional argument that may be left out.
*/
func (f *Flags) OptionalArgument(name, usage string) {
	f.args = append(f.args, argument{name: name, usage: usage, optional: true})
}

/*
use returns the usage line: the name followed by the arguments, with
optional ones in brackets.
*/
func (f *Flags) use(name string) string {
	parts := []string{name}
	for _, arg := range f.args {
		if arg.optional {
			parts = append(parts, "["+arg.name+"]")

			continue
		}

		parts = append(parts, "<"+arg.name+">")
	}

	return strings.Join(parts, " ")
}

/*
help describes each argument, for the long help text.
*/
func (f *Flags) help() string {
	if len(f.args) == 0 {
		return ""
	}

	lines := []string{"Arguments:"}
	for _, arg := range f.args {
		lines = append(lines, fmt.Sprintf("  %-14s %s", arg.name, arg.usage))
	}

	return strings.Join(lines, "\n")
}

/*
bind matches the positional words to the declared arguments, refusing
missing required ones and words left over.
*/
func (f *Flags) bind(words []string) (map[string]string, error) {
	values := map[string]string{}

	for i, arg := range f.args {
		if i >= len(words) {
			if !arg.optional {
				return nil, fmt.Errorf("missing argument <%s>", arg.name)
			}

			continue
		}

		values[arg.name] = words[i]
	}

	if len(words) > len(f.args) {
		return nil, fmt.Errorf("unexpected argument %q", words[len(f.args)])
	}

	return values, nil
}

/*
Input is a parsed command line: the arguments by name and the flags,
the global ones included. Reading a name that was never declared gives
the zero value.
*/
type Input struct {
	flags       *pflag.FlagSet
	args        map[string]string
	interactive bool
}

/*
Argument returns the named argument, or "" when an optional one was
left out.
*/
func (in *Input) Argument(name string) string {
	return in.args[name]
}

/*
String returns the named text flag.
*/
func (in *Input) String(name string) string {
	value, _ := in.flags.GetString(name)

	return value
}

/*
Bool returns the named switch.
*/
func (in *Input) Bool(name string) bool {
	value, _ := in.flags.GetBool(name)

	return value
}

/*
Int returns the named whole-number flag.
*/
func (in *Input) Int(name string) int {
	value, _ := in.flags.GetInt(name)

	return value
}

/*
Interactive reports whether someone is there to answer questions.
*/
func (in *Input) Interactive() bool {
	return in.interactive
}
