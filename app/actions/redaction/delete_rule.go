/*
|--------------------------------------------------------------------------
| Delete Redaction Rule
|--------------------------------------------------------------------------
|
| A removed rule stops acting on the next message. What it already
| replaced stays replaced: nothing kept the original.
|
*/

package redaction

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteRuleInput is who removes which rule.
*/
type DeleteRuleInput struct {
	Membership authz.Membership
	RuleID     ids.UUID
}

/*
DeleteRule removes a rule. Messages it already changed stay as stored.

Rules:
  - The caller needs settings:write and redaction:manage.

Events: [appevents.RedactionRuleDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteRule struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle removes the rule.
*/
func (a *DeleteRule) Handle(ctx context.Context, in DeleteRuleInput) error {
	if !canManage(in.Membership) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		current, err := find(ctx, q, ws, in.RuleID)
		if err != nil {
			return err
		}

		_, err = q.DeleteRedactionRule(ctx, models.DeleteRedactionRuleParams{WorkspaceID: ws, ID: current.ID})
		if err != nil {
			return fmt.Errorf("delete redaction rule: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.RedactionRuleDeleted{
			Base: base(in.Membership, a.Clock.Now()), RuleID: current.ID, Name: current.Name,
		})
	})
}
