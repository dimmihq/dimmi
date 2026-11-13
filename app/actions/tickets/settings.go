/*
|--------------------------------------------------------------------------
| Setting Tickets Up
|--------------------------------------------------------------------------
|
| Members who change settings add, change, order and archive states,
| types and fields. A state keeps its group, a field its key and data
| type, a type its kind, so stored tickets never change meaning.
| Removing a state needs a state of the same group its tickets move to;
| archiving a type or field keeps it on the tickets that use it.
|
*/

package tickets

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of setting tickets up.
*/
var (
	ErrNameInvalid = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 60 characters.").OnField("name")
	ErrKeyInvalid  = fault.New(fault.Invalid, "key_invalid",
		"Keys start with a letter and hold lowercase letters, digits and underscores, at most 40.").OnField("key")
	ErrKeyTaken = fault.New(fault.Conflict, "key_taken",
		"The type already has a field with this key.").OnField("key")
	ErrGroupInvalid = fault.New(fault.Invalid, "state_group_invalid",
		"The group is submitted, in_progress, waiting_customer, waiting_team or resolved.").OnField("group")
	ErrDataTypeInvalid = fault.New(fault.Invalid, "data_type_invalid",
		"The data type is not one fields may have.").OnField("data_type")
	ErrMoveToRequired = fault.New(fault.Invalid, "move_to_required",
		"Choose a live state of the same group for the state's tickets.").OnField("move_to")
	ErrTicketsNeedState = fault.New(fault.Invalid, "tickets_need_state",
		"Choose which of the type's states the tickets in a removed state move to.").OnField("move_tickets")
	ErrLastState = fault.New(fault.Invalid, "last_state",
		"Every group keeps at least one live state.").OnField("state")
	ErrOrderInvalid = fault.New(fault.Invalid, "order_invalid",
		"The order names each item exactly once.").OnField("ids")
)

/*
keyShape is what a field key looks like.
*/
var keyShape = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

/*
manage runs the function for a member who sets tickets up.
*/
func (t *Tickets) manage(ctx context.Context, m authz.Membership, fn func(ctx context.Context, w *Writer) error) error {
	if err := canManage(m); err != nil {
		return err
	}

	return t.write(ctx, m, fn)
}

/*
Settings returns the workspace's setup and each type's ticket counts.
*/
func (t *Tickets) Settings(ctx context.Context, m authz.Membership) (Setup, []models.CountTicketsByTypeRow, error) {
	if err := canRead(m); err != nil {
		return Setup{}, nil, err
	}

	var (
		setup  Setup
		counts []models.CountTicketsByTypeRow
	)

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error
		if setup, err = loadSetup(ctx, q, m.Workspace.ID, t.Clock.Now()); err != nil {
			return err
		}

		counts, err = q.CountTicketsByType(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("count tickets by type: %w", err)
		}

		return nil
	})

	return setup, counts, err
}

/*
sameSet reports whether the order names each id exactly once.
*/
func sameSet(want, got []ids.UUID) bool {
	if len(want) != len(got) {
		return false
	}

	for _, id := range want {
		if !slices.Contains(got, id) {
			return false
		}
	}

	return true
}

/*
setText trims and sets text the input gives.
*/
func setText(dst, v *string) {
	if v != nil {
		*dst = strings.TrimSpace(*v)
	}
}

/*
setValue sets a value the input gives.
*/
func setValue[T any](dst, v *T) {
	if v != nil {
		*dst = *v
	}
}

/*
setOpt sets or clears an optional value the input gives.
*/
func setOpt[T any](dst, v **T) {
	if v != nil {
		*dst = *v
	}
}
