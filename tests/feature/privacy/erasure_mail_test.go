package privacy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
suppressedNow returns which of the addresses the workspace may not mail.
*/
func suppressedNow(t *testing.T, tc *tests.TestCase, emails ...string) []string {
	t.Helper()

	rows, err := models.New(tc.Operator()).ListSuppressedEmails(t.Context(), models.ListSuppressedEmailsParams{
		WorkspaceID: tc.Workspace.Workspace.ID, Emails: emails, Now: tc.Clock.Now(),
	})
	require.NoError(t, err)

	return rows
}

/*
mailTraces gives ann mail outside her conversation: an ignored email
from her address with its stored source, a reply to bob she was copied
on, and suppressions because she unsubscribed and because her address
bounced, beside bob's own suppression. It returns the stored source's
path.
*/
func mailTraces(t *testing.T, tc *tests.TestCase) string {
	t.Helper()
	ctx, db, ws := t.Context(), tc.Operator(), tc.Workspace.Workspace.ID

	raw := "inbound-emails/" + ws.String() + "/ignored.eml"
	disk := stored(t, tc, raw)

	seeds := []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO inbound_emails (workspace_id, source, envelope_to, envelope_from, from_address, subject,
			size_bytes, raw_disk, raw_path, status, received_at)
			VALUES ($1, 'test', '{}', 'Ann@Example.com', 'ann@example.com', 'Out of office', 4, $2, $3,
			'ignored', now()),
			($1, 'test', '{}', 'bob@example.com', 'bob@example.com', 'Hello', 4, $2, NULL, 'ignored', now())`,
			[]any{ws, disk, raw},
		},
		{`INSERT INTO outbound_emails (workspace_id, kind, from_address, to_addresses, cc_addresses, subject,
			message_id_header, transport, recipient_statuses)
			VALUES ($1, 'reply', 'help@acme.example', '{bob@example.com}', '{Ann@example.com,cy@example.com}',
			'Re: Hello', '<bob@acme.example>', 'log',
			'{"ann@example.com": {"status": "delivered"}, "bob@example.com": {"status": "delivered"}}')`, []any{ws}},
		{`INSERT INTO email_suppressions (workspace_id, email, reason, scope, source, details)
			VALUES ($1, 'ann@example.com', 'unsubscribed', 'all', 'one_click', '{"ip": "203.0.113.9"}'),
			($1, 'ann@example.com', 'hard_bounce', 'changelog', 'dsn', NULL),
			($1, 'bob@example.com', 'manual', 'all', 'manual', NULL)`, []any{ws}},
	}
	for _, seed := range seeds {
		_, err := db.Exec(ctx, seed.query, seed.args...)
		require.NoError(t, err)
	}

	return raw
}

func TestErasureMail(t *testing.T) {
	t.Parallel()

	t.Run("deletes received mail, forgets who sent mail went to, and keeps an unsubscribe only as a hash",
		func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			path, contact := customer(t, d)
			file, raw := mailed(t, d.TC, conversationID(t, d.TC, path), "ann")
			ignored := mailTraces(t, d.TC)
			ws := d.TC.Workspace.Workspace.ID

			erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
			d.TC.Clock.Travel(8 * 24 * time.Hour)
			assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))

			d.TC.Table("inbound_emails").AssertCount(1)
			d.TC.Table("inbound_emails").AssertCount(1, testkit.Row{"from_address": "bob@example.com"})
			assert.False(t, exists(t, d.TC, file))
			assert.False(t, exists(t, d.TC, raw))
			assert.False(t, exists(t, d.TC, ignored))

			d.TC.Table("outbound_emails").AssertCount(2)
			d.TC.Table("outbound_emails").AssertCount(1, testkit.Row{
				"subject": "", "message_id_header": "<ann@acme.example>",
			})

			var (
				to, cc   []string
				statuses string
			)
			require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
				`SELECT to_addresses, cc_addresses, recipient_statuses::text FROM outbound_emails
				WHERE message_id_header = '<ann@acme.example>'`).Scan(&to, &cc, &statuses))
			assert.Empty(t, to)
			assert.Empty(t, cc)
			require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
				`SELECT to_addresses, cc_addresses, recipient_statuses::text FROM outbound_emails
				WHERE message_id_header = '<bob@acme.example>'`).Scan(&to, &cc, &statuses))
			assert.Equal(t, []string{"bob@example.com"}, to)
			assert.Equal(t, []string{"cy@example.com"}, cc)
			assert.NotContains(t, statuses, "ann@example.com")
			assert.Contains(t, statuses, "bob@example.com")

			d.TC.Table("email_suppressions").AssertCount(2)
			d.TC.Table("email_suppressions").AssertCount(1, testkit.Row{"email": "bob@example.com"})
			d.TC.Table("email_suppressions").AssertCount(1, testkit.Row{
				"email": nil, "reason": "unsubscribed", "scope": "all", "details": nil,
			})

			var leftovers int
			require.NoError(t, d.TC.Operator().QueryRow(t.Context(), `SELECT count(*) FROM email_suppressions
				WHERE workspace_id = $1 AND email_hash LIKE '%ann%'`, ws).Scan(&leftovers))
			assert.Zero(t, leftovers)

			assert.ElementsMatch(t, []string{"ann@example.com", "bob@example.com"},
				suppressedNow(t, d.TC, "ann@example.com", "bob@example.com", "cy@example.com"))
			assert.Equal(t, []string{"ANN@example.com"}, suppressedNow(t, d.TC, "ANN@example.com"))

			assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))
			d.TC.Table("email_suppressions").AssertCount(2)
		})

	t.Run("shows a suppression kept as a hash on a new customer with the same address", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		mailTraces(t, d.TC)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		d.TC.Clock.Travel(8 * 24 * time.Hour)
		assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))

		d = tests.Desk{TC: d.TC, Inbox: d.Inbox, Owner: d.TC.In(d.TC.Owner)}
		_, again := customer(t, d)
		d.Owner.GetJSON("/api/v1/email-suppressions?filter[contact]="+again).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.reason", "unsubscribed").
			AssertJSONPath("data.0.email", "ann@example.com")
	})
}
