/*
|--------------------------------------------------------------------------
| AI Attribute Facts
|--------------------------------------------------------------------------
|
| Conditions and variables read a conversation's AI attributes as
| conversation.ai.<key>: the value the decision model last found, and
| nothing while it found none.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
loadAIValues reads the AI attributes found on the conversation.
*/
func (s *subject) loadAIValues(ctx context.Context, q *models.Queries) error {
	s.aiValues = map[string]any{}

	row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
		WorkspaceID: s.conversation.WorkspaceID, ConversationID: s.conversation.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find ai values: %w", err)
	}

	_ = json.Unmarshal(row.Values, &s.aiValues)

	return nil
}
