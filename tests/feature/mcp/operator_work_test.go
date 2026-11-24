package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/tests"
)

/*
worker is an agent connected with every ability, and the desk it works
in.
*/
type worker struct {
	t *testing.T
	d tests.Desk
	c *mcp.Client
}

/*
newWorker connects an agent that may do everything its teammate may.
*/
func newWorker(t *testing.T) worker {
	t.Helper()
	d := tests.NewDesk(t)
	_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

	return worker{t: t, d: d, c: agent(t, d, token)}
}

/*
call runs a tool and returns its result.
*/
func (w worker) call(tool string, args map[string]any) mcp.ToolResult {
	w.t.Helper()
	res, err := w.c.CallTool(w.t.Context(), tool, args)
	require.NoError(w.t, err)

	return res
}

/*
ok runs a tool that must work and returns its structured data.
*/
func (w worker) ok(tool string, args map[string]any) testkit.JSON {
	w.t.Helper()
	res := w.call(tool, args)
	require.False(w.t, res.IsError, res.Text)

	var body struct {
		Data any `json:"data"`
	}
	require.NoError(w.t, json.Unmarshal(res.Structured, &body))
	if record, isRecord := body.Data.(map[string]any); isRecord {
		return record
	}

	return testkit.JSON{"items": body.Data}
}

/*
inner reads a record nested in a tool's data.
*/
func inner(data testkit.JSON, key string) map[string]any {
	record, _ := data[key].(map[string]any)

	return record
}

/*
routine creates the help center audit routine and returns its id.
*/
func (w worker) routine() string {
	w.t.Helper()

	return w.d.Owner.PostJSON("/api/v1/operator/routines", testkit.JSON{
		"template_key": operator.TemplateStaleness,
	}).AssertCreated().String("data.id")
}

/*
pending counts the suggestions that wait for review, through the API.
*/
func (w worker) pending() int {
	w.t.Helper()
	res := w.d.Owner.GetJSON("/api/v1/operator/changesets?filter[status]=pending").AssertOK()

	var body struct {
		Data []any `json:"data"`
	}
	res.Decode(&body)

	return len(body.Data)
}

func TestOperatorWork(t *testing.T) {
	t.Parallel()

	t.Run("runs the help center audit's steps end to end", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days to reach your card.")
		routine := w.routine()
		center := w.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")

		listed := w.call("dimmi_list_help_center_articles", map[string]any{"help_center": center})
		assert.Contains(t, listed.Text, id)
		read := w.call("dimmi_get_article_by_id", map[string]any{"article": id})
		assert.Contains(t, read.Text, "Refunds take 5 days")
		assert.False(t, w.call("dimmi_search_knowledge", map[string]any{"search_text": "refund"}).IsError)
		assert.Equal(t, testkit.JSON{}, w.ok("dimmi_get_routine_state", map[string]any{"routine": routine})["state"])
		assert.Empty(t, w.ok("dimmi_list_recent_suggestions", map[string]any{"routine_id": routine})["items"])

		made := w.ok("dimmi_propose_article_edit", map[string]any{
			"article_id": id, "suggestion_title": "Fix the refund window", "reason": "Refunds take 10 days now.",
			"edits":     []map[string]any{{"find": "Refunds take 5 days", "replace": "Refunds take 10 days"}},
			"dedup_key": "stale:" + id, "routine_id": routine,
		})
		assert.Equal(t, true, made["made"])
		assert.Equal(t, 1, w.pending())

		recent := w.call("dimmi_list_recent_suggestions", map[string]any{"routine_id": routine})
		assert.Contains(t, recent.Text, "stale:"+id)
		assert.Contains(t, recent.Text, "Fix the refund window")

		saved := w.ok("dimmi_save_routine_state", map[string]any{
			"routine": routine, "state": map[string]any{"next_offset": 15},
		})
		assert.Equal(t, testkit.JSON{"next_offset": 15.0}, saved["state"])
		assert.Equal(t, testkit.JSON{"next_offset": 15.0},
			w.ok("dimmi_get_routine_state", map[string]any{"routine": routine})["state"])

		quiet := w.ok("dimmi_report_nothing_to_change", map[string]any{
			"routine": routine, "reason": "Read 1 article; nothing else is stale.",
		})
		assert.Equal(t, true, quiet["recorded"])
		w.d.Owner.GetJSON("/api/v1/operator/routines/"+routine+"/runs").AssertOK().
			AssertJSONPath("data.0.outcome", "noop").
			AssertJSONPath("data.0.noop_reason", "Read 1 article; nothing else is stale.")
		assert.Equal(t, "Refunds take 5 days to reach your card.",
			w.d.Owner.GetJSON("/api/v1/articles/"+id).AssertOK().String("data.body_markdown"))
	})

	t.Run("matches a passage whose wording slipped and says what it matched", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 business days to reach your card after approval.")

		made := w.ok("dimmi_propose_article_edit", map[string]any{
			"article_id": id, "suggestion_title": "Fix the refund window", "reason": "It is 10 days now.",
			"edits": []map[string]any{{
				"find":    "Refunds take 5 business days to reach the card after approval.",
				"replace": "Refunds take 10 business days to reach your card after approval.",
			}},
		})

		assert.Equal(t, true, made["made"])
		assert.Contains(t, made["message"], "to reach your card")
	})

	t.Run("does not make the same suggestion twice", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days.")
		args := map[string]any{
			"article_id": id, "suggestion_title": "Fix the refund window", "reason": "It is 10 days now.",
			"edits":     []map[string]any{{"find": "5 days", "replace": "10 days"}},
			"dedup_key": "stale:" + id,
		}
		w.ok("dimmi_propose_article_edit", args)

		again := w.ok("dimmi_propose_article_edit", args)

		assert.Equal(t, false, again["made"])
		assert.Contains(t, again["message"], "still waits for review")
		assert.Equal(t, 1, w.pending())
	})
}
