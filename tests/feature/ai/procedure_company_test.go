package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
TestCompanyFacts covers a procedure that branches on the customer's
company: code decides the branch from the company's details, and the
agent is shown those details so it never asks for them.
*/
func TestCompanyFacts(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil)
	id := h.liveProcedure("Better rates", testkit.JSON{
		"description": "The customer asks for cheaper label rates.",
		"steps": []testkit.JSON{{
			"id": "ent", "kind": "branch", "text": "Their company is on Enterprise",
			"condition": testkit.JSON{"mode": "all", "rules": []testkit.JSON{
				{"field": "company.attr.plan", "operator": "eq", "value": "enterprise"},
			}},
			"then_steps": []testkit.JSON{
				{"id": "contract", "kind": "instruction", "text": "Say their contract sets rates."},
			},
			"else_steps": []testkit.JSON{{"id": "plans", "kind": "instruction", "text": "Explain the plans."}},
		}},
		"handover": []string{},
	})
	args, _ := json.Marshal(map[string]string{"procedure_id": id})
	h.llm.answer.Script(tools(call("follow_procedure", string(args))), reply("Your contract sets your rates."))

	h.d.Owner.PostJSON("/api/v1/procedures/"+id+"/simulate", testkit.JSON{
		"message": "Can we get a better deal?",
		"persona": testkit.JSON{
			"name": "Nadia", "verified": true, "channel": "widget",
			"company": testkit.JSON{"plan": "enterprise", "monthly_labels": 42000},
		},
	}).AssertOK()

	sent := h.llm.answer.Sent()
	require.Len(t, sent, 2)
	second := systemOf(sent[1])
	assert.Contains(t, second, "Decided: Their company is on Enterprise", "code took the side from the company's plan")
	assert.Contains(t, second, "company.attr.plan = enterprise", "and the agent sees what decided it")
	assert.NotContains(t, second, "Explain the plans.")
}

/*
TestHandoverStep covers a step that hands the conversation to a team:
it is finished by handing over, never by the agent's word alone.
*/
func TestHandoverStep(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil)
	team := models.PublicID(models.PrefixTeam, h.team("Enterprise").ID)
	id := h.liveProcedure("Better rates", testkit.JSON{
		"description": "The customer asks for cheaper label rates.",
		"steps": []testkit.JSON{
			{
				"id": "toent", "kind": "instruction",
				"text":     "Tell them the team reviews rates, then [[handoff:" + team + "]].",
				"bindings": []testkit.JSON{{"ref": team, "kind": "handoff"}},
			},
			{"id": "next", "kind": "instruction", "text": "Explain the plans."},
		},
		"handover": []string{},
	})
	args, _ := json.Marshal(map[string]string{"procedure_id": id})
	h.llm.answer.Script(
		tools(call("follow_procedure", string(args))),
		tools(call("update_procedure_status", `{"status":"step_done","step":"1","still_missing":""}`)),
		tools(call("update_procedure_status", `{"status":"step_done","step":"1","still_missing":""}`)),
		reply("The team will review your rates."),
	)

	h.d.Owner.PostJSON("/api/v1/procedures/"+id+"/simulate", testkit.JSON{
		"message": "Can we get a better deal?", "persona": signedInPersona(),
	}).AssertOK()

	sent := h.llm.answer.Sent()
	require.Len(t, sent, 4)
	refused := sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text
	assert.Contains(t, refused, "finished by handing the conversation over", "saying it is done does not finish it")
	again := sent[3].Messages[len(sent[3].Messages)-1].Parts[0].Text
	assert.Contains(t, again, "Do not report it again", "and asking again is cut short")
}
