/*
|--------------------------------------------------------------------------
| Run Environment
|--------------------------------------------------------------------------
|
| What a run loads once and every stage reads: the workspace, its
| agent, the conversation, the trigger, the transcript tail, and the
| connection each model role resolves to.
|
*/

package pipeline

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/pii"
)

/*
env is the run's loaded world.
*/
type env struct {
	patterns  patterns
	redactor  *pii.Redactor
	tx             *database.Tx
	Workspace      models.Workspace
	Agent          *models.AiAgent
	Conversation   models.Conversation
	Trigger        models.Message
	Messages       []models.Message
	ChannelType    string
	AnswerKey      *models.AiProviderKey
	pendingConfirm bool
	now            time.Time
}

/*
loadEnv loads everything the gate and stages need.
*/
func (r *Runner) loadEnv(ctx context.Context, tx *database.Tx, workspaceID, conversationID ids.UUID) (*env, error) {
	conv, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: workspaceID, ID: conversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	ws, err := models.New(tx).FindWorkspace(ctx, conv.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}

	msgs, err := models.New(tx).ListMessages(ctx, models.ListMessagesParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, RowLimit: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	env := &env{tx: tx, Workspace: ws, Conversation: conv, Messages: msgs, patterns: newPatterns(), redactor: pii.New(), now: r.Clock.Now()}
	if agent, err := models.New(tx).FindAgent(ctx, conv.WorkspaceID); err == nil {
		env.Agent = &agent
	}

	if len(msgs) > 0 {
		env.Trigger = msgs[len(msgs)-1]
		for _, msg := range slices.Backward(msgs) {
			if msg.AuthorType == "contact" {
				env.Trigger = msg
				break
			}
		}
	}

	env.ChannelType = channelTypeOf(ctx, tx, conv)

	if env.Agent != nil {
		env.AnswerKey = keyFor(ctx, tx, ws.ID, env.Agent.AnswerKeyID)
	}

	return env, nil
}

/*
keyFor loads a role's connection.
*/
func keyFor(ctx context.Context, tx *database.Tx, workspaceID ids.UUID, keyID *ids.UUID) *models.AiProviderKey {
	if keyID == nil {
		return nil
	}
	key, err := models.New(tx).FindProviderKey(ctx, models.FindProviderKeyParams{
		WorkspaceID: workspaceID, ID: *keyID,
	})
	if err != nil {
		return nil
	}
	return &key
}

/*
channelTypeOf resolves the conversation's channel type.
*/
func channelTypeOf(ctx context.Context, tx *database.Tx, conv models.Conversation) string {
	conn, err := models.New(tx).FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: conv.WorkspaceID, ID: conv.ChannelConnectionID,
	})
	if err != nil {
		return "widget"
	}
	return conn.Type
}
