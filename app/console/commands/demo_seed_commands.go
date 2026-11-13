//go:build !dev

/*
|--------------------------------------------------------------------------
| Demo Seed Commands, Release Build
|--------------------------------------------------------------------------
|
| Release builds carry no demo seed commands.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

func demoSeedCommands(*app.App) []foundation.Command {
	return nil
}
