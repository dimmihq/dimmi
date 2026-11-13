package operator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
loadTools and callTool are scripted calls of the layer's own tools.
*/
func loadTools(names ...string) contracts.ToolCall {
	return call("load_tools", map[string]any{"names": names})
}

func callTool(name string, args map[string]any) contracts.ToolCall {
	return call("call_tool", map[string]any{"name": name, "arguments": args})
}

/*
offered lists the names of a request's tools.
*/
func offered(req contracts.ChatRequest) []string {
	out := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		out = append(out, t.Name)
	}
	return out
}

/*
lastResult is the text of the newest message of a request.
*/
func lastResult(req contracts.ChatRequest) contracts.ChatMessage {
	return req.Messages[len(req.Messages)-1]
}

/*
layerRecord reads what the layer kept on the workspace's latest
Operator run.
*/
func (k *desk) layerRecord() toolset.Record {
	k.t.Helper()
	var raw json.RawMessage
	k.d.TC.InWorkspace(func(ctx context.Context) error {
		return k.d.TC.App.DB.QueryRow(ctx, "SELECT tool_layer FROM ai_runs WHERE workspace_id = $1 "+
			"AND tool_layer IS NOT NULL ORDER BY created_at DESC LIMIT 1", k.d.TC.Workspace.Workspace.ID).Scan(&raw)
	})
	var r toolset.Record
	require.NoError(k.t, json.Unmarshal(raw, &r))
	return r
}

func TestOperatorToolLayer(t *testing.T) {
	t.Parallel()

	t.Run("offers a fixed short list and runs its own tools through the layer", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		thread := k.thread(k.conversation("Where is my order?"))
		k.llm.Script(
			tools(loadTools("list_tags")),
			tools(callTool("list_tags", map[string]any{})),
			reply("Done."),
		)

		res := k.ask(thread, "Which tags do we have?").AssertOK()

		want := []string{"call_tool", "get_conversation", "load_tools", "search_conversations", "search_knowledge"}
		for _, req := range k.llm.Sent() {
			assert.Equal(t, want, offered(req))
			assert.True(t, req.System[0].Cacheable)
		}
		system := k.llm.Sent()[0].System[0].Text
		assert.Contains(t, system, "- conversations: list conversations, read customers, tags and teammates")
		assert.Contains(t, system, "- api_tags: read-only API: tags")
		assert.Equal(t, "true", res.String("data.message.steps.1.ok"), res.String("data.message.steps"))

		r := k.layerRecord()
		assert.Equal(t, []string{"list_tags"}, r.Loaded)
		assert.Equal(t, 1, r.Stats.Calls)
	})

	t.Run("reads the catalog as the teammate, with fields, and leaves writes out of reach", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		k.d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated()
		thread := k.thread("")
		k.llm.Script(
			tools(loadTools("dimmi_tags_list", "dimmi_tags_create")),
			tools(callTool("dimmi_tags_list", map[string]any{"fields": []string{"name"}})),
			reply("One tag."),
		)

		k.ask(thread, "List tags through the API").AssertOK()

		sent := k.llm.Sent()
		loaded := lastResult(sent[1]).Parts[0].Text
		assert.Contains(t, loaded, `"not_found":["dimmi_tags_create"]`)
		assert.Contains(t, loaded, `"fields"`)
		read := lastResult(sent[2])
		require.False(t, read.ToolError, read.Parts[0].Text)
		assert.Contains(t, read.Parts[0].Text, `"name":"billing"`)
		assert.NotContains(t, read.Parts[0].Text, `"created_at"`)
		assert.Equal(t, 1, k.count("SELECT count(*) FROM personal_access_tokens WHERE name = 'Operator catalog reads' "+
			"AND expires_at IS NOT NULL AND NOT abilities && ARRAY['tags:write','settings:write']"))
	})

	t.Run("keeps write policies: a switched-off write is not behind the layer", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		k.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{"policies": testkit.JSON{
			"conversations": "off", "notes": "off", "contacts": "off", "articles": "off", "procedures": "off",
			"actions": "off",
		}}).AssertOK()
		thread := k.thread(k.conversation("Charged twice."))
		k.llm.Script(tools(loadTools("propose_changes")), reply("I cannot change that."))

		k.ask(thread, "File this as billing").AssertOK()

		result := lastResult(k.llm.Sent()[1])
		assert.True(t, result.ToolError)
		assert.Contains(t, result.Parts[0].Text, "propose_changes")
		assert.Equal(t, []string{"propose_changes"}, k.layerRecord().Stats.NotFound)
	})

	t.Run("proposes changes through call_tool as a changeset that waits for review", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		thread := k.thread(k.conversation("Charged twice."))
		k.llm.Script(
			tools(loadTools("propose_changes")),
			tools(callTool("propose_changes", map[string]any{
				"title": "File as billing", "reason": "A double charge.",
				"operations": []map[string]any{{"kind": "conversation.tag", "tags": []string{"billing"}}},
			})),
			reply("Proposed."),
		)

		res := k.ask(thread, "File this").AssertOK()
		assert.Equal(t, "pending", res.String("data.changesets.0.status"))
	})

	t.Run("names the tools to reload in the next question and marks the history cacheable", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		thread := k.thread("")
		k.llm.Script(tools(loadTools("list_tags")), reply("Loaded."), reply("Again."))

		k.ask(thread, "Look at tags").AssertOK()
		k.ask(thread, "And now?").AssertOK()

		req := k.llm.Sent()[len(k.llm.Sent())-1]
		newest := lastResult(req)
		assert.Contains(t, newest.Parts[len(newest.Parts)-1].Text, "load again before use: list_tags.")
		assert.True(t, req.Messages[len(req.Messages)-2].Cacheable)
	})
}
