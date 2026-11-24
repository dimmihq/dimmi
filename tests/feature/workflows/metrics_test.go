package workflows_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
published creates a workflow from the definition, publishes it and
returns its id.
*/
func published(t *testing.T, d tests.Desk, name string, spec testkit.JSON) string {
	t.Helper()

	id := create(t, d, testkit.JSON{"name": name, "spec": spec})
	d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()

	return id
}

/*
workflowMetrics is the part of the analytics the tests read.
*/
type workflowMetrics struct {
	Data struct {
		Triggered  int    `json:"triggered"`
		Engaged    int    `json:"engaged"`
		Completed  int    `json:"completed"`
		Active     int    `json:"active"`
		HandedOver int    `json:"handed_over"`
		Failed     int    `json:"failed"`
		GoalMet    *int   `json:"goal_met"`
		Version    *int32 `json:"version"`
		Goal       *struct {
			Kind   string `json:"kind"`
			Window string `json:"window"`
		} `json:"goal"`
		Days  []struct{ Triggered int } `json:"days"`
		Steps map[string]struct {
			Reached int            `json:"reached"`
			Waiting int            `json:"waiting"`
			Failed  int            `json:"failed"`
			Dropped int            `json:"dropped"`
			Paths   map[string]int `json:"paths"`
		} `json:"steps"`
	} `json:"data"`
}

func TestWorkflowAnalytics(t *testing.T) {
	t.Parallel()

	t.Run("counts runs, paths, drop-off and the version that handled each run", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		id := published(t, d, "Route by topic", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_ask0001", "buttons", testkit.JSON{"text": "What is it about?"},
				path("p_billing", "Billing", step("s_assign1", "assign", testkit.JSON{"mode": "team", "team_id": team})),
				path("p_other1", "Something else"))},
		})

		answered := conversation(t, d, "ada", "Hello")
		d.TC.Clock.Advance(time.Minute)
		d.Owner.PostJSON(answered+"/messages", testkit.JSON{"body": "1"}).AssertCreated()
		d.TC.Drain()

		left := conversation(t, d, "bo", "Hello")
		d.Owner.PostJSON(left+"/replies", testkit.JSON{"body": "I'm here"}).AssertCreated()
		d.TC.Drain()

		conversation(t, d, "cy", "Hello")

		var m workflowMetrics
		d.Owner.GetJSON("/api/v1/workflows/" + id + "/metrics").AssertOK().Decode(&m)
		assert.Equal(t, 3, m.Data.Triggered)
		assert.Equal(t, 1, m.Data.Engaged)
		assert.Equal(t, 1, m.Data.Completed)
		assert.Equal(t, 1, m.Data.Active)
		assert.Equal(t, 1, m.Data.HandedOver)
		assert.Zero(t, m.Data.Failed)
		assert.Nil(t, m.Data.GoalMet, "no goal, no goal count")
		assert.Nil(t, m.Data.Version)
		require.Len(t, m.Data.Days, 1)

		ask := m.Data.Steps["s_ask0001"]
		assert.Equal(t, 3, ask.Reached)
		assert.Equal(t, 1, ask.Waiting)
		assert.Equal(t, 1, ask.Dropped)
		assert.Equal(t, map[string]int{"p_billing": 1}, ask.Paths)
		assert.Equal(t, 1, m.Data.Steps["s_assign1"].Reached)

		var v1, v2 workflowMetrics
		d.Owner.GetJSON("/api/v1/workflows/" + id + "/metrics?version=1").AssertOK().Decode(&v1)
		assert.Equal(t, 3, v1.Data.Triggered)
		d.Owner.GetJSON("/api/v1/workflows/" + id + "/metrics?version=2").AssertOK().Decode(&v2)
		assert.Zero(t, v2.Data.Triggered)
		assert.Empty(t, v2.Data.Steps)

		d.Owner.GetJSON("/api/v1/workflows/"+id+"/runs").AssertOK().
			AssertJSONPath("data.0.version_number", float64(1))
	})

	t.Run("lists the runs whose call failed, with the conversation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("GET https://api.example.com/*", httpclient.Response{Status: 503})
		id := published(t, d, "Lookup", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_http0001", "http_request", testkit.JSON{
				"method": "GET", "url": "https://api.example.com/people", "timeout": "2s",
			})},
		})
		conv := conversation(t, d, "di", "Hello")

		var m workflowMetrics
		d.Owner.GetJSON("/api/v1/workflows/" + id + "/metrics").AssertOK().Decode(&m)
		assert.Equal(t, 1, m.Data.Failed)
		assert.Equal(t, 1, m.Data.Steps["s_http0001"].Failed)

		res := d.Owner.GetJSON("/api/v1/workflows/" + id + "/failures").AssertOK()
		res.AssertJSONPath("data.0.object", "workflow_failure").
			AssertJSONPath("data.0.step_id", "s_http0001").
			AssertJSONPath("data.0.step_kind", "http_request").
			AssertJSONPath("data.0.outcome", "failed").
			AssertJSONPath("data.0.version_number", float64(1)).
			AssertJSONPath("data.0.conversation_id", conv[len("/api/v1/conversations/"):])

		d.Owner.GetJSON("/api/v1/workflows/"+id+"/failures?step_id=s_other01").AssertOK().
			AssertJSONPath("data.#", float64(0))
	})

	t.Run("counts the goal of conversations resolved without a handover", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := published(t, d, "Raise", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"goal":  testkit.JSON{"kind": "resolved", "window": "3d"},
			"steps": []testkit.JSON{step("s_prio001", "priority", testkit.JSON{"value": "high"})},
		})

		closed := conversation(t, d, "ed", "Hello")
		conversation(t, d, "fi", "Hello")
		d.TC.Clock.Advance(time.Hour)
		d.Owner.PatchJSON(closed, testkit.JSON{"status": "closed"}).AssertOK()
		d.TC.Drain()
		refreshFacts(t, d)

		var m workflowMetrics
		d.Owner.GetJSON("/api/v1/workflows/" + id + "/metrics").AssertOK().Decode(&m)
		assert.Equal(t, 2, m.Data.Triggered)
		require.NotNil(t, m.Data.Goal)
		assert.Equal(t, "resolved", m.Data.Goal.Kind)
		assert.Equal(t, "3d", m.Data.Goal.Window)
		require.NotNil(t, m.Data.GoalMet)
		assert.Equal(t, 1, *m.Data.GoalMet)
	})

	t.Run("refuses a goal it cannot count", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Rated", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"goal":  testkit.JSON{"kind": "csat", "window": "3d"},
			"steps": []testkit.JSON{step("s_prio001", "priority", testkit.JSON{"value": "high"})},
		}})

		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertStatus(422)
	})
}

/*
refreshFacts works out every changed conversation's facts at once.
*/
func refreshFacts(t *testing.T, d tests.Desk) {
	t.Helper()
	far := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	for range 10 {
		n, err := d.TC.App.Actions.ReportMaintenance.Drain(t.Context(), far)
		require.NoError(t, err)

		if n == 0 {
			return
		}
	}
}
