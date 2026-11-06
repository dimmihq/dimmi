package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
	"github.com/dimmihq/dimmi/tests"
	"github.com/jackc/pgx/v5/pgtype"
	
)

/*
TestAiPipeline runs the agent's machinery end to end on the real
application with a scripted model answering.
*/
func TestAiPipeline(t *testing.T) {
	t.Parallel()

	t.Run("answers a customer question", func(t *testing.T) {
		t.Parallel()

		d := tests.NewDesk(t)
		setup(t, d, fake.New(contracts.LLMProviderAnthropic).Script(
			fake.Step{Text: "You can change your billing email in Settings > Billing. [1]",
				Usage: contracts.LLMUsage{InputTokens: 120, OutputTokens: 42, Reported: true}},
		))

		conv, trigger := conversation(t, d, "billing email")
		ctx := tenancy.With(t.Context(), d.TC.Workspace.Workspace.ID.String())
		res, err := d.TC.App.AI.Run(ctx, pipeline.Args{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ConversationID: conv,
			TriggerMessageID: trigger, Kind: "conversation", QueuedAt: time.Now(),
		})
		require.NoError(t, err)
		assert.Equal(t, "answered", res.Outcome)

		d.TC.InWorkspace(func(ctx context.Context) error {
			run, err := models.New(d.TC.App.DB).FindRun(ctx, models.FindRunParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, ID: res.RunID,
			})
			require.NoError(t, err)
			t.Logf("run: status=%s outcome=%v err=%v msg=%v", run.Status, run.Outcome, str(run.ErrorCode), str(run.ErrorMessage))
			assert.Equal(t, "completed", run.Status)
			assert.Equal(t, "answered", str(run.Outcome))
			assert.Greater(t, run.OutputTokens, int64(0))

			steps, err := models.New(d.TC.App.DB).ListRunSteps(ctx, models.ListRunStepsParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, RunID: run.ID,
			})
			require.NoError(t, err)
			assert.Len(t, steps, 2)

			conv, err := models.New(d.TC.App.DB).FindConversation(ctx, models.FindConversationParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, ID: conv,
			})
			require.NoError(t, err)
			t.Logf("conv: state=%s replies=%d cost=%d lang=%v", conv.AiState, conv.AiReplyCount, conv.AiCostMicros, conv.AiLanguage)
			msgs, _ := models.New(d.TC.App.DB).ListMessages(ctx, models.ListMessagesParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, ConversationID: conv.ID, RowLimit: 20,
			})
			for _, m := range msgs {
				t.Logf("msg %s author=%s type=%s", m.ID, m.AuthorType, m.Type)
			}
			assert.Equal(t, int32(1), conv.AiReplyCount)
			return nil
		})
	})

	t.Run("hands off when the customer asks for a person", func(t *testing.T) {
		t.Parallel()

		d := tests.NewDesk(t)
		setup(t, d, fake.New(contracts.LLMProviderAnthropic))

		conv, trigger := conversation(t, d, "I want to talk to a human please")
		ctx := tenancy.With(t.Context(), d.TC.Workspace.Workspace.ID.String())
		res, err := d.TC.App.AI.Run(ctx, pipeline.Args{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ConversationID: conv,
			TriggerMessageID: trigger, Kind: "conversation", QueuedAt: time.Now(),
		})
		require.NoError(t, err)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "customer_requested", res.HandoffReason)

		d.TC.InWorkspace(func(ctx context.Context) error {
			conv, err := models.New(d.TC.App.DB).FindConversation(ctx, models.FindConversationParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, ID: conv,
			})
			require.NoError(t, err)
			assert.Equal(t, "handed_off", conv.AiState)
			return nil
		})
	})
}

/*
setup registers a scripted driver and points the workspace's agent at
a fake connection.
*/
func setup(t *testing.T, d tests.Desk, llm *fake.LLM) {
	t.Helper()

	d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return llm, nil
	})

	d.TC.InWorkspace(func(ctx context.Context) error {
		now := d.TC.App.Clock.Now()
		key, err := models.New(d.TC.App.DB).CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: d.TC.Workspace.Workspace.ID, Name: "fake", Provider: "fake",
			AuthType: "none", Status: "valid", Models: json.RawMessage("[]"),
			ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
			Source: "system", Now: now,
		})
		if err != nil {
			return err
		}

		answer, classifier := "fake-answer", "fake-classifier"
		minSim := pgtype.Numeric{Int: big.NewInt(35), Exp: -2, Valid: true}
		_, err = models.New(d.TC.App.DB).UpdateAgent(ctx, models.UpdateAgentParams{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ID: agentID(t, d),
			Name: "Dimmi", Enabled: true, Tone: "friendly", Guidance: "",
			EnabledChannels: []string{"api"}, Schedule: "always",
			AssignedBehavior: "skip_when_human_active",
			AnswerKeyID:      &key.ID, AnswerModel: &answer,
			ClassifierKeyID: &key.ID, ClassifierModel: &classifier,
			ExcludedChannelConnectionIds: []ids.UUID{},
			MaxOutputTokens:              600, MaxAiReplies: 5, UnknownBehavior: "offer_handoff",
			GroundingMode: "citations", RequireCitations: false, DefaultLanguage: "en",
			Languages: []string{}, ContactAttributeAllowlist: []string{},
			LinkAllowlist: []string{},
			Streaming:     true, IncludeContactContext: "basic", PiiRedaction: false,
			MinSimilarity: minSim,
			Handoff:       json.RawMessage("{}"), Messages: json.RawMessage("{}"),
			Now: now,
		})
		return err
	})
}

/*
agentID returns the workspace's agent id, creating the row if needed.
*/
func agentID(t *testing.T, d tests.Desk) ids.UUID {
	t.Helper()
	var id ids.UUID
	d.TC.InWorkspace(func(ctx context.Context) error {
		agent, err := models.New(d.TC.App.DB).FindAgent(ctx, d.TC.Workspace.Workspace.ID)
		if err != nil {
			agent, err = models.New(d.TC.App.DB).CreateAgent(ctx, models.CreateAgentParams{
				WorkspaceID: d.TC.Workspace.Workspace.ID, Now: d.TC.App.Clock.Now(),
			})
			if err != nil {
				return err
			}
		}
		id = agent.ID
		return nil
	})
	return id
}

/*
conversation creates a conversation and returns its id and trigger
message id.
*/
func conversation(t *testing.T, d tests.Desk, body string) (ids.UUID, ids.UUID) {
	t.Helper()

	res := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": "help", "body": body,
		"contact": testkit.JSON{"email": "jane@example.com", "name": "Jane"},
	}).AssertCreated()
	publicID := res.String("data.id")

	var convID, msgID ids.UUID
	d.TC.InWorkspace(func(ctx context.Context) error {
		convUUID, err := d.TC.App.IDs.Decode("conv", publicID)
		if err != nil {
			return err
		}
		convID = convUUID

		msgs, err := models.New(d.TC.App.DB).ListMessages(ctx, models.ListMessagesParams{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ConversationID: convID, RowLimit: 1,
		})
		if err != nil || len(msgs) == 0 {
			return fmt.Errorf("trigger message missing: %w", err)
		}
		msgID = msgs[0].ID
		return nil
	})
	return convID, msgID
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
