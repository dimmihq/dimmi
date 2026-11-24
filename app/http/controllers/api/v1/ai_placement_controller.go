/*
|--------------------------------------------------------------------------
| AI Placement Controller
|--------------------------------------------------------------------------
|
| GET /ai/placements lists everywhere an agent can take new
| conversations, with the agent placed there now.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AIPlacementsOutput is everywhere an agent can be placed.
*/
type AIPlacementsOutput struct {
	Body fhttp.Collection[resources.AIPlacementTarget]
}

/*
Placements lists everywhere an agent can be placed.
*/
func (c *AIAgentController) Placements(ctx context.Context, _ *requests.AIPlacementsRequest,
) (*AIPlacementsOutput, error) {
	rows, err := c.Agents.Placements(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	items := resources.NewAIPlacementTargets(rows)
	page := pagination.Page[resources.AIPlacementTarget]{Items: items, Limit: len(items)}
	return &AIPlacementsOutput{Body: fhttp.NewCollection(page)}, nil
}
