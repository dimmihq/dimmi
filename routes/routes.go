/*
|--------------------------------------------------------------------------
| Route Files
|--------------------------------------------------------------------------
|
| Every route file is declared on the booted application, in one place,
| so the provider that loads them needs to know only this function.
|
*/

package routes

import "github.com/dimmihq/dimmi/app"

/*
Register declares every route file's routes.
*/
func Register(a *app.App) {
	API(a)
	Account(a)
	Web(a)
	Console(a)
	Channels(a)
	Webhooks(a)
}
