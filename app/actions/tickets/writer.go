/*
|--------------------------------------------------------------------------
| Ticket Writer
|--------------------------------------------------------------------------
|
| Every change to a ticket is written one way: the row changes and its
| version goes up, an event row joins the ticket's log with who did it,
| and the domain event goes out, all in the transaction of the change.
| The writer wraps a conversation writer, so a workflow's chain, the
| caller's mutation id and the actor carry over to both, and a customer
| ticket's conversation changes in the same step.
|
*/

package tickets

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Writer writes ticket changes within one transaction. Setup is the
workspace's configuration, read once.
*/
type Writer struct {
	*conversations.Writer
	Setup Setup
	Owner *Tickets
}

/*
writerFor wraps a conversation writer, reading the workspace's setup.
*/
func (t *Tickets) writerFor(ctx context.Context, cw *conversations.Writer, ws ids.UUID) (*Writer, error) {
	setup, err := loadSetup(ctx, cw.Q, ws, cw.Now)
	if err != nil {
		return nil, err
	}

	return &Writer{Writer: cw, Setup: setup, Owner: t}, nil
}

/*
Wrap returns a ticket writer around another action's conversation
writer, such as a workflow's, so its changes carry that actor and chain.
*/
func (t *Tickets) Wrap(ctx context.Context, cw *conversations.Writer, ws ids.UUID) (*Writer, error) {
	return t.writerFor(ctx, cw, ws)
}

/*
actorType returns the type of the actor for the log, system when the
writer acts for nobody.
*/
func (w *Writer) actorType() string {
	if w.Actor.Type == "" {
		return "system"
	}

	return w.Actor.Type
}

/*
actorID returns the actor's id for the log, nil for the system.
*/
func (w *Writer) actorID() *ids.UUID {
	if w.Actor.ID == ids.Nil() {
		return nil
	}

	id := w.Actor.ID

	return &id
}

/*
log appends one change to the ticket's history.
*/
func (w *Writer) log(ctx context.Context, t models.Ticket, kind string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}

	if w.Actor.Name != "" {
		data["actor_name"] = w.Actor.Name
	}

	_, err := w.Q.CreateTicketEvent(ctx, models.CreateTicketEventParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, Kind: kind, ActorType: w.actorType(), ActorID: w.actorID(),
		Data: rawObject(data), Now: w.Now,
	})
	if err != nil {
		return fmt.Errorf("log ticket %s: %w", kind, err)
	}

	return nil
}

/*
changed returns the shared shape of a ticket event.
*/
func (w *Writer) changed(t models.Ticket, changes map[string]appevents.TicketChange) appevents.TicketChanged {
	return appevents.TicketChanged{
		Base:      w.Base(t.WorkspaceID),
		TicketRef: appevents.TicketRef{TicketID: t.ID, Kind: t.Kind, ConversationID: t.ConversationID},
		Changes:   changes,
	}
}

/*
emit dispatches an event within the transaction.
*/
func (w *Writer) emit(ctx context.Context, e events.Event) error {
	return w.Dispatch(ctx, e)
}

/*
change is one moved value, from and to.
*/
func change(from, to any) appevents.TicketChange {
	return appevents.TicketChange{From: from, To: to}
}

/*
save writes the ticket row, bumping its version.
*/
func (w *Writer) save(ctx context.Context, t models.Ticket) (models.Ticket, error) {
	saved, err := w.Q.SaveTicket(ctx, models.SaveTicketParams{
		TypeID: t.TypeID, StateID: t.StateID, StateGroup: t.StateGroup, Title: t.Title, Description: t.Description,
		Fields: t.Fields, Suggestions: t.Suggestions, Priority: t.Priority, AssigneeMemberID: t.AssigneeMemberID,
		AssigneeTeamID: t.AssigneeTeamID, InboxID: t.InboxID, ContactID: t.ContactID, CompanyID: t.CompanyID,
		SlaPolicyID: t.SlaPolicyID, CloseLoop: t.CloseLoop, StateSince: t.StateSince, ResolvedAt: t.ResolvedAt,
		AutoResolveWarnedAt: t.AutoResolveWarnedAt, ConversationID: t.ConversationID, Now: w.Now,
		WorkspaceID: t.WorkspaceID, ID: t.ID,
	})
	if err != nil {
		return t, fmt.Errorf("save ticket: %w", err)
	}

	return saved, nil
}

/*
lockRow reads the ticket and holds its row until the transaction ends.
*/
func (w *Writer) lockRow(ctx context.Context, ws, id ids.UUID) (models.Ticket, error) {
	t, err := w.Q.LockTicket(ctx, models.LockTicketParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return t, notFound(err)
	}

	return t, nil
}
