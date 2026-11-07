/*
|--------------------------------------------------------------------------
| Visitor's Action Confirmations
|--------------------------------------------------------------------------
|
| The confirmation card under an agent reply: the visitor reads what the
| agent proposes to do and presses Confirm or Cancel. Only a call of the
| visitor's own conversation is ever found; any other is simply not
| found, whoever's id is guessed. Pressing a button twice answers with
| where the call stands and never runs anything twice.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ActionCallInput names a call in the visitor's conversation.
*/
type ActionCallInput struct {
	Auth         Authenticated
	Conversation ids.UUID
	Call         ids.UUID
}

/*
VisitorActionCalls shows, confirms and cancels the calls the agent
proposed in a visitor's conversation.
*/
type VisitorActionCalls struct {
	DB  *database.DB
	Kit *actionkit.Kit
}

/*
Show returns the call as the visitor may see it.
*/
func (a *VisitorActionCalls) Show(ctx context.Context, in ActionCallInput) (models.AiActionCall, error) {
	ref, err := a.ref(ctx, in)
	if err != nil {
		return models.AiActionCall{}, err
	}
	var out models.AiActionCall
	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		call, err := models.New(tx).FindConversationActionCall(ctx, models.FindConversationActionCallParams{
			WorkspaceID: ref.WorkspaceID, ConversationID: &ref.ConversationID, ID: ref.CallID,
		})
		if err != nil {
			return exceptions.ErrActionCallNotFound
		}
		out = call
		return nil
	})
	return out, err
}

/*
Confirm records the visitor's yes.
*/
func (a *VisitorActionCalls) Confirm(ctx context.Context, in ActionCallInput) (models.AiActionCall, error) {
	ref, err := a.ref(ctx, in)
	if err != nil {
		return models.AiActionCall{}, err
	}
	return a.Kit.Confirm(ctx, ref)
}

/*
Cancel records the visitor's no.
*/
func (a *VisitorActionCalls) Cancel(ctx context.Context, in ActionCallInput) (models.AiActionCall, error) {
	ref, err := a.ref(ctx, in)
	if err != nil {
		return models.AiActionCall{}, err
	}
	return a.Kit.Cancel(ctx, ref)
}

/*
ref finds the visitor's conversation and names the call in it.
*/
func (a *VisitorActionCalls) ref(ctx context.Context, in ActionCallInput) (actionkit.Ref, error) {
	var ref actionkit.Ref
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := models.New(tx).FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation))
		if err != nil {
			return exceptions.ErrWidgetConversationNotFound
		}
		ref = actionkit.Ref{
			WorkspaceID: found.Conversation.WorkspaceID, ConversationID: found.Conversation.ID, CallID: in.Call,
		}
		return nil
	})
	return ref, err
}
