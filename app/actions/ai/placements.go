/*
|--------------------------------------------------------------------------
| Agent Placements
|--------------------------------------------------------------------------
|
| Everywhere an agent can take new conversations, for the picker in
| Agent Studio: the workspace's channel connections, Slack channels and
| inboxes, each with the agent placed there now.
|
*/

package ai

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Placements lists everywhere an agent can be placed.
*/
func (a *Agents) Placements(ctx context.Context, m authz.Membership) ([]models.ListPlacementTargetsRow, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var out []models.ListPlacementTargetsRow
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListPlacementTargets(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list placement targets: %w", err)
		}
		out = rows
		return nil
	})
	return out, err
}
