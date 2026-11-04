/*
|--------------------------------------------------------------------------
| Provider List
|--------------------------------------------------------------------------
|
| The order is the boot order: the application first, since every other
| provider uses it, then sign-in, then realtime, which authenticates
| with sign-in's guards, then the routes, which need the actions.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
All returns the product's providers, with the function that declares
its routes.
*/
func All(a *app.App, routes func(a *app.App)) []foundation.Provider {
	return []foundation.Provider{
		AppServiceProvider{App: a},
		AuthServiceProvider{App: a},
		BroadcastServiceProvider{App: a},
		RouteServiceProvider{App: a, Routes: routes},
	}
}
