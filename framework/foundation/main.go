/*
|--------------------------------------------------------------------------
| Process Entry Point
|--------------------------------------------------------------------------
|
| A binary's main function only hands the process over: its arguments,
| its standard streams and a context that lives as long as the process.
| Doing that here keeps the application's own code free of the process
| globals, so everything below main can be run by tests with arguments
| and streams of their own.
|
*/

package foundation

import (
	"context"
	"os"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
Main runs the command line with the process's arguments and standard
streams and returns the exit code for os.Exit.
*/
func Main(opts Options) int {
	streams := console.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	return Run(context.Background(), opts, os.Args[1:], streams)
}
