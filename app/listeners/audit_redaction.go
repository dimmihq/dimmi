/*
|--------------------------------------------------------------------------
| Audit Log: Redaction
|--------------------------------------------------------------------------
|
| Redaction rules decide what every later message keeps, so adding,
| changing and removing one, and switching the built-in rules, are kept
| in the audit log. A rule is named as it was called at the time; its
| pattern is not copied into the log.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
The subjects of redaction entries: a rule, or the workspace's switches.
*/
const (
	subjectRedactionRule     = "redaction_rule"
	subjectRedactionSettings = "redaction_settings"
)

/*
listenRedaction registers the entries of redaction rules.
*/
func (l *WriteAuditLog) listenRedaction(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.redactionRuleCreated)
	events.Listen(d, events.InTx, l.redactionRuleUpdated)
	events.Listen(d, events.InTx, l.redactionRuleDeleted)
	events.Listen(d, events.InTx, l.redactionSettingsUpdated)
}

/*
redactionRuleCreated records a new rule.
*/
func (l *WriteAuditLog) redactionRuleCreated(ctx context.Context, e appevents.RedactionRuleCreated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRedactionRuleCreated, ActorID: e.Actor.ID,
		Subject: audit.Subject{Type: subjectRedactionRule, ID: e.RuleID, Label: e.Name},
	})
}

/*
redactionRuleUpdated records which fields of a rule changed.
*/
func (l *WriteAuditLog) redactionRuleUpdated(ctx context.Context, e appevents.RedactionRuleUpdated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRedactionRuleUpdated, ActorID: e.Actor.ID,
		Subject: audit.Subject{Type: subjectRedactionRule, ID: e.RuleID, Label: e.Name},
		Changes: map[string]any{"fields": e.Changes},
	})
}

/*
redactionRuleDeleted records a removed rule.
*/
func (l *WriteAuditLog) redactionRuleDeleted(ctx context.Context, e appevents.RedactionRuleDeleted) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRedactionRuleDeleted, ActorID: e.Actor.ID,
		Subject: audit.Subject{Type: subjectRedactionRule, ID: e.RuleID, Label: e.Name},
	})
}

/*
redactionSettingsUpdated records the built-in rules switched or the
rules reordered.
*/
func (l *WriteAuditLog) redactionSettingsUpdated(ctx context.Context, e appevents.RedactionSettingsUpdated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRedactionSettingsUpdated, ActorID: e.Actor.ID,
		Subject: audit.Subject{Type: subjectRedactionSettings, ID: e.WorkspaceID, Label: "Redaction"},
		Changes: e.Changes,
	})
}
