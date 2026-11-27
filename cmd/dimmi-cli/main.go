/*
Command dimmi is dimmi's command-line tool: dimmi login, dimmi migrate
and the commands that follow. The tool itself lives in package cli.
*/
package main

import (
	"os"

	"github.com/dimmihq/dimmi/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
