//go:build dev

/*
|--------------------------------------------------------------------------
| Development Commands
|--------------------------------------------------------------------------
|
| Commands that only help while building and measuring the product. They
| are compiled into development builds, made with the dev build tag.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

func devCommands(a *app.App) []foundation.Command {
	return []foundation.Command{
		EmailLoadCommand{App: a},
		DisposableDomainsCommand{App: a},
		BroadcastDemoCommand{App: a},
		SeriesDemoCommand{App: a},
		InboxDemoCommand{App: a},
	}
}
