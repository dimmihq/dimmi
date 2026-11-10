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
| drop the entries older than they are kept for, the agent's traces lose
| their payloads and old runs, all a few minutes apart so they never
| compete, every workspace is checked for an active owner, and the
| knowledge vectors are swept: refused inputs retried, missing ones
| filled and models out of use for a week deleted.
| In the morning owners of workspaces about to be purged are reminded,
| and owners of access tokens about to expire are warned. Every hour the
| auth provider key sets visitors sign in with are fetched again when
| their copy is six hours old. Every minute the Slack digests whose
| time came are sent.
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
	a.Schedule.Job(jobs.CheckIntegrationHealthArgs{}).Hourly()
	a.Schedule.Job(jobs.ScheduleWorkspacePurgesArgs{}).DailyAt("03:00")
	a.Schedule.Job(jobs.PruneAuditLogsArgs{}).DailyAt("03:10")
	a.Schedule.Job(jobs.PruneSecurityEventsArgs{}).DailyAt("03:20")
	a.Schedule.Job(jobs.PurgeStaleDraftsArgs{}).DailyAt("03:30")
	a.Schedule.Job(jobs.PruneEmailAddressTestsArgs{}).DailyAt("03:40")
	a.Schedule.Job(jobs.CheckWorkspaceOwnersArgs{}).DailyAt("04:00")
	a.Schedule.Job(jobs.SendWorkspaceDeletionRemindersArgs{}).DailyAt("09:00")
	a.Schedule.Job(jobs.NotifyExpiringTokensArgs{}).DailyAt("09:05")
	a.Schedule.Job(jobs.RecheckOrganizationDomainsArgs{}).DailyAt("05:00")
	a.Schedule.Job(jobs.RefreshSSOMetadataArgs{}).DailyAt("05:10")
	a.Schedule.Job(jobs.WarnExpiringSSOCertificatesArgs{}).DailyAt("09:15")
	a.Schedule.Job(jobs.SweepAssignmentQueuesArgs{}).EveryMinute()
	a.Schedule.Job(jobs.SendNotificationDigestsArgs{}).EveryMinute()
	a.Schedule.Job(jobs.RemindInvitersArgs{}).DailyAt("09:10")
	a.Schedule.Job(jobs.ScheduleEmailDomainChecksArgs{}).EveryMinute()
	a.Schedule.Job(jobs.ExpireContactExportsArgs{}).Hourly()
	a.Schedule.Job(jobs.ExpireDataExportsArgs{}).Hourly()
	a.Schedule.Job(jobs.ApplyConversationRetentionArgs{}).DailyAt("02:30")
	a.Schedule.Job(jobs.PruneAiTracesArgs{}).DailyAt("03:50")
	a.Schedule.Job(jobs.PruneChannelInboundEventsArgs{}).DailyAt("03:45")
	a.Schedule.Job(jobs.RefreshIdentityKeySetsArgs{}).Hourly()
	a.Schedule.Job(jobs.PruneIdentityStatsArgs{}).DailyAt("03:55")
	a.Schedule.Job(jobs.SweepKnowledgeEmbeddingsArgs{}).DailyAt("04:10")
}
