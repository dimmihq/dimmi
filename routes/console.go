/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own. Owners of tokens about to expire
| are warned once a day. Sessions expire by their own timestamps, and
| stale rows are only clutter until pruning is added.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
Console declares the scheduled jobs.
*/
func Console(a *app.App) {
	a.Schedule.Job(jobs.NotifyExpiringTokensArgs{}).DailyAt("09:00")
}
