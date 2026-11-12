package privacy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/tests"
)

/*
erasedBy says how an erasure reaches every table that points at a
contact. A table added with a contact_id column must be listed here,
and either cascade with the contact, be handed to the placeholder, or
have an eraser registered for it.
*/
func erasedBy() map[string]string {
	return map[string]string{
		"conversations":               "handed to the placeholder, messages and files wiped",
		"contact_identities":          "freed on request, deleted with the contact",
		"conversation_participants":   "deleted with the contact",
		"contact_tag":                 "deleted with the contact",
		"contact_event_summaries":     "deleted with the contact, details included",
		"widget_sessions":             "deleted with the contact",
		"contact_erasures":            "the request itself, holding no personal data once done",
		"contact_exports":             "zips deleted by the wipe, rows deleted with the contact",
		"ai_action_calls":             "inputs, results and notes scrubbed by an eraser, rows kept",
		"workflow_runs":               "variables, trigger data and call answers scrubbed by an eraser, rows kept",
		"webhook_deliveries":          "stored payloads deleted by an eraser",
		"article_ratings":             "notes scrubbed by an eraser, answers kept under a hash that names nobody",
		"conversation_ratings":        "deleted with the contact, comments included",
		"changelog_subscribers":       "deleted by an eraser, those under the customer's address included",
		"feedback_votes":              "deleted with the contact",
		"feedback_post_conversations": "the contact cleared when the contact is deleted, the link kept",
		"broadcast_recipients":        "address and delivery details scrubbed by an eraser, rows kept",
		"broadcast_opt_outs":          "deleted with the contact",
		"contact_topic_subscriptions": "deleted with the contact",
		"series_enrollments":          "deleted with the contact",
		"tickets":                     "the contact cleared when the contact is deleted, the conversation handed over",
	}
}

func TestErasureRegistry(t *testing.T) {
	t.Parallel()

	t.Run("accounts for every table that points at a contact", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		rows, err := tc.Operator().Query(t.Context(), `
			SELECT table_name FROM information_schema.columns
			WHERE table_schema = current_schema() AND column_name = 'contact_id'
			ORDER BY table_name`)
		require.NoError(t, err)

		defer rows.Close()

		known := erasedBy()

		for rows.Next() {
			var table string
			require.NoError(t, rows.Scan(&table))
			assert.Contains(t, known, table, "%s has a contact_id: erase it or register an eraser", table)
		}

		require.NoError(t, rows.Err())
	})
}
