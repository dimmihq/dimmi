/*
|--------------------------------------------------------------------------
| Console
|--------------------------------------------------------------------------
|
| Tests run console commands the way an operator does: through the
| binary's own command line, which boots a second copy of the
| application on the test's database and Valkey prefix, changed the way
| the test case's own was. The command sees exactly what the test case's
| application wrote.
|
*/

package tests

import (
	"bytes"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/bootstrap"
)

/*
Command runs the command line with the arguments against the test's
database and returns the exit code and everything the command wrote.
*/
func (tc *TestCase) Command(args ...string) (int, string) {
	tc.T.Helper()

	application, opts := bootstrap.Application()
	for _, fn := range tc.before {
		fn(application)
	}

	opts.Environ = tc.environ
	opts.LogWriter = io.Discard

	var out bytes.Buffer

	code := foundation.Run(tc.T.Context(), opts, args, console.IO{In: strings.NewReader(""), Out: &out, Err: &out})

	return code, out.String()
}
