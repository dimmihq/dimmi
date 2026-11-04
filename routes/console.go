/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own, in UTC. Every hour the sign-in
| records that ran out are purged: unfinished authenticator enrollments,
| spent mailed links and codes, expired password resets and sessions
| nobody can resume. Every night the workspaces whose time in the trash
| ran out are handed to their purges and every workspace is checked for
| an active owner, and in the morning owners of workspaces about to be
| purged are reminded.
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
	a.Schedule.Job(jobs.PurgeExpiredAuthRecordsArgs{}).Hourly()
	a.Schedule.Job(jobs.ScheduleWorkspacePurgesArgs{}).DailyAt("03:00")
	a.Schedule.Job(jobs.CheckWorkspaceOwnersArgs{}).DailyAt("04:00")
	a.Schedule.Job(jobs.SendWorkspaceDeletionRemindersArgs{}).DailyAt("09:00")
}
