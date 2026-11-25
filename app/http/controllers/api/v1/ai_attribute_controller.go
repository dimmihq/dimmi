/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Controller
|--------------------------------------------------------------------------
|
| GET /ai/conversation-attributes lists the workspace's AI conversation
| attributes; POST makes one; PATCH changes its words, its choices or
| its status. GET /conversations/{conversation}/ai-attributes lists what
| the decision model found on one conversation.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/aiattributes"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAttributeController serves AI conversation attributes.
*/
type AIAttributeController struct {
	Attributes *aiattributes.Attributes
	IDs        *ids.Registry
}

/*
AIAttributesOutput is the workspace's AI attributes.
*/
type AIAttributesOutput struct {
	Body fhttp.Collection[resources.AIAttribute]
}

/*
AIAttributeOutput is one AI attribute.
*/
type AIAttributeOutput struct {
	Body fhttp.Resource[resources.AIAttribute]
}

/*
ConversationAIAttributesOutput is what was found on a conversation.
*/
type ConversationAIAttributesOutput struct {
	Body fhttp.Resource[resources.ConversationAIAttributes]
}

/*
Index lists the AI attributes.
*/
func (c *AIAttributeController) Index(ctx context.Context, _ *requests.NoInput) (*AIAttributesOutput, error) {
	rows, err := c.Attributes.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.AIAttribute, len(rows))
	for i, r := range rows {
		items[i] = resources.NewAIAttribute(r)
	}

	page := pagination.Page[resources.AIAttribute]{Items: items, Limit: 100}

	return &AIAttributesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store makes an AI attribute.
*/
func (c *AIAttributeController) Store(ctx context.Context, in *requests.StoreAIAttributeRequest,
) (*AIAttributeOutput, error) {
	b := in.Body
	row, err := c.Attributes.Create(ctx, aiattributes.CreateInput{
		Membership: membership(ctx), Key: b.Key, Label: b.Label, Description: b.Description, DataType: b.DataType,
		Options: b.Options,
	})

	return aiAttributeOutput(row, err)
}

/*
Update changes an AI attribute.
*/
func (c *AIAttributeController) Update(ctx context.Context, in *requests.UpdateAIAttributeRequest,
) (*AIAttributeOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAIAttribute), in.Attribute)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	b := in.Body
	update := aiattributes.UpdateInput{
		Membership: membership(ctx), ID: id, Label: b.Label, Description: b.Description, Status: b.Status,
	}

	if b.Options != nil {
		update.Options = append([]string{}, *b.Options...)
	}

	row, err := c.Attributes.Update(ctx, update)

	return aiAttributeOutput(row, err)
}

/*
Conversation lists what was found on a conversation.
*/
func (c *AIAttributeController) Conversation(ctx context.Context, in *requests.ConversationRequest,
) (*ConversationAIAttributesOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	found, err := c.Attributes.Values(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return &ConversationAIAttributesOutput{
		Body: fhttp.NewResource(resources.NewConversationAIAttributes(found)),
	}, nil
}

func aiAttributeOutput(row models.AiConversationAttribute, err error) (*AIAttributeOutput, error) {
	if err != nil {
		return nil, err
	}

	return &AIAttributeOutput{Body: fhttp.NewResource(resources.NewAIAttribute(row))}, nil
}
