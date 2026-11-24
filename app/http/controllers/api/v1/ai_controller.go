/*
|--------------------------------------------------------------------------
| AI Controller
|--------------------------------------------------------------------------
|
| The workspace's AI connections: its provider keys and the
| subscription connect. Keys come and go sealed;
| the API only ever answers with their public halves, and every id in
| and out is a public one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIController serves the workspace's AI connections.
*/
type AIController struct {
	ListKeys        *ai.ListProviderKeys
	CreateKey       *ai.CreateProviderKey
	UpdateKey       *ai.UpdateProviderKey
	RemoveKey       *ai.DeleteProviderKey
	ValidateKey     *ai.TestProviderKey
	StartChatGPT    *ai.StartChatGPTConnect
	CompleteChatGPT *ai.CompleteChatGPTConnect
	IDs             *ids.Registry
}

/*
Keys lists the connections.
*/
func (c *AIController) Keys(ctx context.Context, _ *requests.AIAgentRequest) (*AIKeysOutput, error) {
	keys, err := c.ListKeys.Handle(ctx, ai.ListKeysInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.AIProviderKey]{Items: resources.NewAIProviderKeys(keys), Limit: 100}
	return &AIKeysOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
StoreKey creates a connection.
*/
func (c *AIController) StoreKey(ctx context.Context, in *requests.AIStoreKeyRequest) (*AIKeyOutput, error) {
	b := in.Body

	key, err := c.CreateKey.Handle(ctx, ai.CreateKeyInput{
		Membership: membership(ctx), Name: b.Name, Provider: enums.AIProvider(b.Provider),
		BaseURL: b.BaseURL, APIKey: b.APIKey, Config: b.Config,
	})
	if err != nil {
		return nil, err
	}
	return &AIKeyOutput{Body: fhttp.NewResource(resources.NewAIProviderKey(*key))}, nil
}

/*
PatchKey changes a connection.
*/
func (c *AIController) PatchKey(ctx context.Context, in *requests.AIUpdateKeyRequest) (*AIKeyOutput, error) {
	id, err := c.keyID(in.Key)
	if err != nil {
		return nil, err
	}
	b := in.Body

	name := ""
	if b.Name != nil {
		name = *b.Name
	}

	key, err := c.UpdateKey.Handle(ctx, ai.UpdateKeyInput{
		Membership: membership(ctx), KeyID: id, Name: name,
		BaseURL: b.BaseURL, APIKey: b.APIKey, Config: b.Config,
	})
	if err != nil {
		return nil, err
	}
	return &AIKeyOutput{Body: fhttp.NewResource(resources.NewAIProviderKey(*key))}, nil
}

/*
DeleteKey removes a connection.
*/
func (c *AIController) DeleteKey(ctx context.Context, in *requests.AIKeyRequest) (*EmptyOutput, error) {
	id, err := c.keyID(in.Key)
	if err != nil {
		return nil, err
	}
	if err := c.RemoveKey.Handle(ctx, ai.DeleteKeyInput{Membership: membership(ctx), KeyID: id}); err != nil {
		return nil, err
	}
	return &EmptyOutput{}, nil
}

/*
TestKey revalidates a connection.
*/
func (c *AIController) TestKey(ctx context.Context, in *requests.AIKeyRequest) (*AIKeyOutput, error) {
	id, err := c.keyID(in.Key)
	if err != nil {
		return nil, err
	}
	key, err := c.ValidateKey.Handle(ctx, ai.TestKeyInput{Membership: membership(ctx), KeyID: id})
	if err != nil {
		return nil, err
	}
	return &AIKeyOutput{Body: fhttp.NewResource(resources.NewAIProviderKey(*key))}, nil
}

/*
StartConnect answers with the authorize URL.
*/
func (c *AIController) StartConnect(ctx context.Context, in *requests.AIChatGPTStartRequest) (*AIConnectOutput, error) {
	out, err := c.StartChatGPT.Handle(ctx, ai.StartInput{Membership: membership(ctx), Method: in.Body.Method})
	if err != nil {
		return nil, err
	}
	return &AIConnectOutput{Body: fhttp.NewResource(*out)}, nil
}

/*
CompleteConnect finishes the sign-in.
*/
func (c *AIController) CompleteConnect(ctx context.Context,
	in *requests.AIChatGPTCompleteRequest,
) (*AIConnectDoneOutput, error) {
	out, err := c.CompleteChatGPT.Handle(ctx, ai.CompleteInput{
		Membership: membership(ctx), State: in.Body.State, Code: in.Body.Code,
	})
	if err != nil {
		return nil, err
	}
	return &AIConnectDoneOutput{Body: fhttp.NewResource(resources.NewAIChatGPTConnection(*out))}, nil
}

/*
keyID decodes a connection's public id; anything else is not found.
*/
func (c *AIController) keyID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAiProviderKey), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
