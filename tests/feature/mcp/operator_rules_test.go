package mcp_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
edit is a suggested fix to the article that swaps 5 days for 10.
*/
func edit(id string, more map[string]any) map[string]any {
	args := map[string]any{
		"article_id": id, "suggestion_title": "Fix the refund window", "reason": "It is 10 days now.",
		"edits": []map[string]any{{"find": "5 days", "replace": "10 days"}},
	}
	maps.Copy(args, more)

	return args
}

func TestOperatorWorkRules(t *testing.T) {
	t.Parallel()

	t.Run("names the arguments a suggestion is missing", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days.")

		res := w.call("dimmi_propose_article_edit", map[string]any{"article_id": id, "reason": "Stale."})

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"edits" is required and missing. The passages to replace, such as`)
		assert.Contains(t, res.Text, `"suggestion_title" is required and missing`)
		assert.Equal(t, 0, w.pending())
	})

	t.Run("says a title is not an article id", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)

		res := w.call("dimmi_propose_article_edit", edit("Refunds", nil))

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `there is no article "Refunds"`)
		assert.Contains(t, res.Text, "never the title")
	})

	t.Run("quotes the closest passage when the one sent is not in the article", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days to reach your card.")

		res := w.call("dimmi_propose_article_edit", edit(id, map[string]any{
			"edits": []map[string]any{{"find": "Shipping takes 3 weeks by sea freight.", "replace": "x"}},
		}))

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "Reason:")
		assert.Contains(t, res.Text, "Tip: When a passage is not found")
	})

	t.Run("marks a dry run's suggestion as a preview and keeps nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days.")
		routine := w.routine()

		made := w.ok("dimmi_propose_article_edit", edit(id, map[string]any{"routine_id": routine, "dry_run": true}))
		saved := w.ok("dimmi_save_routine_state", map[string]any{
			"routine_id": routine, "state": map[string]any{"next_offset": 15}, "dry_run": true,
		})
		quiet := w.ok("dimmi_report_nothing_to_change", map[string]any{
			"routine_id": routine, "reason": "Nothing.", "dry_run": true,
		})

		assert.Equal(t, true, inner(made, "suggestion")["preview"])
		assert.Equal(t, false, saved["saved"])
		assert.Equal(t, false, quiet["recorded"])
		assert.Equal(t, testkit.JSON{}, w.ok("dimmi_get_routine_state", map[string]any{"routine_id": routine})["state"])
		assert.Empty(t, w.ok("dimmi_list_recent_suggestions", nil)["items"])
		w.d.Owner.GetJSON("/api/v1/operator/routines/"+routine+"/runs").AssertOK().AssertJSONPath("data.#", float64(0))
	})

	t.Run("refuses a dry run that names no routine", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days.")

		res := w.call("dimmi_propose_article_edit", edit(id, map[string]any{"dry_run": true}))

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "- routine_id:")
	})

	t.Run("refuses a kind of change the workspace switched off", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := article(t, w.d, "Refunds take 5 days.")
		w.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{
			"policies": testkit.JSON{"articles": "off"},
		}).AssertOK()

		res := w.call("dimmi_propose_article_edit", edit(id, nil))

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "switched off in this workspace")
		assert.Equal(t, 0, w.pending())
	})

	t.Run("never applies a suggestion on its own, even when the workspace allows it", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		id := conversation(t, w.d, "Where is my invoice?")
		w.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{
			"policies": testkit.JSON{"conversations": "auto"},
		}).AssertOK()

		made := w.ok("dimmi_propose_changes", map[string]any{
			"suggestion_title": "Route to Billing", "reason": "It is about an invoice.",
			"operations": []map[string]any{{
				"kind": "conversation.tag", "conversation_number": 1, "tags": []string{"billing"},
			}},
		})

		assert.Equal(t, "pending", inner(made, "suggestion")["status"])
		w.d.Owner.GetJSON("/api/v1/conversations/"+id).AssertOK().AssertJSONPath("data.tags.#", float64(0))
	})

	t.Run("keeps a routine's state from a teammate who may not manage the AI agent", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		routine := w.routine()
		_, member := w.d.Join(t, enums.RoleMember)
		_, token := w.d.TC.MintToken(member, "*")

		res, err := agent(t, w.d, token).CallTool(t.Context(), "dimmi_save_routine_state", map[string]any{
			"routine_id": routine, "state": map[string]any{"next_offset": 15},
		})

		assert.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "Do not retry")
	})
}

func TestOperatorWorkKinds(t *testing.T) {
	t.Parallel()

	t.Run("suggests a new article as a draft for review", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)

		made := w.ok("dimmi_propose_new_article", map[string]any{
			"article_title": "Set up single sign-on", "article_body": "## Before you start\n\nAsk your admin.",
			"suggestion_title": "Add a guide to SSO setup", "reason": "Customers ask and find nothing.",
		})

		assert.Equal(t, true, made["made"])
		assert.Equal(t, "article.create_draft", w.d.Owner.GetJSON("/api/v1/operator/changesets").AssertOK().
			String("data.0.operations.0.kind"))
	})

	t.Run("suggests an edit to a procedure and names what a bad one lacks", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)
		procedure := w.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Refunds"}).AssertCreated().
			String("data.id")
		args := map[string]any{
			"procedure_id": procedure, "suggestion_title": "Ask for the order number", "reason": "Refunds stall.",
		}

		missing := w.call("dimmi_propose_procedure_edit", args)
		args["procedure_definition"] = map[string]any{
			"description": "Refund requests.", "handover": []string{},
			"steps": []map[string]any{{"id": "s1", "kind": "instruction", "text": "Ask for the order number."}},
		}
		made := w.ok("dimmi_propose_procedure_edit", args)

		assert.True(t, missing.IsError)
		assert.Contains(t, missing.Text, `"procedure_definition" is required and missing`)
		assert.Equal(t, true, made["made"])
	})

	t.Run("lists the skills and reads one, naming the skills when the name is wrong", func(t *testing.T) {
		t.Parallel()
		w := newWorker(t)

		listed := w.call("dimmi_list_skills", nil)
		read := w.ok("dimmi_get_skill_by_name", map[string]any{"skill_name": "audit_articles"})
		wrong := w.call("dimmi_get_skill_by_name", map[string]any{"skill_name": "audit"})

		assert.Contains(t, listed.Text, "audit_articles")
		assert.Contains(t, read["steps"], "How to")
		assert.True(t, wrong.IsError)
		assert.Contains(t, wrong.Text, "The skills are find_knowledge_gaps, audit_articles")
	})
}
