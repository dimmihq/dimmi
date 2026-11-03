/*
|--------------------------------------------------------------------------
| Operator Commands
|--------------------------------------------------------------------------
|
| What an operator does from the terminal of the server, for users who
| cannot help themselves through the app. Each command calls the same
| action the app would, so it follows the same rules and leaves the
| same records.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
All returns the product's commands, run against the application.
*/
func All(a *app.App) []foundation.Command {
	return []foundation.Command{
		&UserDisableTwoFactorCommand{App: a},
	}
}
