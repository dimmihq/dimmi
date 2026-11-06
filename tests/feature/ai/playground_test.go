package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TestAiPlayground answers a teammate's test question with the agent's
own turn and writes nothing a customer sees.
*/
func TestAiPlayground(t *testing.T) {
	t.Parallel()

	t.Run("answers with citations and records a playground run", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("pro plan price"), reply("Pro costs $12 per seat per month [1]."))

		var playground contracts.AIPlayground = h.d.TC.App.AI
		ctx := tenancy.With(t.Context(), h.d.TC.Workspace.Workspace.ID.String())
		answer, err := playground.Ask(ctx, contracts.PlaygroundQuestion{
			WorkspaceID: h.d.TC.Workspace.Workspace.ID, MemberID: h.d.TC.Workspace.Owner.ID,
			Message: "How much is Pro per seat?",
		})
		require.NoError(t, err)
		assert.Equal(t, "answered", answer.Outcome)
		assert.Equal(t, "Pro costs $12 per seat per month.", answer.Answer)
		require.Len(t, answer.Citations, 1)
		assert.Equal(t, "https://help.lovelace.test/plans-and-pricing", answer.Citations[0].URL)
		require.NotEmpty(t, answer.Retrieved)
		assert.True(t, answer.Retrieved[0].Used)
		assert.Positive(t, answer.CostMicros)
		require.NotNil(t, answer.RunID)

		h.d.TC.InWorkspace(func(ctx context.Context) error {
			run, err := models.New(h.d.TC.App.DB).FindRun(ctx, models.FindRunParams{
				WorkspaceID: h.d.TC.Workspace.Workspace.ID, ID: *answer.RunID,
			})
			require.NoError(t, err)
			assert.Equal(t, "playground", run.Kind)
			assert.Nil(t, run.ConversationID)
			assert.Equal(t, "answered", str(run.Outcome))
			return nil
		})
	})
}
