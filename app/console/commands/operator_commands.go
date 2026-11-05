/*
|--------------------------------------------------------------------------
| Operator Commands
|--------------------------------------------------------------------------
|
| The commands the operator of a server runs on its workspaces and
| tokens: listing them, freezing and thawing one, taking one out of the
| trash or deleting it for good, and minting a token for a script that
| sets a fresh server up. They work on the database directly, so they
| run with the app stopped.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
operatorCommands returns the operator's workspace and token commands.
*/
func operatorCommands(a *app.App) []foundation.Command {
	return []foundation.Command{
		WorkspaceListCommand{App: a},
		WorkspaceSuspendCommand{App: a},
		WorkspaceUnsuspendCommand{App: a},
		WorkspaceRestoreCommand{App: a},
		WorkspacePurgeCommand{App: a},
		TokenCreateCommand{App: a},
	}
}
