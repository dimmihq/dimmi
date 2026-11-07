package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
TestAiRunRobustness keeps the agent correct when models misbehave,
tools fail, messages arrive in bursts, teammates step in and jobs
retry.
*/
func TestAiRunRobustness(t *testing.T) {
	t.Parallel()

	t.Run("withdraws the tools at the loop cap and makes the model answer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("pro plan price"))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "unknown", res.Outcome)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 6)
		assert.Empty(t, sent[5].Tools)
		assert.NotEmpty(t, sent[4].Tools)
		s := h.load(conv, res.RunID)
		assert.Contains(t, str(s.agentReplies()[0].Body), "couldn't find that in our documentation")
	})

	t.Run("caps searches per turn", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("a"), search("b"), search("c"), search("d"), search("e"),
			reply("I could not find it."))

		conv, trigger := h.conversation("How much is Pro?")
		h.run(conv, trigger)

		last := h.llm.answer.Sent()[5]
		assert.Equal(t, "search limit reached for this turn; answer with what you have or say it isn't covered",
			last.Messages[len(last.Messages)-1].Parts[0].Text)
	})

	t.Run("feeds a tool error back to the model", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(tools(call("search_knowledge", `{"q":"wrong"}`), call("delete_account", `{}`)),
			search("pro plan price"), reply("Pro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)

		second := h.llm.answer.Sent()[1].Messages
		bad, unknown := second[len(second)-2], second[len(second)-1]
		assert.True(t, bad.ToolError)
		assert.Equal(t, "error: invalid arguments", bad.Parts[0].Text)
		assert.True(t, unknown.ToolError)
		assert.Equal(t, "error: unknown tool delete_account", unknown.Parts[0].Text)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "error", s.steps[1].Status)
	})

	t.Run("never shows the model internal notes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hi!"))

		conv, _ := h.conversation("Hi there")
		h.add(conv, "member", "note", "NOTE-7731: customer owes us money, be careful")
		h.add(conv, "member", "message", "Hello! How can I help?")
		trigger := h.add(conv, "contact", "message", "How much is Pro per seat?")
		h.run(conv, trigger)

		messages := h.llm.answer.Sent()[0].Messages
		require.Len(t, messages, 1)
		text := messages[0].Parts[0].Text
		assert.NotContains(t, text, "NOTE-7731")
		assert.Contains(t, text, "<teammate_message>Hello! How can I help?</teammate_message>")
		assert.Contains(t, text, "<customer_message>Hi there</customer_message>")
	})

	t.Run("discards the answer when a teammate replies while the model writes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Pro costs $12 per seat per month."))

		conv, trigger := h.conversation("How much is Pro?")
		h.llm.hook = func() { h.add(conv, "member", "message", "Pro is $12 per seat, Ana.") }

		res := h.run(conv, trigger)
		assert.Equal(t, "superseded", res.Outcome)
		assert.Equal(t, "human_replied", res.SkipReason)

		s := h.load(conv, res.RunID)
		assert.Empty(t, s.agentReplies())
		assert.Equal(t, int32(0), s.conv.AiReplyCount)
	})

	t.Run("answers a burst of three messages once", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("pro plan price"), reply("Pro costs $12 per seat per month [1]."))

		conv, first := h.conversation("hi")
		second := h.add(conv, "contact", "message", "I have a question")
		third := h.add(conv, "contact", "message", "how much is pro per seat?")

		assert.Equal(t, "debounced", h.run(conv, first).SkipReason)
		assert.Equal(t, "debounced", h.run(conv, second).SkipReason)
		res := h.run(conv, third)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Len(t, s.runs, 1)
		assert.Len(t, s.agentReplies(), 1)
		assert.Contains(t, h.llm.answer.Sent()[0].Messages[0].Parts[0].Text,
			"<customer_message>hi\n\nI have a question\n\nhow much is pro per seat?</customer_message>")
	})

	t.Run("never answers twice when the job retries", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hi there!"))

		conv, trigger := h.conversation("hello")
		require.Equal(t, "greeted", h.run(conv, trigger).Outcome)

		again := h.run(conv, trigger)
		assert.Equal(t, "already_answered", again.SkipReason)

		s := h.load(conv, again.RunID)
		assert.Len(t, s.agentReplies(), 1)
		assert.Len(t, s.runs, 1)
		assert.Len(t, h.llm.answer.Sent(), 1)
	})

	t.Run("skips a trigger message that is gone", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, _ := h.conversation("How much is Pro?")

		missing := h.run(conv, ids.New())
		assert.Equal(t, "trigger_missing", missing.SkipReason)
		assert.Empty(t, h.load(conv, missing.RunID).runs)
	})

	t.Run("trims an answer cut off by the token limit to its last full sentence", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("pro plan price"),
			reply("Pro costs $12 per seat per month [1]. It adds unlimited proj"))
		h.llm.stop = contracts.StopReasonMaxTokens

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		body := str(h.load(conv, res.RunID).agentReplies()[0].Body)
		assert.Equal(t, "Pro costs $12 per seat per month.", body)
		assert.False(t, strings.Contains(body, "proj"))
	})

	t.Run("sends the agent's temperature and timeouts at low effort with no output cap", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET temperature = 0.2 WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		h.llm.answer.Script(reply("Hi!"))

		conv, trigger := h.conversation("hello")
		h.run(conv, trigger)

		req := h.llm.answer.Sent()[0]
		require.NotNil(t, req.Temperature)
		assert.InDelta(t, 0.2, *req.Temperature, 0.0001)
		assert.Zero(t, req.MaxOutputTokens)
		lastTurn := req.Messages[len(req.Messages)-1].Parts[0].Text
		today := h.d.TC.Clock.Now().UTC().Format("Monday 2 January 2006")
		assert.Contains(t, lastTurn, "<today>"+today+" (UTC)</today>")
		assert.Equal(t, contracts.EffortLow, req.Effort)
		assert.Positive(t, req.Timeout.Total)
	})
}
