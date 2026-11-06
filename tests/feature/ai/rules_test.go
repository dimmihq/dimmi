package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
TestAiWorkspaceRules makes the workspace's handoff rules hold even when
the agent forgets them.
*/
func TestAiWorkspaceRules(t *testing.T) {
	t.Parallel()

	t.Run("hands off in triage-only mode when the agent did not", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator(`UPDATE ai_agents SET handoff = '{"mode":"triage_only"}' WHERE workspace_id = $1`,
			h.d.TC.Workspace.Workspace.ID)
		h.llm.answer.Script(reply("Thanks for the details."))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "workspace_rule", res.HandoffReason)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "enforced_handoff")
		assert.Equal(t, "handed_off", s.conv.AiState)
		body := str(s.agentReplies()[0].Body)
		assert.Contains(t, body, "Thanks for the details.")
		assert.Contains(t, body, "bringing in a teammate")
		require.Len(t, s.notes(), 1)

		sent := h.llm.answer.Sent()[0]
		assert.NotContains(t, toolNames(sent), "search_knowledge")
		assert.Contains(t, sent.System[0].Text, "Mode: triage only")
	})

	t.Run("does not hand off twice when the agent followed the triage-only rule", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator(`UPDATE ai_agents SET handoff = '{"mode":"triage_only"}' WHERE workspace_id = $1`,
			h.d.TC.Workspace.Workspace.ID)
		h.llm.answer.Script(handoffCall("other"), reply("A teammate will take it from here."))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "other", res.HandoffReason)

		s := h.load(conv, res.RunID)
		assert.NotContains(t, s.run.Flags, "enforced_handoff")
		assert.Equal(t, "A teammate will take it from here.", str(s.agentReplies()[0].Body))
		assert.Len(t, s.notes(), 1)
	})

	t.Run("hands off when an always-handoff tag lands on the conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		legal := h.tag("legal")
		h.operator(`UPDATE ai_agents SET handoff = jsonb_build_object('always_tags', jsonb_build_array($2::text))
			WHERE workspace_id = $1`, h.d.TC.Workspace.Workspace.ID, legal.ID.String())
		h.llm.answer.Script(tools(triageCall(map[string]any{"tags": []string{"legal"}})),
			reply("I understand this is about a contract."))

		conv, trigger := h.conversation("Our lawyer has questions about the contract")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "workspace_rule", res.HandoffReason)
		assert.Contains(t, h.llm.answer.Sent()[0].System[0].Text,
			"Always hand off when the conversation is about: legal.")
	})

	t.Run("does not hand off twice when the agent handed off a tagged conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		legal := h.tag("legal")
		h.operator(`UPDATE ai_agents SET handoff = jsonb_build_object('always_tags', jsonb_build_array($2::text))
			WHERE workspace_id = $1`, h.d.TC.Workspace.Workspace.ID, legal.ID.String())
		h.llm.answer.Script(tools(triageCall(map[string]any{"tags": []string{"legal"}})), handoffCall("legal"),
			reply("I'm bringing in a teammate."))

		conv, trigger := h.conversation("Our lawyer has questions about the contract")
		res := h.run(conv, trigger)
		assert.Equal(t, "legal", res.HandoffReason)
		s := h.load(conv, res.RunID)
		assert.NotContains(t, s.run.Flags, "enforced_handoff")
		assert.Equal(t, "I'm bringing in a teammate.", str(s.agentReplies()[0].Body))
	})

	t.Run("tells the model there are no extra rules by default", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hi!"))

		conv, trigger := h.conversation("hello")
		h.run(conv, trigger)
		assert.Contains(t, h.llm.answer.Sent()[0].System[0].Text, "No extra rules.")
	})
}
