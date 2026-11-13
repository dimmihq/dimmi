//go:build dev

/*
|--------------------------------------------------------------------------
| Demo Seed Commands
|--------------------------------------------------------------------------
|
| The commands that seed and measure the Stowline demo workspace. Like
| the other development commands they are compiled only into builds
| made with the dev build tag, so release builds carry none of the
| demo's content.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

func demoSeedCommands(a *app.App) []foundation.Command {
	return []foundation.Command{
		WorkspaceDemoCommand{App: a},
		KnowledgeDemoEmbedCommand{App: a},
		AISimulateCommand{App: a},
	}
}
