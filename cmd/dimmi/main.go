/*
|--------------------------------------------------------------------------
| Dimmi
|--------------------------------------------------------------------------
|
| The binary only hands the process to the bootstrap, which assembles the
| product and runs the command asked for: serve, migrate and the rest.
|
*/

package main

import (
	"os"

	"github.com/dimmihq/dimmi/bootstrap"
)

func main() {
	os.Exit(bootstrap.Main())
}
