/*
|--------------------------------------------------------------------------
| Web Routes
|--------------------------------------------------------------------------
|
| Pages a browser opens directly rather than calls from the dashboard:
| the sign-in provider redirects and callbacks, and the shells of the
| sign-in screens. None are served yet; the dashboard signs in through
| the API.
|
*/

package routes

import "github.com/dimmihq/dimmi/app"

/*
Web declares the browser page routes.
*/
func Web(*app.App) {}
