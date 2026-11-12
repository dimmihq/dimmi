package workflows_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	engine "github.com/dimmihq/dimmi/app/workflows"
	"github.com/dimmihq/dimmi/tests"
)

/*
lookupWithFallback publishes a workflow that saves the customer's plan
from their system and says it, or says the lookup failed.
*/
func lookupWithFallback(t *testing.T, d tests.Desk) string {
	t.Helper()

	return publish(t, d, "Lookup", testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{
			step("s_http0001", "http_request", testkit.JSON{
				"method": "GET", "url": "https://api.example.com/plan",
				"save": []testkit.JSON{{"path": "plan", "field": "run.vars.plan"}},
			}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
				step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
			}}),
			step("s_plan0001", "message", testkit.JSON{"text": "Your plan is {{run.vars.plan}}"}),
		},
	})
}

/*
lookupStep is the request step of a workflow's only run, as its trace
shows it.
*/
func lookupStep(t *testing.T, d tests.Desk, workflow string) (string, map[string]any) {
	t.Helper()

	var runs struct {
		Data []struct {
			Steps []struct {
				Kind    string         `json:"kind"`
				Outcome string         `json:"outcome"`
				Detail  map[string]any `json:"detail"`
			} `json:"steps"`
		} `json:"data"`
	}
	d.Owner.GetJSON("/api/v1/workflows/" + workflow + "/runs").AssertOK().Decode(&runs)
	require.Len(t, runs.Data, 1)

	for _, s := range runs.Data[0].Steps {
		if s.Kind == "http_request" {
			return s.Outcome, s.Detail
		}
	}

	t.Fatal("the run has no request step")

	return "", nil
}

/*
runEraser is the eraser that forgets what runs learned about a contact.
*/
func runEraser(d tests.Desk) engine.RunEraser {
	return engine.RunEraser{Engine: d.TC.App.Actions.WorkflowEngine}
}

func TestCallOutErasure(t *testing.T) {
	t.Parallel()

	t.Run("never keeps an answer about a customer erased during the call", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Serve("GET https://api.example.com/*", fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
				var contact ids.UUID
				if err := d.TC.Operator().QueryRow(r.Context(),
					`SELECT contact_id FROM workflow_runs WHERE workspace_id = $1`, ws).Scan(&contact); err == nil {
					_ = runEraser(d).Erase(r.Context(), ws, contact)
				}

				_, _ = w.Write([]byte(`{"plan":"team"}`))
			}))
		workflow := lookupWithFallback(t, d)

		got := bodies(t, d, conversation(t, d, "ha", "Hello"))
		assert.Equal(t, "Lookup failed", got[len(got)-1])

		outcome, detail := lookupStep(t, d, workflow)
		assert.Equal(t, "failed", outcome)
		assert.Equal(t, "answer_erased", detail["error"])
	})

	t.Run("forgets a kept answer, and the step goes on as if the call failed", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("GET https://api.example.com/*", httpclient.Response{Body: []byte(`{"plan":"team"}`)})
		workflow := lookupWithFallback(t, d)
		path := conversation(t, d, "jo", "Hello")

		// Put the run back where the job left it: waiting at the request
		// with the answer kept and the run not yet resumed.
		var run, contact ids.UUID
		var seq int64
		ctx := t.Context()
		require.NoError(t, d.TC.Operator().QueryRow(ctx, `UPDATE workflow_runs SET status = 'waiting',
			cursor_step_id = 's_http0001', ended_at = NULL, end_reason = NULL, vars = '{}',
			wait = '{"kind":"call_out","step_id":"s_http0001","answer":{"status":200,
				"saves":[{"field":"run.vars.plan","value":"team"}]}}'
			WHERE workspace_id = $1 RETURNING id, contact_id, seq`, ws).Scan(&run, &contact, &seq))
		_, err := d.TC.Operator().Exec(ctx, `UPDATE workflow_run_steps SET status = 'waiting', outcome = NULL,
			detail = '{}', finished_at = NULL WHERE run_id = $1 AND step_id = 's_http0001'`, run)
		require.NoError(t, err)
		_, err = d.TC.Operator().Exec(ctx, `DELETE FROM workflow_run_steps
			WHERE run_id = $1 AND step_id <> 's_http0001'`, run)
		require.NoError(t, err)

		require.NoError(t, runEraser(d).Erase(ctx, ws, contact))

		var kept bool
		require.NoError(t, d.TC.Operator().QueryRow(ctx,
			`SELECT wait ? 'answer' FROM workflow_runs WHERE id = $1`, run).Scan(&kept))
		assert.False(t, kept, "the erased customer's answer is forgotten")

		require.NoError(t, d.TC.App.Actions.WorkflowEngine.AdvanceRun(ctx, jobs.AdvanceWorkflowRunArgs{
			WorkspaceID: ws, RunID: run, Seq: seq, Input: engine.InputCallOutDone,
		}))
		d.TC.Drain()

		got := bodies(t, d, path)
		assert.Equal(t, "Lookup failed", got[len(got)-1])

		outcome, detail := lookupStep(t, d, workflow)
		assert.Equal(t, "failed", outcome)
		assert.Equal(t, "answer_erased", detail["error"])
	})
}
