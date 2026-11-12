/*
|--------------------------------------------------------------------------
| Status Order
|--------------------------------------------------------------------------
|
| A workspace chooses the order its statuses show in: the roadmap
| columns and filters of its portal, and every status list in the app.
| The team saves the whole order at once, naming each status exactly
| once, so no status is ever lost or shown twice.
|
*/

package feedback

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrStatusOrderInvalid is an order that does not name every status of
the workspace exactly once.
*/
var ErrStatusOrderInvalid = fault.New(fault.Invalid, "statuses_invalid",
	"List every status of the workspace exactly once.").OnField("statuses")

/*
StatusOrder returns the workspace's statuses in the order it chose.
*/
func (p *Posts) StatusOrder(ctx context.Context, m authz.Membership) ([]Status, error) {
	if !m.Can("feedback:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []Status
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = ordered(ctx, models.New(tx), m.Workspace.ID)
		return err
	})
	return out, err
}

/*
Reorder saves the order of the workspace's statuses. Keys names every
status once, first to last; the members who may change the portal may
change its order.
*/
func (p *Posts) Reorder(ctx context.Context, m authz.Membership, keys []string) ([]Status, error) {
	if !m.Can("settings:write") {
		return nil, exceptions.ErrForbidden
	}
	var out []Status
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := ordered(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		if !everyStatusOnce(current, keys) {
			return ErrStatusOrderInvalid
		}
		if err := q.SaveFeedbackStatusOrder(ctx, models.SaveFeedbackStatusOrderParams{
			Now: p.Clock.Now(), Keys: keys, WorkspaceID: m.Workspace.ID,
		}); err != nil {
			return fmt.Errorf("save status order: %w", err)
		}
		out, err = ordered(ctx, q, m.Workspace.ID)
		return err
	})
	return out, err
}

func everyStatusOnce(all []Status, keys []string) bool {
	if len(keys) != len(all) {
		return false
	}
	for _, s := range all {
		if !slices.Contains(keys, s.Key) {
			return false
		}
	}
	return true
}
