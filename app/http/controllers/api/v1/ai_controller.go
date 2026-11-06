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
	"time"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/jackc/pgx/v5/pgtype"
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
	DB              *database.DB
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
	if b.UnknownBehavior != nil {
		agent.UnknownBehavior = *b.UnknownBehavior
	}
	if b.AssignedBehavior != nil {
		agent.AssignedBehavior = *b.AssignedBehavior
	}
	if b.Streaming != nil {
		agent.Streaming = *b.Streaming
	}
	if b.IncludeContact != nil {
		agent.IncludeContactContext = *b.IncludeContact
	}
	if b.HandoffTeamID != nil || b.RaisePriority != nil {
		handoff := map[string]any{}
		_ = json.Unmarshal(agent.Handoff, &handoff)
		if b.HandoffTeamID != nil {
			if *b.HandoffTeamID == "" {
				delete(handoff, "target_team_id")
			} else {
				handoff["target_team_id"] = *b.HandoffTeamID
			}
		}
		if b.RaisePriority != nil {
			handoff["raise_priority"] = *b.RaisePriority
		}
		raw, _ := json.Marshal(handoff)
		agent.Handoff = raw
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

/*
AIRunsOutput is a conversation's run list.
*/
type AIRunsOutput struct {
	Body fhttp.Collection[resources.AiRun]
}

/*
AIRunOutput is one run with its steps.
*/
type AIRunOutput struct {
	Body fhttp.Resource[resources.AiRun]
}

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
AIUsageOutput is the month's usage view.
*/
type AIUsageOutput struct {
	Body fhttp.Resource[resources.AiUsage]
}

/*
Usage answers the month's spend tallies.
*/
func (c *AIController) Usage(ctx context.Context, _ *requests.AIAgentRequest) (*AIUsageOutput, error) {
	m := membership(ctx)

	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	period, _ := models.New(c.DB).GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: m.Workspace.ID, PeriodStart: pgtype.Date{Time: start, Valid: true},
	})
	period.PeriodStart = pgtype.Date{Time: start, Valid: true}

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
