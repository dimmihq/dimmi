/*
|--------------------------------------------------------------------------
| Command List
|--------------------------------------------------------------------------
|
| The product's own console commands, handed to the framework with the
| application they work on. The framework boots the application before
| a command runs, so each one calls the actions it needs.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
All returns the product's console commands.
*/
func All(a *app.App) []foundation.Command {
	return []foundation.Command{
		WorkspaceRepairCommand{App: a},
	}
}
