/*
|--------------------------------------------------------------------------
| AI Attribute Values
|--------------------------------------------------------------------------
|
| What the decision model found on a conversation, read beside the
| attributes that say what each value means. Values of archived
| attributes are kept and still read, so a workflow or view that named
| one keeps working on what it last held.
|
*/

package aiattributes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Value is one attribute of a conversation with the value found for it,
nil when none was found yet.
*/
type Value struct {
	Attribute models.AiConversationAttribute
	Value     any
}

/*
Found is a conversation's attributes and when it was last read.
*/
type Found struct {
	Values    []Value
	CheckedAt *time.Time
}

/*
Values returns the AI attributes of a conversation the member may see.
*/
func (a *Attributes) Values(ctx context.Context, m authz.Membership, ref conversations.Ref) (Found, error) {
	if err := conversations.CanRead(m); err != nil {
		return Found{}, err
	}

	var out Found

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		conversationID := c.ID

		defs, err := q.ListAIConversationAttributes(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list ai attributes: %w", err)
		}

		row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
			WorkspaceID: m.Workspace.ID, ConversationID: conversationID,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find ai values: %w", err)
		}

		kept := decodeValues(row.Values)
		out.CheckedAt = row.CheckedAt

		for _, d := range defs {
			if v, ok := kept[d.Key]; ok || d.Status == "active" {
				out.Values = append(out.Values, Value{Attribute: d, Value: v})
			}
		}

		return nil
	})

	return out, err
}

/*
Kept returns the values found on a conversation, by key, for a caller
inside the workspace's transaction.
*/
func Kept(ctx context.Context, q *models.Queries, workspaceID, conversationID ids.UUID) (map[string]any, error) {
	row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
		WorkspaceID: workspaceID, ConversationID: conversationID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return map[string]any{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find ai values: %w", err)
	}

	return decodeValues(row.Values), nil
}
