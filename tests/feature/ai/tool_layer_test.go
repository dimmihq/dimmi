package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
loadCall and callTool are scripted calls of the layer's own tools.
*/
func loadCall(names ...string) contracts.ToolCall {
	data, _ := json.Marshal(map[string]any{"names": names})
	return call("load_tools", string(data))
}

func callTool(name string, args map[string]any) contracts.ToolCall {
	data, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	return call("call_tool", string(data))
}

/*
layerRecord reads what the layer kept on a run.
*/
func (h *harness) layerRecord(s state) toolset.Record {
	h.t.Helper()
	var r toolset.Record
	require.NotNil(h.t, s.run.ToolLayer, "run keeps the layer's record")
	require.NoError(h.t, json.Unmarshal(s.run.ToolLayer, &r))
	return r
}

/*
TestAiToolLayer offers the agent a fixed short list of tools and the
rest through load_tools and call_tool.
*/
func TestAiToolLayer(t *testing.T) {
	t.Parallel()

	t.Run("offers the same fixed tools on every call and runs a loaded tool", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withToolLayer()
		h.llm.answer.Script(
			tools(loadCall("triage_conversation")),
			tools(callTool("triage_conversation", map[string]any{"title": "Double charge refund"})),
			reply("Let me look into that."),
		)

		conv, trigger := h.conversation("I was charged twice")
		h.operator("UPDATE conversations SET subject = NULL WHERE id = $1", conv)
		res := h.run(conv, trigger)
		require.Equal(t, "greeted", res.Outcome)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 3)
		want := []string{"call_tool", "handoff", "load_tools", "search_knowledge"}
		for _, req := range sent {
			assert.Equal(t, want, toolNames(req))
		}
		assert.Contains(t, sent[0].System[0].Text,
			"- inbox: tag, assign and prioritize this conversation (triage_conversation)")
		assert.Contains(t, sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text, `"input_schema"`)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "Double charge refund", str(s.conv.Subject))
		r := h.layerRecord(s)
		assert.Equal(t, []string{"triage_conversation"}, r.Loaded)
		assert.Equal(t, 1, r.Stats.Loads)
		assert.Equal(t, 1, r.Stats.Calls)
		assert.Equal(t, []string{"triage_conversation"}, r.Stats.Used)
		assert.Positive(t, r.FixedTokens)

		var names []string
		for _, st := range s.steps {
			if st.Kind == "tool_call" {
				names = append(names, str(st.Model))
			}
		}
		assert.Equal(t, []string{"load_tools", "triage_conversation"}, names)
	})

	t.Run("refuses a tool that is not loaded and arguments that do not fit, in short words", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withToolLayer()
		h.llm.answer.Script(
			tools(callTool("triage_conversation", map[string]any{"title": "x"})),
			tools(loadCall("triage_conversation", "refund_card")),
			tools(callTool("triage_conversation", map[string]any{"priority": "very"})),
			reply("Let me check."),
		)

		conv, trigger := h.conversation("I was charged twice")
		res := h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		last := func(i int) contracts.ChatMessage { return sent[i].Messages[len(sent[i].Messages)-1] }
		assert.True(t, last(1).ToolError)
		assert.Contains(t, last(1).Parts[0].Text, "load_tools")
		assert.Contains(t, last(2).Parts[0].Text, `"not_found":["refund_card"]`)
		assert.True(t, last(3).ToolError)
		assert.Contains(t, last(3).Parts[0].Text, "invalid arguments for triage_conversation")
		assert.Less(t, len(last(3).Parts[0].Text), 320)

		r := h.layerRecord(h.load(conv, res.RunID))
		assert.Equal(t, 2, r.Stats.ArgErrors)
		assert.Equal(t, []string{"refund_card"}, r.Stats.NotFound)
		assert.Zero(t, r.Stats.Calls)
	})

	t.Run("names the tools to reload and likely tools in the newest message only", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withToolLayer()
		h.llm.answer.Script(
			tools(loadCall("triage_conversation")),
			reply("Let me look into that."),
			reply("Done."),
		)

		conv, first := h.conversation("I was charged twice")
		h.run(conv, first)
		second := h.add(conv, "contact", "message", "Please tag this and make it a priority")
		res := h.run(conv, second)

		sent := h.llm.answer.Sent()
		req := sent[len(sent)-1]
		newest := req.Messages[len(req.Messages)-1]
		note := newest.Parts[len(newest.Parts)-1].Text
		assert.Contains(t, note, "load again before use: triage_conversation.")
		assert.Contains(t, note, "Likely tools for this message: triage_conversation")
		assert.True(t, req.Messages[len(req.Messages)-2].Cacheable, "stable history ends before the newest message")
		for _, m := range req.Messages[:len(req.Messages)-1] {
			for _, p := range m.Parts {
				assert.NotContains(t, p.Text, "[tools]")
			}
		}

		r := h.layerRecord(h.load(conv, res.RunID))
		assert.Equal(t, []string{"triage_conversation"}, r.Earlier)
		assert.Contains(t, r.Suggested, "triage_conversation")
	})

	t.Run("stays off when the agent turns it off", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hello."))
		conv, trigger := h.conversation("I was charged twice")
		res := h.run(conv, trigger)

		assert.Contains(t, toolNames(h.llm.answer.Sent()[0]), "triage_conversation")
		assert.NotContains(t, toolNames(h.llm.answer.Sent()[0]), "load_tools")
		assert.Nil(t, h.load(conv, res.RunID).run.ToolLayer)
	})
}
