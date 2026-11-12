/*
|--------------------------------------------------------------------------
| Customer Requests
|--------------------------------------------------------------------------
|
| What the messenger and the help center portal need beside the
| customer views: who the customer is, a form checked before its
| conversation starts, so a refused form leaves nothing behind, and the
| ticket each of the customer's conversations carries, by its customer
| label only.
|
*/

package tickets

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Badge is the ticket a conversation carries, with the state its
customer sees.
*/
type Badge struct {
	Ticket models.Ticket
	State  models.TicketState
}

/*
CustomerFor reads who a contact is for the customer views: their
company, and whether the caller proved who they are.
*/
func (t *Tickets) CustomerFor(ctx context.Context, ws, contact ids.UUID, verified bool) (Customer, error) {
	who := Customer{WorkspaceID: ws, ContactID: contact, Verified: verified}

	err := t.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		c, err := models.New(tx).FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: contact})
		if err != nil {
			return fmt.Errorf("find customer: %w", err)
		}

		who.CompanyID = c.CompanyID

		return nil
	})

	return who, err
}

/*
CustomerCheck checks a customer's form without writing anything: the
type, the title, the fields they may set and the ones they must fill.
*/
func (t *Tickets) CustomerCheck(ctx context.Context, who Customer, in CustomerInput) error {
	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		setup, err := loadSetup(ctx, models.New(tx), who.WorkspaceID, t.Clock.Now())
		if err != nil {
			return err
		}

		return checkCustomerForm(setup, in)
	})
}

/*
checkCustomerForm refuses a form the ticket would refuse.
*/
func checkCustomerForm(setup Setup, in CustomerInput) error {
	ty, ok := setup.Type(in.TypeID)
	if !ok || ty.Kind != KindCustomer || !ty.CustomerVisible || ty.ArchivedAt != nil {
		return ErrTypeNotFound.OnField("type_id")
	}

	if n := len([]rune(strings.TrimSpace(in.Title))); n < 1 || n > 200 {
		return ErrTitleInvalid
	}

	fields := setup.FieldsOf(ty.ID)
	problems := map[string]string{}

	for k := range in.Fields {
		if f, ok := fieldByKey(fields, k); !ok || !f.EditableCustomer {
			problems[k] = "not_editable"
		}
	}

	if len(problems) > 0 {
		return invalidError(problems)
	}

	values, problems := merge(fields, nil, in.Fields)
	if len(problems) > 0 {
		return invalidError(problems)
	}

	if missing := missingFor(fields, values, requiredFor(CreateInput{Customer: true})); len(missing) > 0 {
		return missingError(missing)
	}

	return nil
}

/*
CustomerBadges returns the customer ticket each of the conversations
carries, keyed by conversation, leaving out any the customer does not
see.
*/
func (t *Tickets) CustomerBadges(ctx context.Context, ws ids.UUID, convs []ids.UUID) (map[ids.UUID]Badge, error) {
	out := map[ids.UUID]Badge{}
	if len(convs) == 0 {
		return out, nil
	}

	err := t.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListConversationTickets(ctx, models.ListConversationTicketsParams{
			WorkspaceID: ws, ConversationIds: convs,
		})
		if err != nil || len(rows) == 0 {
			return wrapBadges(err)
		}

		setup, err := loadSetup(ctx, q, ws, t.Clock.Now())
		if err != nil {
			return err
		}

		for _, r := range rows {
			if r.Kind != KindCustomer || !r.CustomerVisible || r.ConversationID == nil {
				continue
			}

			st, _ := setup.State(r.StateID)
			out[*r.ConversationID] = Badge{Ticket: r, State: st}
		}

		return nil
	})

	return out, err
}

/*
wrapBadges explains a failed read of conversations' tickets.
*/
func wrapBadges(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("list conversation tickets: %w", err)
}

/*
CustomerShares reports whether any type the customer may open shares
its tickets with the requester's company.
*/
func CustomerShares(types []CustomerType) bool {
	for _, ty := range types {
		if ty.Type.CompanySharing {
			return true
		}
	}

	return false
}
