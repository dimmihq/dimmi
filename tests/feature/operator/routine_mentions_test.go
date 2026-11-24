package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
asked is the routine's instructions as the model was given them: the
last thing a teammate "said" in the first request.
*/
func asked(t *testing.T, sent []contracts.ChatRequest) string {
	t.Helper()
	require.NotEmpty(t, sent)

	var b strings.Builder

	for _, m := range sent[0].Messages {
		for _, part := range m.Parts {
			b.WriteString(part.Text)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func TestRoutineMentions(t *testing.T) {
	t.Parallel()

	t.Run("spells out a tag and a teammate by the names they have now", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		tag := k.d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Refund risk"}).AssertCreated().String("data.id")
		owner := models.PublicID(models.PrefixMember, k.d.TC.Workspace.Owner.ID)
		routine := k.routine(testkit.JSON{
			"name": "Flag refunds", "cadence": "weekly", "cron_expression": "0 9 * * 1",
			"prompt": "Find conversations tagged [[tag:" + tag + "]] and tell [[teammate:" + owner + "]] about them.",
		})
		k.llm.Script(reply("Done."))

		k.runRoutine(routine, true)

		got := asked(t, k.llm.Sent())
		assert.Contains(t, got, `Find conversations tagged the tag "Refund risk" and tell `)
		assert.Contains(t, got, " ("+owner+") about them.")
		assert.NotContains(t, got, "[[tag:")
		assert.NotContains(t, got, "[[teammate:")
	})

	t.Run("says a mention is gone when what it pointed at no longer exists", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		tag := k.d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Soon gone"}).AssertCreated().String("data.id")
		k.d.Owner.Delete("/api/v1/tags/" + tag).AssertStatus(204)
		routine := k.routine(testkit.JSON{
			"name": "Flag", "cadence": "weekly", "cron_expression": "0 9 * * 1",
			"prompt": "List conversations tagged [[tag:" + tag + "]], and anything in [[unknown:xyz]].",
		})
		k.llm.Script(reply("Done."))

		k.runRoutine(routine, true)

		got := asked(t, k.llm.Sent())
		assert.Contains(t, got, "List conversations tagged something that no longer exists, and anything in "+
			"something that no longer exists.")
	})

	t.Run("leaves plain instructions exactly as written", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{
			"name": "Plain", "cadence": "weekly", "cron_expression": "0 9 * * 1",
			"prompt": "Tell the Billing team which customers asked for a refund this week.",
		})
		k.llm.Script(reply("Done."))

		k.runRoutine(routine, true)

		sent := asked(t, k.llm.Sent())

		assert.Contains(t, sent, "Tell the Billing team which customers asked for a refund this week.")
	})
}
