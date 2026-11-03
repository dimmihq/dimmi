/*
|--------------------------------------------------------------------------
| Record Payloads
|--------------------------------------------------------------------------
|
| Broadcast payloads read from the stored records, inside the workspace
| the event happened in, when the broadcast goes out.
|
*/

package broadcasting

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RecordPayloads renders broadcasts from the stored records.
*/
type RecordPayloads struct {
	DB *database.DB
}

/*
ConversationRow renders the conversation as lists show it.
*/
func (p RecordPayloads) ConversationRow(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	return p.Conversation(ctx, workspaceID, id)
}

/*
Conversation renders the whole conversation.
*/
func (p RecordPayloads) Conversation(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	var c models.Conversation

	found, err := p.read(ctx, workspaceID, func(ctx context.Context, q *models.Queries) (err error) {
		c, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: id})

		return err
	})
	if err != nil || !found {
		return nil, found, err
	}

	return NewConversationPayload(c), true, nil
}

/*
Message renders the message.
*/
func (p RecordPayloads) Message(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	var m models.Message

	found, err := p.read(ctx, workspaceID, func(ctx context.Context, q *models.Queries) (err error) {
		m, err = q.FindBroadcastMessage(ctx, models.FindBroadcastMessageParams{WorkspaceID: workspaceID, ID: id})

		return err
	})
	if err != nil || !found {
		return nil, found, err
	}

	return NewMessagePayload(m), true, nil
}

/*
read runs the lookup inside the workspace and reports whether it found
its record.
*/
func (p RecordPayloads) read(
	ctx context.Context, workspaceID ids.UUID, find func(ctx context.Context, q *models.Queries) error,
) (bool, error) {
	err := p.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		return find(ctx, models.New(tx))
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read broadcast subject: %w", err)
	}

	return true, nil
}
