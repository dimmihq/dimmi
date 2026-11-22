package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
offered is every tool the scripted models were offered, with what each
says, so a test can see what an agent could do.
*/
func offered(l *fake.LLM, from int) string {
	var b strings.Builder
	for _, req := range l.Sent()[from:] {
		for _, tool := range req.Tools {
			b.WriteString(tool.Name + ": " + tool.Description + "\n")
		}
		for _, s := range req.System {
			b.WriteString(s.Text + "\n")
		}
	}
	return b.String()
}

func TestAiAgentSimulations(t *testing.T) {
	t.Parallel()

	t.Run("previews the draft, and the live version only once there is one", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		h.d.Owner.PatchJSON(path, testkit.JSON{"guidance": "Always mention the 30-day guarantee."}).AssertOK()
		h.llm.answer.Script(reply("Hello there, we have a 30-day guarantee."))

		res := h.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": "Can I return this?"}).AssertOK()

		res.AssertJSONPath("data.object", "ai_agent_preview")
		res.AssertJSONPath("data.reply", "Hello there, we have a 30-day guarantee.")
		assert.Contains(t, res.String("data.run_id"), "air_")
		assert.Contains(t, sentText(h.llm.answer), "30-day guarantee", "the draft's guidance reaches the model")

		h.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": "hi", "version": "live"}).
			AssertValidationError("version")
		h.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": ""}).AssertValidationError("message")
	})

	t.Run("offers only the procedures the agent may follow", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		path := agentPath(t, r.d)
		r.llm.answer.Script(reply("Sorry to hear that."), reply("Sorry to hear that."))

		r.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": "My order arrived broken"}).AssertOK()
		assert.Contains(t, offered(r.llm.answer, 0), "follow_procedure:", "every procedure while the scope is all")

		before := len(r.llm.answer.Sent())
		r.d.Owner.PatchJSON(path, testkit.JSON{"procedure_scope": testkit.JSON{"mode": "selected"}}).AssertOK()
		r.llm.answer.Script(reply("Sorry to hear that."))
		r.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": "My order arrived broken"}).AssertOK()
		assert.NotContains(t, offered(r.llm.answer, before), "follow_procedure:", "no procedure once none is picked")
	})

	t.Run("spells out the chips in the guidance by name", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		path := agentPath(t, r.d)
		r.d.Owner.PatchJSON(path, testkit.JSON{
			"guidance": "Broken orders: follow [[sub_procedure:" + r.proc + "]]. Gone: [[tag:tag_missing]].",
		}).AssertOK()
		r.llm.answer.Script(reply("Sorry to hear that."))

		r.d.Owner.PostJSON(path+"/preview", testkit.JSON{"message": "My order arrived broken"}).AssertOK()

		sent := sentText(r.llm.answer)
		assert.Contains(t, sent, `follow the "Refunds" procedure (start it with follow_procedure).`)
		assert.Contains(t, sent, "Gone: something that no longer exists.")
		assert.NotContains(t, sent, "[[sub_procedure:")
	})

	t.Run("saves simulations, runs them on the draft and keeps the result there", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		h.llm.answer.Script(reply("Growth is $29 a seat each month."))

		made := h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
			"name":       "Quotes the price",
			"messages":   []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
		}).AssertCreated()
		made.AssertJSONPath("data.object", "ai_agent_simulation")
		made.AssertJSONPath("data.draft_result", nil)
		id := made.String("data.id")
		assert.True(t, strings.HasPrefix(id, "aias_"))

		ran := h.d.Owner.PostJSON(path+"/simulations/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", true)
		ran.AssertJSONPath("data.0.live_result", nil)
		ran.AssertJSONPath("data.0.draft_result.turns.0.reply", "Growth is $29 a seat each month.")

		h.d.Owner.PatchJSON(path+"/simulations/"+id, testkit.JSON{"name": "Quotes the Growth price"}).AssertOK().
			AssertJSONPath("data.name", "Quotes the Growth price")
		h.d.Owner.GetJSON(path+"/simulations").AssertOK().AssertJSONPath("data.#", 1)
		h.d.Owner.Delete(path + "/simulations/" + id).AssertNoContent()
		h.d.Owner.GetJSON(path + "/simulations/" + id).AssertNotFound()
	})

	t.Run("saves a real conversation as a simulation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		conv, _ := h.conversation("My parcel is two weeks late")

		res := h.d.Owner.PostJSON(path+"/simulations/from-conversation", testkit.JSON{
			"conversation_id": models.PublicID(models.PrefixConversation, conv),
		}).AssertCreated()

		res.AssertJSONPath("data.messages.0.text", "My parcel is two weeks late")
		res.AssertJSONPath("data.source_conversation_id", models.PublicID(models.PrefixConversation, conv))
	})

	t.Run("lets members read simulations and only admins run them", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		member, _ := h.d.Join(t, enums.RoleMember)

		member.GetJSON(path + "/simulations").AssertOK()
		member.PostJSON(path+"/preview", testkit.JSON{"message": "hi"}).AssertForbidden()
		member.PostJSON(path+"/simulations/run", nil).AssertForbidden()
	})
}
