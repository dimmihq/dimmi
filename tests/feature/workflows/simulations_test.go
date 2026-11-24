package workflows_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/tests"
)

/*
tapper plays the customer of a workflow simulation: it taps the button
it is told to, answers a question with its answer, or says nothing.
*/
type tapper struct {
	mu     sync.Mutex
	tap    int
	answer string
	asked  []pipeline.CustomerTurn
}

func (p *tapper) PlayCustomer(_ context.Context, in pipeline.CustomerTurn) (pipeline.CustomerMove, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.asked = append(p.asked, in)
	switch {
	case len(in.Choices) > 0 && p.tap > 0:
		return pipeline.CustomerMove{Message: in.Choices[p.tap-1], Choice: p.tap}, nil
	case p.answer != "":
		return pipeline.CustomerMove{Message: p.answer}, nil
	case in.CanWait:
		return pipeline.CustomerMove{Waited: true}, nil
	}
	return pipeline.CustomerMove{Done: true}, nil
}

func (p *tapper) DescribeCustomer(context.Context, pipeline.CustomerProfile) (string, error) {
	return "A customer with a billing question.", nil
}

/*
simDesk is a desk whose simulated customer is the tapper.
*/
func simDesk(t *testing.T, p *tapper) tests.Desk {
	t.Helper()
	d := tests.NewDesk(t)
	require.NotNil(t, d.TC.App.Actions.Procedures)
	d.TC.App.Actions.Procedures.Customer = p
	return d
}

/*
menu is a workflow that asks what the conversation is about, tags and
answers billing, and collects an email for anything else.
*/
func menu(tag string) testkit.JSON {
	return testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{
			step("s_ask0001", "buttons", testkit.JSON{
				"text":    "Hi {{contact.first_name}}, what is it about?",
				"timeout": "1h",
			},
				path("p_billing", "Billing",
					step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}}),
					step("s_wait001", "wait", testkit.JSON{"for": "2h"}),
					step("s_thanks1", "message", testkit.JSON{"text": "Thanks, billing will reply soon."})),
				path("p_other1", "Something else",
					step("s_email01", "collect", testkit.JSON{
						"text": "What's your email?", "field": "run.vars.email",
						"type": "email",
					}),
					step("s_bye0001", "message", testkit.JSON{"text": "We'll write to {{run.vars.email}}."})),
				testkit.JSON{"id": "p_timeout", "outcome": "timeout", "steps": []testkit.JSON{
					step("s_late001", "message", testkit.JSON{"text": "We'll be in touch."}),
				}}),
		},
	}
}

func TestWorkflowSimulations(t *testing.T) {
	t.Parallel()

	t.Run("taps a button, skips the wait, records the tag and judges the checks", func(t *testing.T) {
		t.Parallel()
		player := &tapper{tap: 1}
		d := simDesk(t, player)
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		wf := create(t, d, testkit.JSON{"name": "Menu", "spec": menu(tag)})
		sims := "/api/v1/workflows/" + wf + "/simulations"

		made := d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Billing question", "persona": testkit.JSON{"name": "Ada Lovelace"},
			"messages": []testkit.JSON{{"text": "Hello there"}},
			"assertions": []testkit.JSON{
				{"type": "workflow_started"},
				{"type": "path_taken", "step_id": "s_ask0001", "branch": "p_billing"},
				{"type": "action_called", "action": "tag"},
				{"type": "step_reached", "step_id": "s_thanks1"},
				{"type": "reply_contains", "text": "billing will reply"},
			},
		}).AssertCreated()
		made.AssertJSONPath("data.object", "workflow_simulation")
		id := made.String("data.id")
		assert.True(t, strings.HasPrefix(id, "wfs_"))

		ran := d.Owner.PostJSON(sims+"/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", true)
		ran.AssertJSONPath("data.0.draft_result.trace.verdict.started", true)
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.path_id", "p_billing")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.path_label", "Billing")
		ran.AssertJSONPath("data.0.draft_result.trace.steps.2.skipped", true)
		ran.AssertJSONPath("data.0.draft_result.trace.actions.0.kind", "tag")
		ran.AssertJSONPath("data.0.draft_result.transcript.1.text", "Hi Ada, what is it about?")
		ran.AssertJSONPath("data.0.draft_result.transcript.2.tapped", true)
		ran.AssertJSONPath("data.0.draft_history.state", "passing")
		ran.AssertJSONPath("data.0.live_result", nil)
		require.NotEmpty(t, player.asked)
		assert.Equal(t, []string{"Billing", "Something else"}, player.asked[0].Choices)

		d.Owner.GetJSON("/api/v1/conversations?limit=5").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON(sims+"/"+id+"/runs").AssertOK().AssertJSONPath("data.0.counted", true)

		d.Owner.PostJSON(sims+"/run", testkit.JSON{"version": "live"}).AssertValidationError("version")
		d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()
		d.Owner.PostJSON(sims+"/run", testkit.JSON{"version": "live"}).AssertOK().
			AssertJSONPath("data.0.live_result.passed", true)

		d.Owner.Delete(sims + "/" + id).AssertNoContent()
		d.Owner.GetJSON(sims + "/" + id).AssertNotFound()
	})

	t.Run("lets the timeout pass when the customer says nothing", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		wf := create(t, d, testkit.JSON{"name": "Menu", "spec": menu(tag)})
		sims := "/api/v1/workflows/" + wf + "/simulations"
		d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Silent", "messages": []testkit.JSON{{"text": "Hello"}},
			"assertions": []testkit.JSON{{"type": "path_taken", "step_id": "s_ask0001", "branch": "p_billing"}},
		}).AssertCreated()

		ran := d.Owner.PostJSON(sims+"/run", nil).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", false)
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.path_id", "p_timeout")
		ran.AssertJSONPath("data.0.draft_result.transcript.2.silent", true)
		ran.AssertJSONPath("data.0.draft_history.state", "failing")
	})

	t.Run("answers a collect step and keeps the value in the run's variables", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{tap: 2, answer: "ada@example.com"})
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		wf := create(t, d, testkit.JSON{"name": "Menu", "spec": menu(tag)})
		sims := "/api/v1/workflows/" + wf + "/simulations"
		d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Other", "messages": []testkit.JSON{{"text": "Hello"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "write to ada@example.com"}},
		}).AssertCreated()

		ran := d.Owner.PostJSON(sims+"/run", nil).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", true)
		ran.AssertJSONPath("data.0.draft_result.trace.vars.email", "ada@example.com")
	})

	t.Run("refuses a path check without its path, and a workflow of another workspace", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		wf := create(t, d, testkit.JSON{"name": "Menu", "spec": menu("tag_none")})
		sims := "/api/v1/workflows/" + wf + "/simulations"
		d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Bad", "messages": []testkit.JSON{{"text": "Hi"}},
			"assertions": []testkit.JSON{{"type": "path_taken", "step_id": "s_ask0001"}},
		}).AssertValidationError("assertions")
		d.Owner.GetJSON("/api/v1/workflows/wf_nothing/simulations").AssertNotFound()
	})
}

func TestWorkflowSimulationAgentSteps(t *testing.T) {
	t.Parallel()

	t.Run("an agent step whose agent cannot answer takes the handed-over path", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{})
		wf := create(t, d, testkit.JSON{"name": "Agent", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_dimmi01", "dimmi", testkit.JSON{},
				testkit.JSON{"id": "p_handed1", "outcome": "handed_over", "steps": []testkit.JSON{
					step("s_sorry01", "message", testkit.JSON{"text": "A teammate will take it from here."}),
				}})},
		}})
		sims := "/api/v1/workflows/" + wf + "/simulations"
		d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Hands over", "messages": []testkit.JSON{{"text": "I need help"}},
			"assertions": []testkit.JSON{{"type": "handed_off"}},
		}).AssertCreated()

		ran := d.Owner.PostJSON(sims+"/run", nil).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.trace.steps.0.path_id", "p_handed1")
		ran.AssertJSONPath("data.0.draft_result.passed", true)
	})
}
