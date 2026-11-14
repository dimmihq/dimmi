package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

func TestAiTokenSavers(t *testing.T) {
	t.Parallel()

	t.Run("searches with the customer's message before the first call, compactly", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(reply("It costs per seat [1]."))

		conv, trigger := h.conversation("What is the pro plan price per seat?")
		res := h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 1, "one model call answers")
		msgs := sent[0].Messages
		require.GreaterOrEqual(t, len(msgs), 3)
		call := msgs[len(msgs)-2]
		require.Len(t, call.ToolCalls, 1)
		assert.Equal(t, "search_knowledge", call.ToolCalls[0].Name)
		result := msgs[len(msgs)-1].Parts[0].Text
		assert.NotContains(t, result, `"results"`, "results are written compactly")
		assert.Contains(t, result, "[1] ")
		s := h.load(conv, res.RunID)
		var searches int
		for _, st := range s.steps {
			if st.Kind == "tool_call" && str(st.Model) == "search_knowledge" {
				searches++
			}
		}
		assert.Equal(t, 1, searches)
	})

	t.Run("does not search a thank-you", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(reply("You're welcome!"))

		conv, trigger := h.conversation("Thanks so much!")
		h.run(conv, trigger)

		msgs := h.llm.answer.Sent()[0].Messages
		assert.Empty(t, msgs[len(msgs)-1].ToolCalls)
		assert.NotEqual(t, "tool", string(msgs[len(msgs)-1].Role))
	})

	t.Run("points to a passage already shown instead of sending it again", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(search("pro plan price per seat"), reply("Per seat [1]."))

		conv, trigger := h.conversation("What is the pro plan price per seat?")
		h.run(conv, trigger)

		msgs := h.llm.answer.Sent()[1].Messages
		assert.Contains(t, msgs[len(msgs)-1].Parts[0].Text, "(shown above)")
	})
}

func TestAiSearchResultsSetting(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	h.operator("UPDATE ai_agents SET search_results = 1 WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
	h.llm.answer.Script(search("pro plan price per seat"), reply("Per seat [1]."))

	conv, trigger := h.conversation("What is the pro plan price per seat?")
	h.run(conv, trigger)

	msgs := h.llm.answer.Sent()[1].Messages
	result := msgs[len(msgs)-1].Parts[0].Text
	assert.Contains(t, result, `"id":1`)
	assert.NotContains(t, result, `"id":2`, "one passage per search")
}

func TestLLMManagerFlushDropsCachedDrivers(t *testing.T) {
	t.Parallel()
	h := newHarness(t, nil)
	builds := 0
	h.d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		builds++
		return h.llm, nil
	})
	ctx := tenancy.With(t.Context(), h.d.TC.Workspace.Workspace.ID.String())
	var key models.AiProviderKey
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		agent, err := models.New(h.d.TC.App.DB).FindAgent(ctx, h.d.TC.Workspace.Workspace.ID)
		if err != nil {
			return err
		}
		key, err = models.New(h.d.TC.App.DB).FindProviderKey(ctx, models.FindProviderKeyParams{
			WorkspaceID: agent.WorkspaceID, ID: *agent.AnswerKeyID,
		})
		return err
	})

	for range 2 {
		_, err := h.d.TC.App.LLM.ForKey(ctx, key)
		require.NoError(t, err)
	}
	h.d.TC.App.LLM.Flush()
	_, err := h.d.TC.App.LLM.ForKey(ctx, key)
	require.NoError(t, err)

	assert.Equal(t, 2, builds, "a driver is built once, and again after a flush")
}
