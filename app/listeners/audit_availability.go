/*
|--------------------------------------------------------------------------
| Audit Log: Availability
|--------------------------------------------------------------------------
|
| Going away and coming back is everyday work and stays out of the audit
| log. Someone else changing a member's availability, or how many
| conversations they hold, is a decision about that member and is kept.
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
listenAvailability registers the entry of availability changes.
*/
func (l *WriteAuditLog) listenAvailability(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.memberAvailabilityChanged)
}

/*
memberAvailabilityChanged records a change of a member's availability
made by someone else.
*/
func (l *WriteAuditLog) memberAvailabilityChanged(ctx context.Context, e appevents.MemberAvailabilityChanged) error {
	if e.ChangedBy == e.MemberID {
		return nil
	}

	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventMemberAvailability, ActorID: e.ChangedBy,
		Changes: map[string]any{"availability": map[string]any{
			"from": auditedAvailability(e.From), "to": auditedAvailability(e.To),
		}},
	})
}

/*
auditedAvailability is how the audit log writes one state of a member's
availability.
*/
func auditedAvailability(s appevents.AvailabilityState) map[string]any {
	return map[string]any{
		"availability": s.Availability, "away_reassign": s.AwayReassign, "assign_on_reply": s.AssignOnReply,
		"conversation_capacity": s.Capacity, "capacity_by_channel": s.CapacityByChannel,
	}
}
