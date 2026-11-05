/*
|--------------------------------------------------------------------------
| SAML Probe Application
|--------------------------------------------------------------------------
|
| The foundation with every SAML path an application reaches linked in:
| starting a sign-in, reading an answer and publishing metadata. The SAML
| library carries template code for a posting binding the wrapper never
| uses; this probe proves none of it keeps the linker from dropping
| unused methods.
|
*/

package main

import (
	"context"
	"os"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/saml"
)

func main() {
	streams := console.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	if len(os.Args) > 1 && os.Args[1] == "saml" {
		provider, err := saml.New(saml.Config{Clock: clock.System()})
		if err == nil {
			_, _, _ = provider.AuthURL("state")
			_, _ = provider.Consume(os.Args[2], "")
			_, _ = provider.Metadata()
		}

		_, _ = saml.ParseMetadata([]byte(os.Args[2]))
	}

	os.Exit(foundation.Run(context.Background(), foundation.Options{Name: "app"}, os.Args[1:], streams))
}
