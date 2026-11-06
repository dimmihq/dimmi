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
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
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
	DB              *database.DB
	Clock           clock.Clock
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
	if err := c.mergeAgent(&agent, in); err != nil {
		return nil, err
	}

	if _, err := c.Update.Handle(ctx, ai.UpdateAgentInput{Membership: m, Agent: &agent}); err != nil {
		return nil, err
	}
	return c.Agent(ctx, &requests.AIAgentRequest{})
}

/*
mergeAgent folds every patch field the request carried into the agent
row.
*/
func (c *AIController) mergeAgent(agent *models.AiAgent, in *requests.AIUpdateAgentRequest) error {
	if err := c.mergeIdentity(agent, in); err != nil {
		return err
	}
	c.mergeBehavior(agent, in)
	return nil
}

/*
mergeIdentity applies the fields that name and route the agent: its
profile, channels, schedule, models and connections.
*/
func (c *AIController) mergeIdentity(agent *models.AiAgent, in *requests.AIUpdateAgentRequest) error {
	b := in.Body
	assign(&agent.Name, b.Name)
	assign(&agent.Enabled, b.Enabled)
	assign(&agent.Tone, b.Tone)
	assign(&agent.Guidance, b.Guidance)
	assignPtr(&agent.ProductName, b.ProductName)
	assign(&agent.EnabledChannels, b.EnabledChannels)
	assign(&agent.Schedule, b.Schedule)
	answerID, err := c.keyPtr(b.AnswerKeyID)
	if err != nil {
		return err
	}
	agent.AnswerKeyID = answerID
	assignPtr(&agent.AnswerModel, b.AnswerModel)
	classifierID, err := c.keyPtr(b.ClassifierKeyID)
	if err != nil {
		return err
	}
	agent.ClassifierKeyID = classifierID
	assignPtr(&agent.ClassifierModel, b.ClassifierModel)
	fallbackID, err := c.keyPtr(b.FallbackKeyID)
	if err != nil {
		return err
	}
	agent.FallbackKeyID = fallbackID
	assignPtr(&agent.FallbackModel, b.FallbackModel)
	return nil
}

/*
mergeBehavior applies the fields that govern how the agent answers:
caps, citations, languages and the handoff shape.
*/
func (c *AIController) mergeBehavior(agent *models.AiAgent, in *requests.AIUpdateAgentRequest) {
	b := in.Body
	if b.MonthlyCapUSD != nil {
		micros := int64(*b.MonthlyCapUSD * 1_000_000)
		agent.MonthlySpendCapMicros = &micros
	}
	if b.MaxAiReplies != nil {
		agent.MaxAiReplies = int32(max(0, min(*b.MaxAiReplies, 1000)))
	}
	assign(&agent.RequireCitations, b.RequireCitations)
	assign(&agent.PiiRedaction, b.PiiRedaction)
	assign(&agent.Languages, b.Languages)
	assign(&agent.DefaultLanguage, b.DefaultLanguage)
	assign(&agent.UnknownBehavior, b.UnknownBehavior)
	assign(&agent.AssignedBehavior, b.AssignedBehavior)
	assign(&agent.Streaming, b.Streaming)
	assign(&agent.IncludeContactContext, b.IncludeContact)
	mergeHandoff(agent, b.HandoffTeamID, b.RaisePriority)
}

/*
assign applies a patch field when the request carried it.
*/
func assign[T any](dst, src *T) {
	if src != nil {
		*dst = *src
	}
}

/*
assignPtr applies a pointer patch field when the request carried it.
*/
func assignPtr[T any](dst **T, src *T) {
	if src != nil {
		*dst = src
	}
}

/*
keyPtr resolves a connection's id when the request carried one.
*/
func (c *AIController) keyPtr(s *string) (*ids.UUID, error) {
	if s == nil {
		return nil, nil
	}
	id, err := c.keyID(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

/*
mergeHandoff folds the handoff patch fields into the agent's jsonb.
*/
func mergeHandoff(agent *models.AiAgent, teamID *string, raisePriority *bool) {
	if teamID == nil && raisePriority == nil {
		return
	}
	handoff := map[string]any{}
	_ = json.Unmarshal(agent.Handoff, &handoff)
	if teamID != nil && *teamID == "" {
		delete(handoff, "target_team_id")
	} else if teamID != nil {
		handoff["target_team_id"] = *teamID
	}
	if raisePriority != nil {
		handoff["raise_priority"] = *raisePriority
	}
	raw, _ := json.Marshal(handoff)
	agent.Handoff = raw
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
func (c *AIController) CompleteConnect(ctx context.Context,
	in *requests.AIChatGPTCompleteRequest,
) (*AIConnectDoneOutput, error) {
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

/*
Runs lists a conversation's agent runs, newest first.
*/
func (c *AIController) Runs(ctx context.Context, in *requests.AIRunsRequest) (*AIRunsOutput, error) {
	m := membership(ctx)

	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	runs, err := models.New(c.DB).ListConversationRuns(ctx, models.ListConversationRunsParams{
		WorkspaceID: m.Workspace.ID, ConversationID: &conv,
	})
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	out := make([]resources.AiRun, len(runs))
	for i, r := range runs {
		out[i] = resources.NewAiRun(r)
	}
	return &AIRunsOutput{Body: fhttp.NewCollection(pagination.Page[resources.AiRun]{Items: out, Limit: 100})}, nil
}

/*
Run shows one run with every traced step.
*/
func (c *AIController) Run(ctx context.Context, in *requests.AIRunRequest) (*AIRunOutput, error) {
	m := membership(ctx)

	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	runID, err := c.IDs.Decode(string(models.PrefixAiRun), in.Run)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	run, err := models.New(c.DB).FindRun(ctx, models.FindRunParams{WorkspaceID: m.Workspace.ID, ID: runID})
	if err != nil || run.ConversationID == nil || *run.ConversationID != conv {
		return nil, exceptions.ErrNotFound
	}

	steps, err := models.New(c.DB).ListRunSteps(ctx, models.ListRunStepsParams{
		WorkspaceID: m.Workspace.ID, RunID: run.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	out := resources.NewAiRun(run)
	out.Steps = make([]resources.AiRunStep, len(steps))
	for i, st := range steps {
		out.Steps[i] = resources.NewAiRunStep(st)
	}

	return &AIRunOutput{Body: fhttp.NewResource(out)}, nil
}

/*
Usage answers the month's spend tallies.
*/
func (c *AIController) Usage(ctx context.Context, _ *requests.AIAgentRequest) (*AIUsageOutput, error) {
	m := membership(ctx)

	now := c.Clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	period, _ := models.New(c.DB).GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: m.Workspace.ID, PeriodStart: database.Date(start),
	})
	period.PeriodStart = database.Date(start)

	daily, err := models.New(c.DB).ListDailySpend(ctx, models.ListDailySpendParams{
		WorkspaceID: m.Workspace.ID, PeriodStart: start,
	})
	if err != nil {
		return nil, fmt.Errorf("list daily spend: %w", err)
	}
	byModel, err := models.New(c.DB).ListSpendByModel(ctx, models.ListSpendByModelParams{
		WorkspaceID: m.Workspace.ID, PeriodStart: start,
	})
	if err != nil {
		return nil, fmt.Errorf("list model spend: %w", err)
	}
	byOutcome, err := models.New(c.DB).ListSpendByOutcome(ctx, models.ListSpendByOutcomeParams{
		WorkspaceID: m.Workspace.ID, PeriodStart: start,
	})
	if err != nil {
		return nil, fmt.Errorf("list outcome spend: %w", err)
	}

	return &AIUsageOutput{Body: fhttp.NewResource(
		resources.NewAiUsage(period, daily, byModel, byOutcome),
	)}, nil
}
