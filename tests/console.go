/*
|--------------------------------------------------------------------------
| Commands In Tests
|--------------------------------------------------------------------------
|
| Operator commands run in tests through the real command line, against
| the test's own application, so a test types what an operator would and
| reads what they would see, exit code included.
|
*/

package tests

import (
	"bytes"
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	commands "github.com/dimmihq/dimmi/app/console/commands"
)

/*
RunCommand runs the command line with the arguments, such as
"user:disable-2fa" and an email, and returns the exit code and
everything it printed.
*/
func (tc *TestCase) RunCommand(args ...string) (int, string) {
	tc.T.Helper()

	var out bytes.Buffer

	cli := console.New("dimmi", "test")
	for _, command := range commands.All(tc.App) {
		cli.Register(boundCommand{command: command, app: tc.App.Application})
	}

	code := cli.Run(tc.T.Context(), args, console.IO{In: strings.NewReader(""), Out: &out, Err: &out})

	return code, out.String()
}

/*
boundCommand runs a product command against an application already
booted, as the binary does once it booted one.
*/
type boundCommand struct {
	command foundation.Command
	app     *foundation.Application
}

/*
Name returns the command's name.
*/
func (b boundCommand) Name() string {
	return b.command.Name()
}

/*
Description returns the command's description.
*/
func (b boundCommand) Description() string {
	return b.command.Description()
}

/*
Flags declares the command's arguments and flags.
*/
func (b boundCommand) Flags(flags *console.Flags) {
	b.command.Flags(flags)
}

/*
Handle runs the command against the application.
*/
func (b boundCommand) Handle(ctx context.Context, in *console.Input, out *console.Output) error {
	return b.command.Handle(ctx, b.app, in, out)
}
