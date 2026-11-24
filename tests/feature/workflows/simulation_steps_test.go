package workflows_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/tests"
)

/*
simulated saves one simulation of the workflow with the mocks given,
runs it on the draft and returns the run.
*/
func simulated(t *testing.T, d tests.Desk, wf string, mocks testkit.JSON) *testkit.Response {
	t.Helper()
	sims := "/api/v1/workflows/" + wf + "/simulations"
	body := testkit.JSON{"name": "Case", "messages": []testkit.JSON{{"text": "My order A-1234 came broken"}}}
	if mocks != nil {
		body["action_mocks"] = mocks
	}
	id := d.Owner.PostJSON(sims, body).AssertCreated().String("data.id")

	return d.Owner.PostJSON(sims+"/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
}

/*
waiter waits a day after a message, and says something else when the
customer writes first.
*/
func waiter() testkit.JSON {
	return testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{
			step("s_hello01", "message", testkit.JSON{"text": "We'll check in tomorrow."}),
			step("s_wait001", "wait", testkit.JSON{"for": "1d"},
				testkit.JSON{"id": "p_wrote01", "outcome": "interrupted", "steps": []testkit.JSON{
					step("s_wrote01", "message", testkit.JSON{"text": "Thanks for writing back."}),
				}}),
			step("s_later01", "message", testkit.JSON{"text": "Checking in as promised."}),
		},
	}
}

func TestWorkflowSimulationSteps(t *testing.T) {
	t.Parallel()

	t.Run("lets the customer interrupt a wait, or the time pass", func(t *testing.T) {
		t.Parallel()
		player := &tapper{interrupt: "Any news on my order?"}
		d := simDesk(t, player)
		wf := create(t, d, testkit.JSON{"name": "Wait", "spec": waiter()})

		ran := simulated(t, d, wf, nil)
		ran.AssertJSONPath("data.0.draft_result.trace.steps.1.outcome", "interrupted")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.1.path_id", "p_wrote01")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.1.detail.message", "Any news on my order?")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.2.step_id", "s_wrote01")
		ran.AssertJSONPath("data.0.draft_result.transcript.2.text", "Any news on my order?")
		require.NotEmpty(t, player.asked)
		assert.Equal(t, "1d", player.asked[0].Waiting)

		quiet := simDesk(t, &tapper{})
		wf = create(t, quiet, testkit.JSON{"name": "Wait", "spec": waiter()})
		passed := simulated(t, quiet, wf, nil)
		passed.AssertJSONPath("data.0.draft_result.trace.steps.1.skipped", true)
		passed.AssertJSONPath("data.0.draft_result.trace.steps.2.step_id", "s_later01")
	})

	t.Run("walks a called workflow's steps inline, on the caller's variables", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		reusable := publish(t, d, "Ask plan", testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{
				step("s_inner01", "message", testkit.JSON{"text": "Shared, about {{run.vars.order}}"}),
			},
		})
		wf := create(t, d, testkit.JSON{"name": "Caller", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				extractor(nil),
				step("s_call0001", "run_workflow", testkit.JSON{"workflow_id": reusable}),
				step("s_after001", "message", testkit.JSON{"text": "Back in the caller"}),
			},
		}})

		ran := simulated(t, d, wf, testkit.JSON{"s_extr0001": testkit.JSON{"body": testkit.JSON{
			"order": "A-1234", "refund": true,
		}}})
		trace := "data.0.draft_result.trace."
		ran.AssertJSONPath(trace+"steps.0.outcome", "extracted")
		ran.AssertJSONPath(trace+"steps.0.detail.mocked", true)
		ran.AssertJSONPath(trace+"vars.order", "A-1234")
		ran.AssertJSONPath(trace+"vars.refund", true)
		ran.AssertJSONPath(trace+"steps.1.kind", "run_workflow")
		ran.AssertJSONPath(trace+"steps.1.outcome", "completed")
		ran.AssertJSONPath(trace+"steps.1.detail.child_end", "completed")
		ran.AssertJSONPath(trace+"steps.2.step_id", "s_inner01")
		ran.AssertJSONPath(trace+"steps.2.depth", 1)
		ran.AssertJSONPath(trace+"steps.2.workflow", "Ask plan")
		ran.AssertJSONPath(trace+"steps.3.step_id", "s_after001")
		ran.AssertJSONPath("data.0.draft_result.transcript.1.text", "Shared, about A-1234")
	})

	t.Run("asks the extractor about what the customer wrote, or takes the none path", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"order":"A-1234","refund":null}`})
		classifyingModel(t, d, llm)
		wf := create(t, d, testkit.JSON{"name": "Extract", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{extractor(nil)},
		}})

		ran := simulated(t, d, wf, nil)
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.outcome", "none")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.path_id", "p_none0001")
		ran.AssertJSONPath("data.0.draft_result.trace.vars.order", "A-1234")
		sent := llm.Sent()
		require.Len(t, sent, 1)
		assert.Contains(t, sent[0].Messages[0].Parts[0].Text, "My order A-1234 came broken")
	})

	t.Run("answers a run action step with its mock and never calls the action", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		api := httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests()
		action := publishedAction(t, d)
		wf := create(t, d, testkit.JSON{"name": "Lookup", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_act00001", "run_action", testkit.JSON{
					"action_id": action,
					"inputs":    []testkit.JSON{{"name": "email", "value": "{{contact.email}}"}},
					"save":      []testkit.JSON{{"output": "plan", "field": "run.vars.plan"}},
				}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
					step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
				}}),
				step("s_plan0001", "message", testkit.JSON{"text": "Your plan is {{run.vars.plan}}"}),
			},
		}})

		ok := simulated(t, d, wf, testkit.JSON{"s_act00001": testkit.JSON{"status": 200, "body": testkit.JSON{
			"plan": "pro",
		}}})
		ok.AssertJSONPath("data.0.draft_result.trace.steps.0.outcome", "succeeded")
		ok.AssertJSONPath("data.0.draft_result.trace.vars.plan", "pro")
		ok.AssertJSONPath("data.0.draft_result.trace.actions.0.mocked", true)
		ok.AssertJSONPath("data.0.draft_result.trace.actions.0.summary", "Would run the action Look up plan")
		ok.AssertJSONPath("data.0.draft_result.transcript.1.text", "Your plan is pro")

		failed := simulated(t, d, wf, testkit.JSON{action: testkit.JSON{"status": 503}})
		failed.AssertJSONPath("data.0.draft_result.trace.steps.0.path_id", "p_failed01")
		failed.AssertJSONPath("data.0.draft_result.transcript.1.text", "Lookup failed")
		assert.Empty(t, api.Sent(), "a simulation never calls the action")
	})

	t.Run("works out the reply time and names the article", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		center := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		article := d.Owner.PostJSON("/api/v1/help-centers/"+center+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": "Five days.",
		}).AssertCreated().String("data.id")
		wf := create(t, d, testkit.JSON{"name": "Answers", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_reply01", "reply_time", testkit.JSON{"text": "We reply {{reply_time}}."}),
				step("s_artic01", "article", testkit.JSON{"article": article, "text": "This may help."}),
			},
		}})

		ran := simulated(t, d, wf, nil)
		assert.Regexp(t, `^We reply (in a few minutes|within an hour|in a few hours|within a day|`+
			`later today after|tomorrow after|on )`, ran.String("data.0.draft_result.transcript.1.text"))
		ran.AssertJSONPath("data.0.draft_result.transcript.2.text", "This may help.\n\n[Help article: Refunds]")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.1.detail.title", "Refunds")
	})
}
