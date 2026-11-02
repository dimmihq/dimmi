/*
|--------------------------------------------------------------------------
| Channel Routes
|--------------------------------------------------------------------------
|
| Who may listen on which realtime channel. No channel is open yet, so
| the realtime server refuses every subscription.
|
*/

package routes

import "github.com/dimmihq/dimmi/app"

/*
Channels declares the realtime channel authorizers.
*/
func Channels(*app.App) {}
