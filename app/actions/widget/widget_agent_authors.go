/*
|--------------------------------------------------------------------------
| Widget Agent Authors
|--------------------------------------------------------------------------
|
| A reply an AI agent wrote shows the agent's name in the messenger, so a
| visitor talking to the Sales agent sees "Sales", not the workspace.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
agentAuthors names the AI agents among the authors.
*/
func agentAuthors(ctx context.Context, q *models.Queries, ws ids.UUID, agents []ids.UUID,
) (map[authoredBy]Author, error) {
	out := map[authoredBy]Author{}
	if len(agents) == 0 {
		return out, nil
	}
	found, err := q.ListWidgetAgentAuthors(ctx, models.ListWidgetAgentAuthorsParams{WorkspaceID: ws, AgentIds: agents})
	if err != nil {
		return nil, fmt.Errorf("list widget agent authors: %w", err)
	}
	for _, row := range found {
		name := row.Name
		out[authoredBy{kind: string(enums.AuthorTypeAgent), id: row.ID}] = Author{
			Type: string(enums.AuthorTypeAgent), Name: &name,
		}
	}
	return out, nil
}
