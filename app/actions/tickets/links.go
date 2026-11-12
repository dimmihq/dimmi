/*
|--------------------------------------------------------------------------
| Links
|--------------------------------------------------------------------------
|
| A customer ticket links to the back-office work behind it; escalating
| makes that work and moves the customer ticket to a state that waits on
| the team, so the customer sees "with our billing team" while the
| agent's queue stays clean. When the back-office work resolves, the
| customer ticket comes back to the team marked "close the loop", or,
| when its type sends a prepared reply, resolves itself with it.
|
| Trackers and incidents link to conversations; a conversation may join
| several. Follow-up and related links join any two tickets. Linking
| twice is a no-op.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The link kinds.
*/
const (
	LinkBackOffice = "back_office"
	LinkTracker    = "tracker"
	LinkFollowUp   = "follow_up"
	LinkRelated    = "related"
)

/*
LinkKinds returns every link kind.
*/
func LinkKinds() []string {
	return []string{LinkBackOffice, LinkTracker, LinkFollowUp, LinkRelated}
}

/*
LinkInput is a link from a ticket to one other ticket or one
conversation.
*/
type LinkInput struct {
	Kind           string
	TicketID       *ids.UUID
	ConversationID *ids.UUID
}

/*
Link joins the ticket to the target, and returns the link with whether
it is new.
*/
func (w *Writer) Link(ctx context.Context, from models.Ticket, in LinkInput) (models.TicketLink, error) {
	if !slices.Contains(LinkKinds(), in.Kind) || (in.TicketID == nil) == (in.ConversationID == nil) {
		return models.TicketLink{}, ErrLinkInvalid
	}

	if err := w.checkLink(ctx, from, in); err != nil {
		return models.TicketLink{}, err
	}

	var member *ids.UUID
	if w.Actor.Type == string(enums.AuthorTypeMember) {
		member = w.actorID()
	}

	link, err := w.Q.CreateTicketLink(ctx, models.CreateTicketLinkParams{
		WorkspaceID: from.WorkspaceID, Kind: in.Kind, FromTicketID: from.ID, ToTicketID: in.TicketID,
		ConversationID: in.ConversationID, CreatedByMemberID: member, Now: w.Now,
	})
	if err != nil {
		return link, ignoreExisting(err)
	}

	return link, w.announceLink(ctx, from, link)
}

/*
checkLink refuses links that do not fit their tickets: a tracker link
goes from a tracker or incident to a conversation, a back-office link
from a customer ticket to back-office work, and nothing links to
itself.
*/
func (w *Writer) checkLink(ctx context.Context, from models.Ticket, in LinkInput) error {
	switch in.Kind {
	case LinkTracker:
		if (from.Kind != KindTracker && from.Kind != KindIncident) || in.ConversationID == nil {
			return ErrLinkInvalid
		}

		return w.checkConversation(ctx, from.WorkspaceID, *in.ConversationID)
	case LinkBackOffice:
		if from.Kind != KindCustomer || in.TicketID == nil {
			return ErrLinkInvalid
		}
	}

	if in.TicketID == nil {
		return w.checkConversation(ctx, from.WorkspaceID, *in.ConversationID)
	}

	if *in.TicketID == from.ID {
		return ErrLinkInvalid
	}

	to, err := w.Q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: from.WorkspaceID, ID: *in.TicketID})
	if err != nil || to.DeletedAt != nil {
		return ErrTicketNotFound.OnField("ticket_id")
	}

	if in.Kind == LinkBackOffice && to.Kind != KindBackOffice {
		return ErrLinkInvalid
	}

	return nil
}

/*
checkConversation refuses a conversation that is not live.
*/
func (w *Writer) checkConversation(ctx context.Context, ws, id ids.UUID) error {
	c, err := w.Q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: id})
	if err != nil || c.DeletedAt != nil {
		return ErrLinkInvalid.OnField("conversation_id")
	}

	return nil
}

/*
ignoreExisting reads a link that already existed as no change.
*/
func ignoreExisting(err error) error {
	if isNoRows(err) {
		return nil
	}

	return fmt.Errorf("link ticket: %w", err)
}

/*
announceLink logs and sends a new link.
*/
func (w *Writer) announceLink(ctx context.Context, from models.Ticket, l models.TicketLink) error {
	data := linkData(l)
	if err := w.log(ctx, from, "link", data); err != nil {
		return err
	}

	changes := map[string]appevents.TicketChange{"link": change(nil, data)}

	return w.emit(ctx, appevents.TicketLinked{TicketChanged: w.changed(from, changes)})
}

/*
linkData describes a link for the log and events.
*/
func linkData(l models.TicketLink) map[string]any {
	out := map[string]any{"id": models.PublicID(models.PrefixTicketLink, l.ID), "kind": l.Kind}
	if l.ToTicketID != nil {
		out["ticket_id"] = models.PublicID(models.PrefixTicket, *l.ToTicketID)
	}

	if l.ConversationID != nil {
		out["conversation_id"] = models.PublicID(models.PrefixConversation, *l.ConversationID)
	}

	return out
}

/*
Unlink removes a link and announces it on the ticket it came from.
*/
func (w *Writer) Unlink(ctx context.Context, l models.TicketLink) error {
	from, err := w.Q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: l.WorkspaceID, ID: l.FromTicketID})
	if err != nil {
		return notFound(err)
	}

	err = w.Q.DeleteTicketLink(ctx, models.DeleteTicketLinkParams{WorkspaceID: l.WorkspaceID, ID: l.ID})
	if err != nil {
		return fmt.Errorf("unlink ticket: %w", err)
	}

	data := linkData(l)
	if err := w.log(ctx, from, "unlink", data); err != nil {
		return err
	}

	changes := map[string]appevents.TicketChange{"link": change(data, nil)}

	return w.emit(ctx, appevents.TicketUnlinked{TicketChanged: w.changed(from, changes)})
}

/*
EscalateInput is back-office work made for a customer ticket.
*/
type EscalateInput struct {
	TypeID      ids.UUID
	Title       string
	Description string
	Fields      map[string]any
	TeamID      *ids.UUID
	Assignee    *ids.UUID
}

/*
Escalate makes back-office work for the customer ticket and moves the
customer ticket to its type's first state that waits on the team.
*/
func (w *Writer) Escalate(ctx context.Context, t models.Ticket, in EscalateInput) (models.Ticket, error) {
	if t.Kind != KindCustomer {
		return models.Ticket{}, ErrLinkInvalid
	}

	ty, ok := w.Setup.Type(in.TypeID)
	if !ok || ty.Kind != KindBackOffice {
		return models.Ticket{}, ErrKindMismatch
	}

	made, err := w.Create(ctx, t.WorkspaceID, CreateInput{
		TypeID: ty.ID, Title: in.Title, Description: in.Description, Fields: in.Fields, ContactID: t.ContactID,
		CompanyID: t.CompanyID, AssigneeTeam: in.TeamID, AssigneeMember: in.Assignee,
	})
	if err != nil {
		return models.Ticket{}, err
	}

	if _, err := w.Link(ctx, t, LinkInput{Kind: LinkBackOffice, TicketID: &made.Ticket.ID}); err != nil {
		return models.Ticket{}, err
	}

	if waiting, ok := w.Setup.FirstOf(t.TypeID, GroupWaitingTeam); ok {
		if _, err := w.move(ctx, t, waiting); err != nil {
			return models.Ticket{}, err
		}
	}

	return made.Ticket, nil
}

/*
returnToTeam brings a customer ticket back once the back-office work
behind it resolved: to "close the loop" with the work's resolution. As
in Plain, nothing is sent to the customer here; a workflow on the
close-the-loop trigger, or a teammate, tells them.
*/
func (w *Writer) returnToTeam(ctx context.Context, customerID ids.UUID, work models.Ticket) error {
	t, err := w.lock(ctx, work.WorkspaceID, customerID)
	if err != nil || t.DeletedAt != nil || t.StateGroup == GroupResolved {
		return nil //nolint:nilerr // a customer ticket gone or done needs no loop closed
	}

	target, ok := w.Setup.StateByPurpose(PurposeCloseLoop)
	if !ok || !w.Setup.Allows(t.TypeID, target.ID) {
		target, ok = w.Setup.FirstOf(t.TypeID, GroupInProgress)
	}

	if !ok {
		return nil
	}

	moved, err := w.move(ctx, t, target)
	if err != nil {
		return err
	}

	return w.log(ctx, moved.ticket, "close_loop", map[string]any{
		"ticket_id": models.PublicID(models.PrefixTicket, work.ID), "reference": Reference(work.Number),
		"title": work.Title,
	})
}
