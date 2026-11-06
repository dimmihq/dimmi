/*
|--------------------------------------------------------------------------
| AI Controller
|--------------------------------------------------------------------------
|
| The agent's settings surface: the agent itself, its provider
| connections and the subscription connect. Keys come and go sealed;
| the API only ever answers with their public halves, and every id in
| and out is a public one.
|
*/

package v1

import (
	"context"
	"math"

	"github.com/dimmihq/dimmi/framework/fault"
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
}

/*
Agent answers with the agent view.
*/
func (c *AIController) Agent(ctx context.Context, _ *requests.AIAgentRequest) (*AIAgentOutput, error) {
	view, err := c.Get.Handle(ctx, ai.GetAgentInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}
	return &AIAgentOutput{Body: fhttp.NewResource(resources.NewAIAgentView(*view))}, nil
}

/*
UpdateAgent saves the fields the request sent and answers with the
fresh agent view.
*/
func (c *AIController) UpdateAgent(ctx context.Context, in *requests.AIUpdateAgentRequest) (*AIAgentOutput, error) {
	patch, err := c.agentPatch(in.Body, requests.Sent(in.RawBody))
	if err != nil {
		return nil, err
	}

	if _, err := c.Update.Handle(ctx, ai.UpdateAgentInput{Membership: membership(ctx), Patch: patch}); err != nil {
		return nil, err
	}
	return c.Agent(ctx, &requests.AIAgentRequest{})
}

/*
agentPatch maps the request to the action's patch: plain fields as
sent, nullable fields set or cleared, public ids decoded.
*/
func (c *AIController) agentPatch(b requests.AIUpdateAgentBody, sent map[string]bool) (ai.AgentPatch, error) {
	p := ai.AgentPatch{
		Name: b.Name, Tone: b.Tone, Guidance: b.Guidance, Schedule: b.Schedule,
		AssignedBehavior: b.AssignedBehavior, UnknownBehavior: b.UnknownBehavior,
		DefaultLanguage: b.DefaultLanguage, IncludeContact: b.IncludeContact,
		Enabled: b.Enabled, RequireCitations: b.RequireCitations, PiiRedaction: b.PiiRedaction,
		Streaming: b.Streaming, RaisePrio: b.RaisePriority,
		EnabledChannels: b.EnabledChannels, Languages: b.Languages, LinkAllowlist: b.LinkAllowlist,
		ProductName:      ai.Optional[string]{Set: sent["product_name"], Value: b.ProductName},
		AnswerModel:      ai.Optional[string]{Set: sent["answer_model"], Value: b.AnswerModel},
		FallbackModel:    ai.Optional[string]{Set: sent["fallback_model"], Value: b.FallbackModel},
		MonthlyCapMicros: capMicros(sent, b.MonthlyCapUSD),
		MaxAiReplies:     b.MaxAiReplies,
		HandoffMode:      handoffMode(b.HandoffMode),
	}

	tags, err := c.tagIDs(sent, b.AlwaysHandoffTag)
	if err != nil {
		return p, err
	}
	p.AlwaysHandoffTags = tags

	refs := []struct {
		field  string
		prefix models.Prefix
		value  *string
		dst    *ai.Optional[ids.UUID]
	}{
		{"answer_key_id", models.PrefixAiProviderKey, b.AnswerKeyID, &p.AnswerKeyID},
		{"fallback_key_id", models.PrefixAiProviderKey, b.FallbackKeyID, &p.FallbackKeyID},
		{"handoff_team_id", models.PrefixTeam, b.HandoffTeamID, &p.HandoffTeamID},
	}
	for _, ref := range refs {
		decoded, err := c.optionalID(sent, ref.field, ref.prefix, ref.value)
		if err != nil {
			return p, err
		}
		*ref.dst = decoded
	}

	return p, nil
}

/*
optionalID decodes a nullable public id the request may have sent; null
or an empty string clears it.
*/
func (c *AIController) optionalID(sent map[string]bool, field string, prefix models.Prefix,
	value *string,
) (ai.Optional[ids.UUID], error) {
	if !sent[field] {
		return ai.Optional[ids.UUID]{}, nil
	}
	if value == nil || *value == "" {
		return ai.Optional[ids.UUID]{Set: true}, nil
	}

	id, err := c.IDs.Decode(string(prefix), *value)
	if err != nil {
		return ai.Optional[ids.UUID]{}, fault.New(fault.Invalid, "invalid_id", "the id is not valid").OnField(field)
	}
	return ai.Optional[ids.UUID]{Set: true, Value: &id}, nil
}

/*
handoffMode maps the agent's role the request sent, if any.
*/
func handoffMode(mode *string) *enums.AIHandoffMode {
	if mode == nil {
		return nil
	}
	m := enums.AIHandoffMode(*mode)
	return &m
}

/*
tagIDs decodes the public tag ids that always hand off; a list sent as
null clears it.
*/
func (c *AIController) tagIDs(sent map[string]bool, list *[]string) (*[]ids.UUID, error) {
	const field = "always_handoff_tag_ids"
	if !sent[field] {
		return nil, nil
	}

	out := []ids.UUID{}
	if list == nil {
		return &out, nil
	}
	for _, v := range *list {
		id, err := c.IDs.Decode(string(models.PrefixTag), v)
		if err != nil {
			return nil, fault.New(fault.Invalid, "invalid_id", "the id is not valid").OnField(field)
		}
		out = append(out, id)
	}
	return &out, nil
}

/*
capMicros maps the cap in dollars to micro-dollars; null clears it.
*/
func capMicros(sent map[string]bool, usd *float64) ai.Optional[int64] {
	if !sent["monthly_cap_usd"] || usd == nil {
		return ai.Optional[int64]{Set: sent["monthly_cap_usd"]}
	}
	micros := int64(math.Round(*usd * 1_000_000))
	return ai.Optional[int64]{Set: true, Value: &micros}
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
