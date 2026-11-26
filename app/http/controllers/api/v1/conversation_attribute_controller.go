/*
|--------------------------------------------------------------------------
| Conversation Attribute Controller
|--------------------------------------------------------------------------
|
| GET /conversation-attributes lists the workspace's conversation
| attributes; POST makes one; PATCH changes its words, choices,
| settings or status. GET /conversations/{conversation}/attributes
| reads a conversation's values and who set each; PATCH sets some.
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
ConversationAttributeController serves conversation attributes.
*/
type ConversationAttributeController struct {
	Attributes *aiattributes.Attributes
	IDs        *ids.Registry
}

/*
ConversationAttributesOutput is the workspace's conversation attributes.
*/
type ConversationAttributesOutput struct {
	Body fhttp.Collection[resources.ConversationAttribute]
}

/*
ConversationAttributeOutput is one conversation attribute.
*/
type ConversationAttributeOutput struct {
	Body fhttp.Resource[resources.ConversationAttribute]
}

/*
ConversationAttributeValuesOutput is a conversation's values.
*/
type ConversationAttributeValuesOutput struct {
	Body fhttp.Resource[resources.ConversationAttributeValues]
}

/*
Index lists the conversation attributes.
*/
func (c *ConversationAttributeController) Index(ctx context.Context, _ *requests.NoInput,
) (*ConversationAttributesOutput, error) {
	rows, err := c.Attributes.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.ConversationAttribute, len(rows))
	for i, r := range rows {
		items[i] = resources.NewConversationAttribute(r)
	}

	page := pagination.Page[resources.ConversationAttribute]{Items: items, Limit: 200}

	return &ConversationAttributesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store makes a conversation attribute.
*/
func (c *ConversationAttributeController) Store(ctx context.Context, in *requests.StoreConversationAttributeRequest,
) (*ConversationAttributeOutput, error) {
	b := in.Body

	teams, err := c.teams(b.TeamIDs)
	if err != nil {
		return nil, err
	}

	row, err := c.Attributes.Create(ctx, aiattributes.CreateInput{
		Membership: membership(ctx), Key: b.Key, Label: b.Label, Description: b.Description, DataType: b.DataType,
		Options: b.Options, FilledBy: b.FilledBy, RequiredToClose: b.RequiredToClose, TeamIDs: teams,
	})

	return conversationAttributeOutput(row, err)
}

/*
Update changes a conversation attribute.
*/
func (c *ConversationAttributeController) Update(ctx context.Context, in *requests.UpdateConversationAttributeRequest,
) (*ConversationAttributeOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAIAttribute), in.Attribute)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	b := in.Body
	update := aiattributes.UpdateInput{
		Membership: membership(ctx), ID: id, Label: b.Label, Description: b.Description, Status: b.Status,
		FilledBy: b.FilledBy, RequiredToClose: b.RequiredToClose,
	}

	if b.Options != nil {
		update.Options = append([]string{}, *b.Options...)
	}

	if b.TeamIDs != nil {
		teams, err := c.teams(*b.TeamIDs)
		if err != nil {
			return nil, err
		}

		update.TeamIDs = &teams
	}

	row, err := c.Attributes.Update(ctx, update)

	return conversationAttributeOutput(row, err)
}

/*
Values reads a conversation's values.
*/
func (c *ConversationAttributeController) Values(ctx context.Context, in *requests.ConversationRequest,
) (*ConversationAttributeValuesOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	found, err := c.Attributes.Values(ctx, membership(ctx), ref)

	return conversationValuesOutput(found, err)
}

/*
SetValues sets some of a conversation's values.
*/
func (c *ConversationAttributeController) SetValues(
	ctx context.Context, in *requests.UpdateConversationAttributeValuesRequest,
) (*ConversationAttributeValuesOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	found, err := c.Attributes.Set(ctx, aiattributes.SetInput{
		Membership: membership(ctx), Ref: ref, Values: in.Body.Values, MutationID: mutationID(ctx),
	})

	return conversationValuesOutput(found, err)
}

/*
teams reads team ids, refusing one that is not a team's.
*/
func (c *ConversationAttributeController) teams(public []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(public))
	for _, p := range public {
		id, err := c.IDs.Decode(string(models.PrefixTeam), p)
		if err != nil {
			return nil, aiattributes.ErrTeams
		}

		out = append(out, id)
	}

	return out, nil
}

func conversationAttributeOutput(row models.AiConversationAttribute, err error) (*ConversationAttributeOutput, error) {
	if err != nil {
		return nil, err
	}

	return &ConversationAttributeOutput{Body: fhttp.NewResource(resources.NewConversationAttribute(row))}, nil
}

func conversationValuesOutput(found aiattributes.Found, err error) (*ConversationAttributeValuesOutput, error) {
	if err != nil {
		return nil, err
	}

	return &ConversationAttributeValuesOutput{
		Body: fhttp.NewResource(resources.NewConversationAttributeValues(found)),
	}, nil
}
