/*
|--------------------------------------------------------------------------
| AI Controller
|--------------------------------------------------------------------------
|
| The agent's settings surface: the agent itself, its provider
| connections and the subscription connect. Keys come and go sealed;
| the API only ever answers with their public halves.
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
AIAgentOutput is the agent view for the settings page.
*/
type AIAgentOutput struct {
	Body fhttp.Resource[ai.AgentView]
}

/*
AIKeysOutput is the connection list.
*/
type AIKeysOutput struct {
	Body fhttp.Collection[resources.AIProviderKey]
}

/*
AIKeyOutput is one connection.
*/
type AIKeyOutput struct {
	Body fhttp.Resource[resources.AIProviderKey]
}

/*
AIConnectOutput is the authorize answer.
*/
type AIConnectOutput struct {
	Body fhttp.Resource[ai.StartAnswer]
}

/*
AIConnectDoneOutput is the completed connection.
*/
type AIConnectDoneOutput struct {
	Body fhttp.Resource[ai.CompleteAnswer]
}

/*
AIController serves the agent's settings.
*/
type AIController struct {
	Get             *ai.GetAiAgent
	Update          *ai.UpdateAiAgent
	ListKeys        *ai.ListProviderKeys
	CreateKey       *ai.CreateProviderKey
	UpdateKey       *ai.UpdateProviderKey
	RemoveKey       *ai.DeleteProviderKey
	ValidateKey     *ai.TestProviderKey
	StartChatGPT    *ai.StartChatGPTConnect
	CompleteChatGPT *ai.CompleteChatGPTConnect
	IDs             *ids.Registry
	URL             string
}

/*
Agent answers with the agent view.
*/
func (c *AIController) Agent(ctx context.Context, _ *requests.AIAgentRequest) (*AIAgentOutput, error) {
	view, err := c.Get.Handle(ctx, ai.GetAgentInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}
	return &AIAgentOutput{Body: fhttp.NewResource(*view)}, nil
}

/*
UpdateAgent saves the agent's settings.
*/
func (c *AIController) UpdateAgent(ctx context.Context, in *requests.AIUpdateAgentRequest) (*AIAgentOutput, error) {
	m := membership(ctx)

	current, err := c.Get.Handle(ctx, ai.GetAgentInput{Membership: m})
	if err != nil {
		return nil, err
	}

	agent := current.Agent
	b := in.Body
	if b.Name != nil {
		agent.Name = *b.Name
	}
	if b.Enabled != nil {
		agent.Enabled = *b.Enabled
	}
	if b.Tone != nil {
		agent.Tone = *b.Tone
	}
	if b.Guidance != nil {
		agent.Guidance = *b.Guidance
	}
	if b.ProductName != nil {
		agent.ProductName = b.ProductName
	}
	if b.EnabledChannels != nil {
		agent.EnabledChannels = *b.EnabledChannels
	}
	if b.Schedule != nil {
		agent.Schedule = *b.Schedule
	}
	if b.AnswerKeyID != nil {
		id, err := c.keyID(*b.AnswerKeyID)
		if err != nil {
			return nil, err
		}
		agent.AnswerKeyID = &id
	}
	if b.AnswerModel != nil {
		agent.AnswerModel = b.AnswerModel
	}
	if b.ClassifierKeyID != nil {
		id, err := c.keyID(*b.ClassifierKeyID)
		if err != nil {
			return nil, err
		}
		agent.ClassifierKeyID = &id
	}
	if b.ClassifierModel != nil {
		agent.ClassifierModel = b.ClassifierModel
	}
	if b.FallbackKeyID != nil {
		id, err := c.keyID(*b.FallbackKeyID)
		if err != nil {
			return nil, err
		}
		agent.FallbackKeyID = &id
	}
	if b.FallbackModel != nil {
		agent.FallbackModel = b.FallbackModel
	}
	if b.MonthlyCapUSD != nil {
		micros := int64(*b.MonthlyCapUSD * 1_000_000)
		agent.MonthlySpendCapMicros = &micros
	}
	if b.MaxAiReplies != nil {
		agent.MaxAiReplies = int32(*b.MaxAiReplies)
	}
	if b.RequireCitations != nil {
		agent.RequireCitations = *b.RequireCitations
	}
	if b.PiiRedaction != nil {
		agent.PiiRedaction = *b.PiiRedaction
	}
	if b.Languages != nil {
		agent.Languages = *b.Languages
	}
	if b.DefaultLanguage != nil {
		agent.DefaultLanguage = *b.DefaultLanguage
	}

	if _, err := c.Update.Handle(ctx, ai.UpdateAgentInput{Membership: m, Agent: &agent}); err != nil {
		return nil, err
	}
	return c.Agent(ctx, &requests.AIAgentRequest{})
}

/*
Keys lists the connections.
*/
func (c *AIController) Keys(ctx context.Context, _ *requests.AIAgentRequest) (*AIKeysOutput, error) {
	keys, err := c.ListKeys.Handle(ctx, ai.ListKeysInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	out := make([]resources.AIProviderKey, len(keys))
	for i, k := range keys {
		out[i] = resources.NewAIProviderKey(k)
	}
	page := pagination.Page[resources.AIProviderKey]{Items: out, Limit: 100}
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
func (c *AIController) DeleteKey(ctx context.Context, in *requests.AIKeyRequest) (*AIKeysOutput, error) {
	id, err := c.keyID(in.Key)
	if err != nil {
		return nil, err
	}
	if err := c.RemoveKey.Handle(ctx, ai.DeleteKeyInput{Membership: membership(ctx), KeyID: id}); err != nil {
		return nil, err
	}
	return c.Keys(ctx, &requests.AIAgentRequest{})
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
func (c *AIController) StartConnect(ctx context.Context, _ *requests.AIChatGPTStartRequest) (*AIConnectOutput, error) {
	out, err := c.StartChatGPT.Handle(ctx, ai.StartInput{
		Membership: membership(ctx), RedirectURI: c.URL + "/d/settings/ai/chatgpt/callback",
	})
	if err != nil {
		return nil, err
	}
	return &AIConnectOutput{Body: fhttp.NewResource(*out)}, nil
}

/*
CompleteConnect finishes the sign-in.
*/
func (c *AIController) CompleteConnect(ctx context.Context, in *requests.AIChatGPTCompleteRequest) (*AIConnectDoneOutput, error) {
	out, err := c.CompleteChatGPT.Handle(ctx, ai.CompleteInput{
		Membership: membership(ctx), State: in.Body.State, Code: in.Body.Code,
		RedirectURI: c.URL + "/d/settings/ai/chatgpt/callback",
	})
	if err != nil {
		return nil, err
	}
	return &AIConnectDoneOutput{Body: fhttp.NewResource(*out)}, nil
}

/*
keyID decodes a connection's public id.
*/
func (c *AIController) keyID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode("aik", public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

var _ = models.AiAgent{}
