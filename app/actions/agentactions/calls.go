/*
|--------------------------------------------------------------------------
| Action Calls
|--------------------------------------------------------------------------
|
| Every call the agent made, for the team: any member may read them and
| decide the ones waiting for a teammate; those who manage actions also
| decide the ones waiting for an admin. Inputs, requests and responses
| are payloads like a run's prompts, shown only to those who may read
| payloads.
|
*/

package agentactions

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
callsPerPage is the size of one page of calls.
*/
const callsPerPage = 50

/*
Calls reads and decides the agent's calls.
*/
type Calls struct {
	DB  *database.DB
	Kit *actionkit.Kit
}

/*
CallFilter narrows the call list, and Before pages it.
*/
type CallFilter struct {
	ConversationID *ids.UUID
	ActionID       *ids.UUID
	State          *string
	Before         *ids.UUID
}

/*
List returns a page of the workspace's calls, newest first, and whether
more follow.
*/
func (c *Calls) List(ctx context.Context, m authz.Membership, f CallFilter) ([]models.AiActionCall, bool, error) {
	if err := RequireRead(m); err != nil {
		return nil, false, err
	}
	var out []models.AiActionCall
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListActionCalls(ctx, models.ListActionCallsParams{
			WorkspaceID: m.Workspace.ID, ConversationID: f.ConversationID, ActionID: f.ActionID, State: f.State,
			BeforeID: f.Before, RowLimit: callsPerPage + 1,
		})
		return err
	})
	if err != nil {
		return nil, false, fmt.Errorf("list action calls: %w", err)
	}
	more := len(out) > callsPerPage
	if more {
		out = out[:callsPerPage]
	}
	return out, more, nil
}

/*
Show returns one call.
*/
func (c *Calls) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.AiActionCall, error) {
	if err := RequireRead(m); err != nil {
		return models.AiActionCall{}, err
	}
	var out models.AiActionCall
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrActionCallNotFound
		}
		return err
	})
	return out, err
}

/*
Decide records a teammate's approval or decline.
*/
func (c *Calls) Decide(ctx context.Context, in actionkit.DecideInput) (models.AiActionCall, error) {
	return c.Kit.Decide(ctx, in)
}

/*
CanReadPayloads reports whether the caller may see a call's input,
request, response and result.
*/
func CanReadPayloads(m authz.Membership) bool {
	return m.Can("ai_runs:read_payloads")
}
