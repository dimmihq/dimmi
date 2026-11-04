/*
|--------------------------------------------------------------------------
| Audit Wiring
|--------------------------------------------------------------------------
|
| The audit log and the security log: the one writer of audit entries,
| the listener that records workspace and member events through it, the
| queued listener that copies members' security events into it, the
| actions that read both logs back and export the audit log, and the
| jobs that prune them.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/listeners"
)

/*
bootAudit builds the audit actions, listens for the events the audit
log records, and registers the prune jobs' workers.
*/
func bootAudit(a *app.App) error {
	acts := a.Actions
	acts.RecordAuditLog = &audit.RecordAuditLog{DB: a.DB, Clock: a.Clock}
	acts.ListAuditLogs = &audit.ListAuditLogs{DB: a.DB}
	acts.ShowAuditLog = &audit.ShowAuditLog{DB: a.DB}
	acts.ExportAuditLogs = &audit.ExportAuditLogs{
		List: acts.ListAuditLogs, Record: acts.RecordAuditLog, Limiter: a.Limiter, Clock: a.Clock,
	}
	acts.ListSecurityEvents = &auth.ListSecurityEvents{DB: a.DB}

	writer := &listeners.WriteAuditLog{DB: a.DB, Record: acts.RecordAuditLog}
	writer.Listen(a.Events)

	mirror := &listeners.MirrorSecurityEventToAudit{DB: a.DB, Record: acts.RecordAuditLog}
	mirror.Listen(a.Events)

	return errors.Join(
		queue.Register(a.Queue, mirror),
		queue.Register(a.Queue, &jobs.PruneAuditLogs{
			DB: a.DB, Clock: a.Clock, RetentionDays: auditRetention(a), Retention: a.Retention,
		}),
		queue.Register(a.Queue, &jobs.PruneSecurityEvents{
			DB: a.DB, Clock: a.Clock, RetentionDays: a.Config.SecurityEvents.RetentionDays,
		}),
	)
}

/*
auditRetention returns the days audit entries are kept, 0 for ever.
*/
func auditRetention(a *app.App) int {
	if a.Config.Audit.RetentionDays == nil {
		return 0
	}

	return *a.Config.Audit.RetentionDays
}
