/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own, in UTC. Every hour the sign-in
| records that ran out are purged: unfinished authenticator enrollments,
| spent mailed links and codes, expired password resets and sessions
| nobody can resume. Every night the workspaces whose time in the trash
| ran out are handed to their purges, the audit log and the security log
| drop the entries older than they are kept for, a few minutes apart so
| they never compete, and every workspace is checked for an active owner.
| In the morning owners of workspaces about to be purged are reminded,
| and owners of access tokens about to expire are warned.
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
	a.Schedule.Job(jobs.PruneAuditLogsArgs{}).DailyAt("03:10")
	a.Schedule.Job(jobs.PruneSecurityEventsArgs{}).DailyAt("03:20")
	a.Schedule.Job(jobs.PurgeStaleDraftsArgs{}).DailyAt("03:30")
	a.Schedule.Job(jobs.PruneEmailAddressTestsArgs{}).DailyAt("03:40")
	a.Schedule.Job(jobs.CheckWorkspaceOwnersArgs{}).DailyAt("04:00")
	a.Schedule.Job(jobs.SendWorkspaceDeletionRemindersArgs{}).DailyAt("09:00")
	a.Schedule.Job(jobs.NotifyExpiringTokensArgs{}).DailyAt("09:05")
	a.Schedule.Job(jobs.SweepAssignmentQueuesArgs{}).EveryMinute()
	a.Schedule.Job(jobs.RemindInvitersArgs{}).DailyAt("09:10")
	a.Schedule.Job(jobs.ScheduleEmailDomainChecksArgs{}).EveryMinute()
}
