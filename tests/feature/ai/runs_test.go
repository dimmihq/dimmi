package ai_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/ids"

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
		one.AssertJSONMissing("data.cost_micros")
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
		one.AssertJSONMissing("data.steps.0.cost_micros")
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
