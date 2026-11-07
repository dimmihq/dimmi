/*
|--------------------------------------------------------------------------
| Widget Action Call Controller
|--------------------------------------------------------------------------
|
| The confirmation card's two buttons and the card's own refresh. A
| visitor only ever reaches calls of their own conversations; any other
| id, valid or not, is simply not found.
|
*/

package widget

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ActionCallController serves the visitor's confirmation cards.
*/
type ActionCallController struct {
	Calls *actions.VisitorActionCalls
	IDs   *ids.Registry
}

/*
ActionCallOutput is one call as the visitor sees it.
*/
type ActionCallOutput struct {
	Body fhttp.Resource[resources.WidgetActionCall]
}

/*
Show answers the call.
*/
func (c *ActionCallController) Show(ctx context.Context, in *requests.WidgetActionCallRequest,
) (*ActionCallOutput, error) {
	input, err := c.input(ctx, in.Conversation, in.Call)
	if err != nil {
		return nil, err
	}
	call, err := c.Calls.Show(ctx, input)
	if err != nil {
		return nil, err
	}
	return &ActionCallOutput{Body: fhttp.NewResource(resources.NewWidgetActionCall(call))}, nil
}

/*
Update confirms or cancels the call.
*/
func (c *ActionCallController) Update(ctx context.Context, in *requests.UpdateWidgetActionCallRequest,
) (*ActionCallOutput, error) {
	input, err := c.input(ctx, in.Conversation, in.Call)
	if err != nil {
		return nil, err
	}
	decide := c.Calls.Cancel
	if in.Body.Status == "confirmed" {
		decide = c.Calls.Confirm
	}
	call, err := decide(ctx, input)
	if err != nil {
		return nil, err
	}
	return &ActionCallOutput{Body: fhttp.NewResource(resources.NewWidgetActionCall(call))}, nil
}

/*
input decodes the path's public ids; an unknown one is not found.
*/
func (c *ActionCallController) input(ctx context.Context, conversation, call string,
) (actions.ActionCallInput, error) {
	convID, err := c.IDs.Decode(string(models.PrefixConversation), conversation)
	if err != nil {
		return actions.ActionCallInput{}, exceptions.ErrWidgetConversationNotFound
	}
	callID, err := c.IDs.Decode(string(models.PrefixAiActionCall), call)
	if err != nil {
		return actions.ActionCallInput{}, exceptions.ErrActionCallNotFound
	}
	return actions.ActionCallInput{Auth: authenticated(ctx), Conversation: convID, Call: callID}, nil
}
