/*
|--------------------------------------------------------------------------
| Size Probe Application
|--------------------------------------------------------------------------
|
| A realistic application binary: everything the foundation boots, run
| through its command line. The architecture tests link it to check that
| the linker can still drop unused methods, which keeps binaries small.
|
*/

package main

import (
	"context"
	"os"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
)

func main() {
	streams := console.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	os.Exit(foundation.Run(context.Background(), foundation.Options{Name: "app"}, os.Args[1:], streams))
}
