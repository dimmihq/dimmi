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
| filled and models out of use for a week deleted. Every five minutes the
| knowledge sources whose sync is due are started.
| In the morning owners of workspaces about to be purged are reminded,
| and owners of access tokens about to expire are warned. Every hour the
| auth provider key sets visitors sign in with are fetched again when
| their copy is six hours old. Every minute the Slack digests whose
| time came are sent. Every minute webhook deliveries due a retry are
| queued, every five minutes failing webhook endpoints are checked, and
| every night the webhook delivery log past its retention is pruned.
| Report facts are refreshed every few seconds, each workspace's backlog
| is counted hourly once its day ends, and report digests go out every
| five minutes as they come due.
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
	a.Schedule.Job(jobs.SweepBroadcastsArgs{}).Hourly()
	a.Schedule.Job(jobs.SweepSeriesArgs{}).Cron("*/15 * * * *")
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
	a.Schedule.Job(jobs.RemindInvitersArgs{}).DailyAt("09:10")
	a.Schedule.Job(jobs.ScheduleEmailDomainChecksArgs{}).EveryMinute()
	a.Schedule.Job(jobs.ExpireContactExportsArgs{}).Hourly()
	a.Schedule.Job(jobs.ExpireDataExportsArgs{}).Hourly()
	a.Schedule.Job(jobs.ApplyConversationRetentionArgs{}).DailyAt("02:30")
	a.Schedule.Job(jobs.PruneAiTracesArgs{}).DailyAt("03:50")
	a.Schedule.Job(jobs.PruneChannelInboundEventsArgs{}).DailyAt("03:45")
	a.Schedule.Job(jobs.RefreshIdentityKeySetsArgs{}).Hourly()
	a.Schedule.Job(jobs.PruneIdentityStatsArgs{}).DailyAt("03:55")
	a.Schedule.Job(jobs.SweepAttachmentScansArgs{}).Cron("*/5 * * * *")
	scheduleReports(a)
	scheduleKnowledge(a)
	scheduleWebhooks(a)
	scheduleWork(a)
}

/*
scheduleWork runs the notification digests and the sweeps of imports and
tickets.
*/
func scheduleWork(a *app.App) {
	scheduleNotifications(a)
	scheduleImports(a)
	scheduleTickets(a)
}

/*
scheduleTickets resolves tickets that waited on their customer too
long, every fifteen minutes, and every minute warns about and breaches
service level clocks. Linked issues not checked for fifteen minutes are
read again every five, in case their tool's webhook was missed.
*/
func scheduleTickets(a *app.App) {
	a.Schedule.Job(jobs.SweepTicketsArgs{}).Cron("*/15 * * * *")
	a.Schedule.Job(jobs.SweepSLAClocksArgs{}).EveryMinute()
	a.Schedule.Job(jobs.SyncTicketIssuesArgs{}).Cron("*/5 * * * *")
}

/*
scheduleImports empties import files a month old and drops what can no
longer be undone.
*/
func scheduleImports(a *app.App) {
	a.Schedule.Job(jobs.ExpireImportsArgs{}).DailyAt("04:30")
}

/*
scheduleWebhooks retries failed deliveries, checks endpoints' health and
prunes the delivery log.
*/
func scheduleWebhooks(a *app.App) {
	a.Schedule.Job(jobs.SweepWebhookDeliveriesArgs{}).EveryMinute()
	a.Schedule.Job(jobs.CheckWebhookEndpointsArgs{}).Cron("*/5 * * * *")
	a.Schedule.Job(jobs.PruneWebhookDeliveriesArgs{}).DailyAt("03:35")
}

/*
scheduleNotifications declares the teammate notification jobs: the
Slack digests, the quiet hours summaries and the delivery log's prune.
*/
func scheduleNotifications(a *app.App) {
	a.Schedule.Job(jobs.SendNotificationDigestsArgs{}).EveryMinute()
	a.Schedule.Job(jobs.SendQuietHoursSummariesArgs{}).Cron("*/5 * * * *")
	a.Schedule.Job(jobs.PruneNotificationDeliveriesArgs{}).DailyAt("03:25")
}

/*
scheduleKnowledge declares the knowledge and help center jobs: the
vector sweep, the source syncs that are due, the custom domain checks
that are due, and the prune of old view and search counts.
*/
func scheduleKnowledge(a *app.App) {
	a.Schedule.Job(jobs.SweepKnowledgeEmbeddingsArgs{}).DailyAt("04:10")
	a.Schedule.Job(jobs.ScheduleKnowledgeSyncsArgs{}).Cron("*/5 * * * *")
	a.Schedule.Job(jobs.ScheduleHelpCenterDomainChecksArgs{}).EveryMinute()
	a.Schedule.Job(jobs.PruneHelpAnalyticsArgs{}).DailyAt("04:20")
}

/*
scheduleReports restarts the report refresh every minute should it have
stopped, counts each workspace's backlog once its day ends, and sends
the report digests that are due.
*/
func scheduleReports(a *app.App) {
	a.Schedule.Job(jobs.RefreshReportsArgs{}).EveryMinute()
	a.Schedule.Job(jobs.SnapshotBacklogsArgs{}).Cron("5 * * * *")
	a.Schedule.Job(jobs.SendReportDigestsArgs{}).Cron("*/5 * * * *")
}
