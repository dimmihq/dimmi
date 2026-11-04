//go:build !dev

/*
|--------------------------------------------------------------------------
| Development Commands, Release Build
|--------------------------------------------------------------------------
|
| Release builds carry no development commands, such as the load run that
| floods an install with mail.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

func devCommands(*app.App) []foundation.Command {
	return nil
}
