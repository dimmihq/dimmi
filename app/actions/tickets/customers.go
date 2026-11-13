/*
|--------------------------------------------------------------------------
| What Customers See
|--------------------------------------------------------------------------
|
| The messenger and the help center portal show customers the ticket
| types they may open, with only the fields the customer sees, and
| their own tickets with the customer label of each state, never the
| team's names. A type may share tickets with the requester's whole
| company, which a verified customer then sees too. The customer's form
| checks the fields the customer must fill and may only set fields the
| customer may edit.
|
*/

package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Customer is who asks: the contact, their company, and whether their
identity is verified, which company sharing needs.
*/
type Customer struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
	CompanyID   *ids.UUID
	Verified    bool
}

/*
CustomerType is a type the customer may open, its visible fields and
the labels of its states.
*/
type CustomerType struct {
	Type   models.TicketType
	Fields []models.TicketField
	States []models.TicketState
}

/*
CustomerTypes returns the customer types the messenger and portal show.
*/
func (t *Tickets) CustomerTypes(ctx context.Context, ws ids.UUID) ([]CustomerType, error) {
	var out []CustomerType

	err := t.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		setup, err := loadSetup(ctx, models.New(tx), ws, t.Clock.Now())
		if err != nil {
			return err
		}

		for _, ty := range setup.Types {
			if ty.Kind != KindCustomer || !ty.CustomerVisible || ty.ArchivedAt != nil {
				continue
			}

			out = append(out, CustomerType{
				Type: ty, Fields: customerFields(setup.FieldsOf(ty.ID)), States: setup.Allowed(ty.ID),
			})
		}

		return nil
	})

	return out, err
}

/*
customerFields keeps the live fields the customer sees.
*/
func customerFields(fields []models.TicketField) []models.TicketField {
	var out []models.TicketField
	for _, f := range fields {
		if f.ArchivedAt == nil && f.VisibleCustomer {
			out = append(out, f)
		}
	}

	return out
}

/*
CustomerInput is a customer's form: the type, a title, the values, the
conversation it opened, and a client key a retry repeats.
*/
type CustomerInput struct {
	TypeID         ids.UUID
	Title          string
	Description    string
	Fields         map[string]any
	ConversationID ids.UUID
	ExternalID     *string
}

/*
CustomerCreate makes the customer's ticket on the conversation their
form opened. Values for fields the customer may not edit are refused.
*/
func (t *Tickets) CustomerCreate(ctx context.Context, who Customer, in CustomerInput) (Created, error) {
	var out Created

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.contactWriter(ctx, tx, who)
		if err != nil {
			return err
		}

		ty, ok := w.Setup.Type(in.TypeID)
		if !ok || ty.Kind != KindCustomer || !ty.CustomerVisible || ty.ArchivedAt != nil {
			return ErrTypeNotFound.OnField("type_id")
		}

		problems := map[string]string{}
		for k := range in.Fields {
			if f, ok := fieldByKey(w.Setup.FieldsOf(ty.ID), k); !ok || !f.EditableCustomer {
				problems[k] = "not_editable"
			}
		}

		if len(problems) > 0 {
			return invalidError(problems)
		}

		out, err = w.Create(ctx, who.WorkspaceID, CreateInput{
			TypeID: ty.ID, Title: in.Title, Description: in.Description, Fields: in.Fields, Customer: true,
			ConversationID: &in.ConversationID, ContactID: &who.ContactID, ExternalID: in.ExternalID,
		})

		return err
	})

	return out, err
}

/*
contactWriter returns a writer acting as the customer.
*/
func (t *Tickets) contactWriter(ctx context.Context, tx *database.Tx, who Customer) (*Writer, error) {
	q := models.New(tx)

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: who.WorkspaceID, ID: who.ContactID})
	if err != nil {
		return nil, fmt.Errorf("find customer: %w", err)
	}

	name := ""
	if contact.Name != nil {
		name = *contact.Name
	}

	cw := &conversations.Writer{
		Q: q, Tx: tx, Events: t.Events, Now: t.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeContact), ID: contact.ID, Name: name},
	}

	return t.writerFor(ctx, cw, who.WorkspaceID)
}

/*
The scopes of a customer's ticket list.
*/
const (
	ScopeMine    = "mine"
	ScopeCompany = "company"
)

/*
CustomerTicket is a ticket as its customer sees it: the type, the state
with its customer label, the visible fields and when it reached each
state.
*/
type CustomerTicket struct {
	Ticket  models.Ticket
	Type    models.TicketType
	State   models.TicketState
	Fields  []models.TicketField
	History []StateStep
}

/*
StateStep is one state a ticket reached, as its customer reads it.
*/
type StateStep struct {
	Label string
	Group string
	At    time.Time
}

/*
CustomerTickets returns the customer's tickets, their company's too
when asked for and allowed.
*/
func (t *Tickets) CustomerTickets(
	ctx context.Context, who Customer, scope string, openOnly bool,
) ([]CustomerTicket, error) {
	var out []CustomerTicket

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListContactTickets(ctx, models.ListContactTicketsParams{
			WorkspaceID: who.WorkspaceID, ContactID: &who.ContactID, CompanyID: who.CompanyID,
			CompanyScope: scope == ScopeCompany && who.Verified && who.CompanyID != nil, OpenOnly: openOnly,
			RowLimit: maxPage,
		})
		if err != nil {
			return fmt.Errorf("list customer tickets: %w", err)
		}

		setup, err := loadSetup(ctx, q, who.WorkspaceID, t.Clock.Now())
		if err != nil {
			return err
		}

		for _, r := range rows {
			out = append(out, customerView(setup, r))
		}

		return nil
	})

	return out, err
}

/*
customerView shapes a ticket for its customer.
*/
func customerView(setup Setup, r models.Ticket) CustomerTicket {
	ty, _ := setup.Type(r.TypeID)
	st, _ := setup.State(r.StateID)

	return CustomerTicket{Ticket: r, Type: ty, State: st, Fields: customerFields(setup.FieldsOf(r.TypeID))}
}

/*
CustomerShow returns one of the customer's tickets with its history,
or ErrTicketNotFound for any other.
*/
func (t *Tickets) CustomerShow(ctx context.Context, who Customer, ref Ref) (CustomerTicket, error) {
	var out CustomerTicket

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		r, err := findRow(ctx, q, who.WorkspaceID, ref)
		if err != nil {
			return err
		}

		setup, err := loadSetup(ctx, q, who.WorkspaceID, t.Clock.Now())
		if err != nil {
			return err
		}

		if !customerSees(setup, who, r) {
			return ErrTicketNotFound
		}

		out = customerView(setup, r)
		out.History, err = stateHistory(ctx, q, setup, r)

		return err
	})

	return out, err
}

/*
customerSees reports whether the customer may see the ticket: their
own, or their company's when the type shares it and they are verified.
*/
func customerSees(setup Setup, who Customer, r models.Ticket) bool {
	if r.DeletedAt != nil || r.Kind != KindCustomer || !r.CustomerVisible {
		return false
	}

	if r.ContactID != nil && *r.ContactID == who.ContactID {
		return true
	}

	ty, _ := setup.Type(r.TypeID)

	return ty.CompanySharing && who.Verified && who.CompanyID != nil && samePtr(r.CompanyID, who.CompanyID)
}

/*
stateHistory reads the states a ticket reached, by customer label.
*/
func stateHistory(ctx context.Context, q *models.Queries, setup Setup, r models.Ticket) ([]StateStep, error) {
	events, err := q.ListTicketEvents(ctx, models.ListTicketEventsParams{
		WorkspaceID: r.WorkspaceID, TicketID: r.ID, RowLimit: 500,
	})
	if err != nil {
		return nil, fmt.Errorf("list ticket history: %w", err)
	}

	var out []StateStep

	for _, e := range events {
		if e.Kind != "created" && e.Kind != "state" {
			continue
		}

		data := objectOf(e.Data)
		id, _ := data["to"].(string)

		parsed, err := ids.Parse(id)
		if err != nil {
			continue
		}

		if st, ok := setup.State(parsed); ok {
			out = append(out, StateStep{Label: st.CustomerLabel, Group: st.StateGroup, At: e.CreatedAt})
		}
	}

	return out, nil
}
