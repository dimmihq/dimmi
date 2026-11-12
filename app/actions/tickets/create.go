/*
|--------------------------------------------------------------------------
| Creating Tickets
|--------------------------------------------------------------------------
|
| A ticket is created with a type and a title, its fields checked as the
| caller's form requires: the customer's form may ask fewer questions
| than the team's. A create that carries an external id the workspace
| already used returns the ticket made the first time, so a retried
| create never makes a second ticket. A customer ticket is tied to its
| conversation; given only a contact, the conversation starts with the
| ticket's description as the first message.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Priorities returns the priorities a ticket may have.
*/
func Priorities() []string {
	return []string{"none", "low", "medium", "high", "urgent"}
}

/*
CreateInput is a new ticket. StateID is where it starts, the type's
default when nil. Customer says the customer filled the form, which
checks their own required fields. ConversationID ties a customer ticket
to an existing conversation; ContactID starts one for it otherwise.
Links are made in the same step.
*/
type CreateInput struct {
	TypeID         ids.UUID
	StateID        *ids.UUID
	Title          string
	Description    string
	Fields         map[string]any
	Suggestions    map[string]any
	ContactID      *ids.UUID
	CompanyID      *ids.UUID
	ConversationID *ids.UUID
	AssigneeMember *ids.UUID
	AssigneeTeam   *ids.UUID
	InboxID        *ids.UUID
	Priority       string
	ExternalID     *string
	SLAPolicyID    *ids.UUID
	Customer       bool
	AIFill         bool
	Links          []LinkInput
}

/*
Created is a ticket a create returned and whether it is new, false when
the external id found the first one.
*/
type Created struct {
	Ticket models.Ticket
	New    bool
}

/*
Create makes a ticket within the writer's transaction.
*/
func (w *Writer) Create(ctx context.Context, ws ids.UUID, in CreateInput) (Created, error) {
	if in.ExternalID != nil {
		found, err := w.Q.FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{
			WorkspaceID: ws, ExternalID: in.ExternalID,
		})
		if err == nil {
			return Created{Ticket: found}, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return Created{}, fmt.Errorf("find ticket by external id: %w", err)
		}
	}

	plan, err := w.plan(ctx, ws, in)
	if err != nil {
		return Created{}, err
	}

	t, err := w.insert(ctx, ws, plan)
	if err != nil {
		return Created{}, err
	}

	return Created{Ticket: t, New: true}, w.afterCreate(ctx, t, in)
}

/*
planned is a create checked and filled in: its type, starting state,
values and inbox.
*/
type planned struct {
	in          CreateInput
	kind        models.TicketType
	state       models.TicketState
	values      map[string]any
	suggestions map[string]any
	inbox       ids.UUID
}

/*
plan checks a create and fills in what it left out.
*/
func (w *Writer) plan(ctx context.Context, ws ids.UUID, in CreateInput) (planned, error) {
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		return planned{}, ErrTitleInvalid
	}

	ty, ok := w.Setup.Type(in.TypeID)
	if !ok {
		return planned{}, ErrTypeNotFound
	}

	if ty.ArchivedAt != nil {
		return planned{}, ErrTypeArchived
	}

	state, err := w.startState(ty, in.StateID)
	if err != nil {
		return planned{}, err
	}

	fields := w.Setup.FieldsOf(ty.ID)

	values, problems := merge(fields, nil, in.Fields)
	if len(problems) > 0 {
		return planned{}, invalidError(problems)
	}

	suggestions := autoFill(ty, fields, values, in.Suggestions)

	if missing := missingFor(fields, values, requiredFor(in)); len(missing) > 0 {
		return planned{}, missingError(missing)
	}

	p := planned{in: in, kind: ty, state: state, values: values, suggestions: suggestions}

	return w.placeIn(ctx, ws, p)
}

/*
startState returns the state a new ticket of the type starts in.
*/
func (w *Writer) startState(ty models.TicketType, asked *ids.UUID) (models.TicketState, error) {
	if asked == nil {
		st, ok := w.Setup.DefaultState(ty.ID)
		if !ok {
			return st, ErrStateNotAllowed
		}

		return st, nil
	}

	st, ok := w.Setup.State(*asked)
	if !ok {
		return st, ErrStateNotFound
	}

	if !w.Setup.Allows(ty.ID, st.ID) {
		return st, ErrStateNotAllowed
	}

	return st, nil
}

/*
placeIn finds the ticket's inbox, its customer's company and, for a
customer ticket, its conversation.
*/
func (w *Writer) placeIn(ctx context.Context, ws ids.UUID, p planned) (planned, error) {
	if p.kind.Kind == KindCustomer {
		if p.in.ConversationID == nil && p.in.ContactID == nil {
			return p, ErrContactRequired
		}

		if err := w.conversationOf(ctx, ws, &p); err != nil {
			return p, err
		}
	}

	if p.in.ContactID != nil && p.in.CompanyID == nil {
		contact, err := w.Q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *p.in.ContactID})
		if err != nil {
			return p, fmt.Errorf("find ticket contact: %w", err)
		}

		p.in.CompanyID = contact.CompanyID
	}

	inbox, err := w.inboxOf(ctx, ws, p)
	if err != nil {
		return p, err
	}

	p.inbox = inbox

	return p, nil
}

/*
conversationOf checks the conversation a customer ticket is tied to,
taking its customer and inbox, and refusing one that is a ticket.
*/
func (w *Writer) conversationOf(ctx context.Context, ws ids.UUID, p *planned) error {
	if p.in.ConversationID == nil {
		return nil
	}

	c, err := w.Q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: *p.in.ConversationID})
	if err != nil || c.DeletedAt != nil {
		return ErrLinkInvalid.OnField("conversation_id")
	}

	_, err = w.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: ws, ConversationID: &c.ID,
	})
	if err == nil {
		return ErrAlreadyTicket
	}

	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find conversation ticket: %w", err)
	}

	p.in.ContactID = &c.ContactID
	if p.in.InboxID == nil {
		p.in.InboxID = &c.InboxID
	}

	if p.in.AssigneeMember == nil && p.in.AssigneeTeam == nil {
		p.in.AssigneeMember, p.in.AssigneeTeam = c.AssigneeMemberID, c.AssigneeTeamID
	}

	return nil
}

/*
inboxOf returns the inbox a new ticket lands in: the one asked for, its
type's, or the workspace's first.
*/
func (w *Writer) inboxOf(ctx context.Context, ws ids.UUID, p planned) (ids.UUID, error) {
	switch {
	case p.in.InboxID != nil:
		return *p.in.InboxID, nil
	case p.kind.InboxID != nil:
		return *p.kind.InboxID, nil
	}

	inboxes, err := w.Q.ListInboxes(ctx, ws)
	if err != nil {
		return ids.Nil(), fmt.Errorf("list inboxes: %w", err)
	}

	if len(inboxes) == 0 {
		return ids.Nil(), ErrLinkInvalid.OnField("inbox_id")
	}

	return inboxes[0].ID, nil
}

/*
insert stores the planned ticket under the workspace's next number.
*/
func (w *Writer) insert(ctx context.Context, ws ids.UUID, p planned) (models.Ticket, error) {
	number, err := w.Q.NextTicketNumber(ctx, ws)
	if err != nil {
		return models.Ticket{}, fmt.Errorf("number ticket: %w", err)
	}

	team := p.in.AssigneeTeam
	if team == nil && p.in.AssigneeMember == nil {
		team = p.kind.TeamID
	}

	priority := p.in.Priority
	if priority == "" {
		priority = "none"
	}

	t, err := w.Q.CreateTicket(ctx, models.CreateTicketParams{
		WorkspaceID: ws, Number: number, Kind: p.kind.Kind, TypeID: p.kind.ID, StateID: p.state.ID,
		StateGroup: p.state.StateGroup, ConversationID: p.in.ConversationID, ContactID: p.in.ContactID,
		CompanyID: p.in.CompanyID, InboxID: p.inbox, Title: p.in.Title, Description: p.in.Description,
		Fields: rawObject(p.values), Suggestions: rawObject(p.suggestions), Priority: priority,
		AssigneeMemberID: p.in.AssigneeMember, AssigneeTeamID: team, ExternalID: p.in.ExternalID,
		SlaPolicyID: p.in.SLAPolicyID, CustomerVisible: visibleToCustomer(p.kind),
		CreatedByType: w.creatorType(), CreatedByID: w.actorID(), ResolvedAt: resolvedAt(p.state, w),
		Now: w.Now,
	})
	if err != nil {
		return t, fmt.Errorf("create ticket: %w", err)
	}

	return t, nil
}

/*
resolvedAt is when a ticket created resolved was resolved: now.
*/
func resolvedAt(st models.TicketState, w *Writer) *time.Time {
	if st.StateGroup != GroupResolved {
		return nil
	}

	now := w.Now

	return &now
}

/*
visibleToCustomer says whether customers see tickets of the type: a
customer ticket always, an incident when its type says so, other work
never.
*/
func visibleToCustomer(ty models.TicketType) bool {
	return ty.Kind == KindCustomer || (ty.Kind == KindIncident && ty.CustomerVisible)
}

/*
creatorType is the actor type a ticket records as its creator.
*/
func (w *Writer) creatorType() string {
	switch w.Actor.Type {
	case string(enums.AuthorTypeMember), string(enums.AuthorTypeContact), "agent", "integration":
		return w.Actor.Type
	}

	if len(w.Chain) > 0 {
		return "workflow"
	}

	return "system"
}

/*
afterCreate logs the create, makes the links asked for and announces
the ticket.
*/
func (w *Writer) afterCreate(ctx context.Context, t models.Ticket, in CreateInput) error {
	st, _ := w.Setup.State(t.StateID)

	err := w.log(ctx, t, "created", map[string]any{
		"to": st.ID.String(), "to_name": st.Name, "to_group": st.StateGroup, "type_id": t.TypeID.String(),
	})
	if err != nil {
		return err
	}

	if t.ConversationID != nil {
		if err := w.markConversation(ctx, t); err != nil {
			return err
		}
	}

	for _, l := range in.Links {
		if _, err := w.Link(ctx, t, l); err != nil {
			return err
		}
	}

	changes := map[string]appevents.TicketChange{"state": change(nil, t.StateID)}

	return w.emit(ctx, appevents.TicketCreated{TicketChanged: w.changed(t, changes)})
}

/*
markConversation notes in the conversation's timeline that it became a
ticket.
*/
func (w *Writer) markConversation(ctx context.Context, t models.Ticket) error {
	c, err := w.Q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: t.WorkspaceID, ID: *t.ConversationID,
	})
	if err != nil {
		return fmt.Errorf("find ticket conversation: %w", err)
	}

	return w.Record(ctx, c, "conversation.ticket_created", map[string]any{
		"ticket_id": models.PublicID(models.PrefixTicket, t.ID), "reference": Reference(t.Number), "title": t.Title,
	})
}
