package ai_test

import (
	"context"
	"testing"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestAiRuns(t *testing.T) {
	t.Parallel()

	t.Run("lists a conversation's runs and shows one with its steps", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		conv, convID := conversationOf(t, d, "jane")
		runID := seedRun(t, d, agent, run{
			conversation: &convID, outcome: "answered", cost: 900, model: "claude-a",
			at: d.TC.Clock.Now(), payload: true,
		})
		public := models.PublicID(models.PrefixAiRun, runID)

		list := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/ai/runs").AssertOK()
		list.AssertJSONPath("data.#", 1)
		list.AssertJSONPath("data.0.id", public)
		list.AssertJSONPath("data.0.conversation_id", conv)

		one := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/ai/runs/" + public).AssertOK()
		one.AssertJSONPath("data.outcome", "answered")
		one.AssertJSONPath("data.cost_micros", 900)
		one.AssertJSONPath("data.payloads_visible", true)
		one.AssertJSONPath("data.steps.0.model", "claude-a")
		one.AssertJSONPath("data.steps.0.request.prompt", "secret prompt")
	})

	t.Run("shows members the numbers and never the payloads", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		conv, convID := conversationOf(t, d, "jane")
		runID := seedRun(t, d, agent, run{
			conversation: &convID, outcome: "answered", cost: 900, model: "claude-a",
			at: d.TC.Clock.Now(), payload: true,
		})
		member, _ := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)
		path := "/api/v1/conversations/" + conv + "/ai/runs/" + models.PublicID(models.PrefixAiRun, runID)

		one := member.GetJSON(path).AssertOK()
		one.AssertJSONPath("data.payloads_visible", false)
		one.AssertJSONPath("data.steps.0.cost_micros", 900)
		one.AssertJSONPath("data.steps.0.request", nil)
		one.AssertJSONPath("data.steps.0.response", nil)

		admin.GetJSON(path).AssertOK().AssertJSONPath("data.steps.0.request.prompt", "secret prompt")
	})

	t.Run("answers not found for a run of another conversation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		_, convID := conversationOf(t, d, "jane")
		other, _ := conversationOf(t, d, "joe")
		runID := seedRun(t, d, agent, run{conversation: &convID, outcome: "answered", at: d.TC.Clock.Now()})

		d.Owner.GetJSON("/api/v1/conversations/" + other + "/ai/runs/" + models.PublicID(models.PrefixAiRun, runID)).
			AssertNotFound()
		d.Owner.GetJSON("/api/v1/conversations/" + models.PublicID(models.PrefixConversation, ids.New()) + "/ai/runs").
			AssertNotFound()
	})
}

/*
playground is a stand-in for the pipeline's playground entry that
answers every question the same way.
*/
type playground struct {
	asked []contracts.PlaygroundQuestion
}

/*
Ask records the question and answers it.
*/
func (p *playground) Ask(_ context.Context, q contracts.PlaygroundQuestion) (*contracts.PlaygroundAnswer, error) {
	p.asked = append(p.asked, q)

	return &contracts.PlaygroundAnswer{
		Outcome:        "answered",
		Classification: contracts.PlaygroundClassification{Intent: "question", Language: "en", Query: q.Message},
		Retrieved:      []contracts.PlaygroundSource{{N: 1, Title: "Billing", URL: "https://help.acme.com", Used: true}},
		Answer:         "Go to Settings > Billing [1].",
		Citations:      []contracts.PlaygroundCitation{{N: 1, Title: "Billing", URL: "https://help.acme.com"}},
		InputTokens:    120, OutputTokens: 30, CostMicros: 450, LatencyMs: 800,
	}, nil
}

func TestAiPlayground(t *testing.T) {
	t.Parallel()

	t.Run("answers a test question with everything the agent did", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := &playground{}
		d.TC.App.Actions.AI.AskPlayground.Playground = fake
		conv, convID := conversationOf(t, d, "jane")

		res := d.Owner.PostJSON("/api/v1/ai/playground", testkit.JSON{
			"message": "How do I change billing?", "conversation": conv,
		}).AssertOK()

		res.AssertJSONPath("data.object", "ai_playground_answer")
		res.AssertJSONPath("data.outcome", "answered")
		res.AssertJSONPath("data.classification.query", "How do I change billing?")
		res.AssertJSONPath("data.retrieved.0.title", "Billing")
		res.AssertJSONPath("data.answer.body", "Go to Settings > Billing [1].")
		res.AssertJSONPath("data.answer.citations.0.n", 1)
		res.AssertJSONPath("data.usage.cost_micros", 450)
		res.AssertJSONPath("data.timing.latency_ms", 800)
		if len(fake.asked) != 1 || fake.asked[0].ConversationID == nil || *fake.asked[0].ConversationID != convID {
			t.Fatalf("the playground was not asked about the conversation: %+v", fake.asked)
		}
		d.TC.Table("messages").AssertCount(1, testkit.Row{"conversation_id": convID})
	})

	t.Run("lets members try the agent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.TC.App.Actions.AI.AskPlayground.Playground = &playground{}
		member, _ := d.Join(t, enums.RoleMember)

		member.PostJSON("/api/v1/ai/playground", testkit.JSON{"message": "hi"}).AssertOK()
	})

	t.Run("refuses an empty question or an unknown conversation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.TC.App.Actions.AI.AskPlayground.Playground = &playground{}

		d.Owner.PostJSON("/api/v1/ai/playground", testkit.JSON{"message": ""}).AssertValidationError("message")
		d.Owner.PostJSON("/api/v1/ai/playground", testkit.JSON{"message": "   "}).AssertValidationError("message")
		d.Owner.PostJSON("/api/v1/ai/playground", testkit.JSON{
			"message": "hi", "conversation": models.PublicID(models.PrefixConversation, ids.New()),
		}).AssertNotFound()
	})

	t.Run("says it is unavailable while nothing answers it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.TC.App.Actions.AI.AskPlayground.Playground = nil

		d.Owner.PostJSON("/api/v1/ai/playground", testkit.JSON{"message": "hi"}).AssertStatus(503)
	})
}
