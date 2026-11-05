/*
|--------------------------------------------------------------------------
| Conversation Context Controller
|--------------------------------------------------------------------------
|
| GET /conversations/{conversation}/context answers with everything the
| customer panel shows, so opening a conversation costs one request for
| the panel however many sections it has.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConversationContextOutput is a conversation's context.
*/
type ConversationContextOutput struct {
	Body fhttp.Resource[resources.ConversationContext]
}

/*
ConversationContextController serves a conversation's context.
*/
type ConversationContextController struct {
	Show *conversations.ShowContext
	IDs  *ids.Registry
}

/*
Get answers with the context of the conversation.
*/
func (c *ConversationContextController) Get(
	ctx context.Context, in *requests.ConversationRequest,
) (*ConversationContextOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	out, err := c.Show.Handle(ctx, conversations.ContextInput{Membership: membership(ctx), Ref: ref})
	if err != nil {
		return nil, err
	}

	return &ConversationContextOutput{Body: fhttp.NewResource(resources.NewConversationContext(out))}, nil
}
