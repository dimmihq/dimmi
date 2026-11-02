/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own, such as pruning expired sign-in
| records. Nothing is scheduled yet: sessions expire by their own
| timestamps, and stale rows are only clutter until pruning is added.
|
*/

package routes

import "github.com/dimmihq/dimmi/app"

/*
Console declares the scheduled jobs.
*/
func Console(*app.App) {}
