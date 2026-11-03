/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own. Every night the audit log and the
| security log drop the entries older than they are kept for, at a quiet
| hour, a few minutes apart so they never compete.
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
	a.Schedule.Job(jobs.PruneAuditLogsArgs{}).DailyAt("03:10")
	a.Schedule.Job(jobs.PruneSecurityEventsArgs{}).DailyAt("03:20")
}
