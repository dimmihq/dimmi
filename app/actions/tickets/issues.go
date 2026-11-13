/*
|--------------------------------------------------------------------------
| Tickets and Engineering Tools
|--------------------------------------------------------------------------
|
| What the linked-issue sync needs from tickets, and nothing more: a
| writer acting as a tool, such as Linear, so its moves, notes and
| public updates carry the tool's name in the history; a ticket found
| on a member's terms or locked for a change; a link logged and
| announced; and a message to everyone linked, planned with the same
| holds a teammate's message gets.
|
*/

package tickets

import (
	"context"
	"maps"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AsTool runs the work in one transaction of the workspace with a writer
acting as the named tool.
*/
func (t *Tickets) AsTool(ctx context.Context, ws ids.UUID, tool string, fn func(ctx context.Context, w *Writer) error,
) error {
	ctx = tenancy.With(ctx, ws.String())

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, ws)
		if err != nil {
			return err
		}

		w.Actor = conversations.Actor{Type: w.Actor.Type, Name: tool}

		return fn(ctx, w)
	})
}

/*
AsMember runs the work in one transaction with a writer acting as the
member.
*/
func (t *Tickets) AsMember(ctx context.Context, m authz.Membership, fn func(ctx context.Context, w *Writer) error,
) error {
	return t.write(ctx, m, fn)
}

/*
Find returns the live ticket the reference names, when the member may
see it, within the writer's transaction.
*/
func (w *Writer) Find(ctx context.Context, m authz.Membership, ref Ref) (models.Ticket, error) {
	return find(ctx, w.Q, m, ref)
}

/*
MayRead refuses a member without tickets:read.
*/
func MayRead(m authz.Membership) error { return canRead(m) }

/*
MayWrite refuses a member without tickets:write.
*/
func MayWrite(m authz.Membership) error { return canWrite(m) }

/*
IssueChange is a linked issue's change for the ticket's history: the
link, linked for a new one, and what the history keeps.
*/
type IssueChange struct {
	Issue  models.TicketIssue
	Linked bool
	Data   map[string]any
}

/*
AnnounceIssue logs the issue's change in the ticket's history and sends
ticket.issue_linked or ticket.issue_updated. Changes carries the status
group under status_group, from what Data names as from_group, and
deployed when a deployment shipped the fix, for workflows to read.
*/
func (w *Writer) AnnounceIssue(ctx context.Context, t models.Ticket, c IssueChange) error {
	data := map[string]any{
		"issue_id": models.PublicID(models.PrefixTicketIssue, c.Issue.ID), "provider": c.Issue.Provider,
		"key": c.Issue.Key, "url": c.Issue.Url, "status": c.Issue.Status, "status_group": c.Issue.StatusGroup,
	}
	maps.Copy(data, c.Data)

	kind := "issue_updated"
	if c.Linked {
		kind = "issue_linked"
	}

	if err := w.log(ctx, t, kind, data); err != nil {
		return err
	}

	changes := map[string]appevents.TicketChange{
		"issue": change(nil, data), "status_group": change(c.Data["from_group"], c.Issue.StatusGroup),
	}
	if deployed, ok := c.Data["deployed"]; ok {
		changes["deployed"] = change(nil, deployed)
	}
	if c.Linked {
		return w.emit(ctx, appevents.TicketIssueLinked{TicketChanged: w.changed(t, changes)})
	}

	return w.emit(ctx, appevents.TicketIssueUpdated{TicketChanged: w.changed(t, changes)})
}

/*
PlanMessage plans a message to everyone the tracker links:
a dry run counts who it reaches and who is held and why; otherwise the
message is stored and queued.
*/
func (w *Writer) PlanMessage(ctx context.Context, t models.Ticket, in UpdateInput) (Plan, error) {
	if t.Kind != KindTracker {
		return Plan{}, ErrNotTracker
	}

	return w.planUpdate(ctx, t, in)
}
