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
		"conversations":             "handed to the placeholder, messages and files wiped",
		"contact_identities":        "freed on request, deleted with the contact",
		"conversation_participants": "deleted with the contact",
		"contact_tag":               "deleted with the contact",
		"widget_sessions":           "deleted with the contact",
		"contact_erasures":          "the request itself, holding no personal data once done",
		"contact_exports":           "zips deleted by the wipe, rows deleted with the contact",
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
