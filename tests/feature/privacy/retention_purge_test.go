package privacy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/retention"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
stored writes a file to the default disk and returns the disk's name.
*/
func stored(t *testing.T, tc *tests.TestCase, path string) string {
	t.Helper()

	disk, err := tc.App.Storage.Default()
	require.NoError(t, err)
	require.NoError(t, disk.Put(t.Context(), path, []byte("kept"), filesystem.PutOptions{}))

	return tc.App.Application.Config.Filesystems.Default
}

/*
exists reports whether the default disk still holds the file.
*/
func exists(t *testing.T, tc *tests.TestCase, path string) bool {
	t.Helper()

	disk, err := tc.App.Storage.Default()
	require.NoError(t, err)

	found, err := disk.Exists(t.Context(), path)
	require.NoError(t, err)

	return found
}

/*
conversationID decodes the id at the end of a conversation's path.
*/
func conversationID(t *testing.T, tc *tests.TestCase, path string) ids.UUID {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixConversation), path[len("/api/v1/conversations/"):])
	require.NoError(t, err)

	return id
}

/*
mailed gives the conversation an attachment, a received email with its
stored source and a sent email, returning the paths of the two files.
*/
func mailed(t *testing.T, tc *tests.TestCase, conversation ids.UUID, name string) (string, string) {
	t.Helper()
	ctx, db, ws := t.Context(), tc.Operator(), tc.Workspace.Workspace.ID

	var message ids.UUID
	require.NoError(t, db.QueryRow(ctx,
		`SELECT id FROM messages WHERE conversation_id = $1 ORDER BY created_at LIMIT 1`, conversation).Scan(&message))

	file, raw := "workspaces/"+ws.String()+"/attachments/"+name+".txt", "inbound-emails/"+ws.String()+"/"+name+".eml"
	disk := stored(t, tc, file)
	stored(t, tc, raw)

	seeds := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO attachments
			(workspace_id, message_id, uploaded_by_type, disk, path, filename, content_type, size_bytes)
			VALUES ($1, $2, 'contact', $3, $4, 'note.txt', 'text/plain', 4)`, []any{ws, message, disk, file}},
		{
			`INSERT INTO inbound_emails (workspace_id, source, envelope_to, from_address, subject, size_bytes, raw_disk,
			raw_path, status, conversation_id, message_id, received_at, processed_at)
			VALUES ($1, 'test', '{}', 'ann@example.com', 'My order', 4, $2, $3, 'processed', $4, $5, $6, $6)`,
			[]any{ws, disk, raw, conversation, message, tc.Clock.Now()},
		},
		{
			`INSERT INTO outbound_emails (workspace_id, conversation_id, kind, from_address, to_addresses, subject,
			message_id_header, transport)
			VALUES ($1, $2, 'reply', 'help@acme.example', '{ann@example.com}', 'Re: My order', $3, 'log')`,
			[]any{ws, conversation, "<" + name + "@acme.example>"},
		},
	}
	for _, seed := range seeds {
		_, err := db.Exec(ctx, seed.query, seed.args...)
		require.NoError(t, err)
	}

	return file, raw
}

/*
purgeRetention runs the retention purge for the test case's workspace.
*/
func purgeRetention(t *testing.T, tc *tests.TestCase) retention.Purged {
	t.Helper()

	ws, err := models.New(tc.Operator()).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
	require.NoError(t, err)

	purged, err := tc.App.Actions.Inbox.PurgeRetention.Workspace(t.Context(), ws)
	require.NoError(t, err)

	return purged
}

func TestRetentionPurge(t *testing.T) {
	t.Parallel()

	t.Run("empties the trash after its days, with the files and mail of what it held", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		old, recent, live := closed(t, d, "ann"), closed(t, d, "bob"), closed(t, d, "cy")
		file, raw := mailed(t, d.TC, conversationID(t, d.TC, old), "old")
		keptFile, keptRaw := mailed(t, d.TC, conversationID(t, d.TC, recent), "recent")
		now := d.TC.Clock.Now()
		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE conversations SET deleted_at = $1 WHERE id = $2`,
			now.Add(-31*24*time.Hour), conversationID(t, d.TC, old))
		require.NoError(t, err)
		_, err = d.TC.Operator().Exec(t.Context(), `UPDATE conversations SET deleted_at = $1 WHERE id = $2`,
			now.Add(-29*24*time.Hour), conversationID(t, d.TC, recent))
		require.NoError(t, err)

		purged := purgeRetention(t, d.TC)

		assert.Equal(t, retention.Purged{Trashed: 1}, purged)
		d.TC.Table("conversations").AssertCount(2)
		d.TC.Table("conversations").AssertCount(0, testkit.Row{"id": conversationID(t, d.TC, old)})
		d.TC.Table("attachments").AssertCount(1)
		d.TC.Table("inbound_emails").AssertCount(1, testkit.Row{"raw_path": keptRaw})
		d.TC.Table("inbound_emails").AssertCount(1)
		d.TC.Table("outbound_emails").AssertCount(1)
		assert.False(t, exists(t, d.TC, file))
		assert.False(t, exists(t, d.TC, raw))
		assert.True(t, exists(t, d.TC, keptFile))
		assert.True(t, exists(t, d.TC, keptRaw))
		d.Owner.GetJSON(live).AssertOK()

		assert.Equal(t, retention.Purged{}, purgeRetention(t, d.TC))
	})

	t.Run("deletes spam after its days and leaves what was taken out of spam", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		spam, cleared, live := closed(t, d, "ann"), closed(t, d, "bob"), closed(t, d, "cy")
		d.Owner.PatchJSON(spam, testkit.JSON{"spam": true}).AssertOK()
		d.Owner.PatchJSON(cleared, testkit.JSON{"spam": true}).AssertOK()
		d.Owner.PatchJSON(cleared, testkit.JSON{"spam": false}).AssertOK()

		assert.Equal(t, retention.Purged{}, purgeRetention(t, d.TC))

		d.TC.Clock.Travel(31 * 24 * time.Hour)

		assert.Equal(t, retention.Purged{Spam: 1}, purgeRetention(t, d.TC))
		d.TC.Table("conversations").AssertCount(2)
		d.TC.Table("conversations").AssertCount(0, testkit.Row{"id": conversationID(t, d.TC, spam)})
		d.TC.Table("conversations").AssertHas(testkit.Row{"id": conversationID(t, d.TC, live)})
	})

	t.Run("honors the days the workspace chose, never the day they are set", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		old := closed(t, d, "ann")
		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE conversations SET deleted_at = $1 WHERE id = $2`,
			d.TC.Clock.Now().Add(-10*24*time.Hour), conversationID(t, d.TC, old))
		require.NoError(t, err)

		assert.Equal(t, retention.Purged{}, purgeRetention(t, d.TC))

		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"trash_days": 7}}).AssertOK()
		assert.Equal(t, retention.Purged{}, purgeRetention(t, d.TC))

		d.TC.Clock.Travel(25 * time.Hour)
		assert.Equal(t, retention.Purged{Trashed: 1}, purgeRetention(t, d.TC))
	})

	t.Run("removes the source of old email, keeping the record of mail that became a message", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conversation := conversationID(t, d.TC, closed(t, d, "ann"))
		_, raw := mailed(t, d.TC, conversation, "read")
		ws, db := d.TC.Workspace.Workspace.ID, d.TC.Operator()
		ignored, shared := "inbound-emails/"+ws.String()+"/ignored.eml", "inbound-emails/"+ws.String()+"/shared.eml"
		disk := stored(t, d.TC, ignored)
		stored(t, d.TC, shared)
		long, short := d.TC.Clock.Now().Add(-31*24*time.Hour), d.TC.Clock.Now().Add(-24*time.Hour)
		_, err := db.Exec(t.Context(), `UPDATE inbound_emails SET processed_at = $1, received_at = $1`, long)
		require.NoError(t, err)
		_, err = db.Exec(t.Context(), `INSERT INTO inbound_emails
			(workspace_id, source, envelope_to, from_address, size_bytes, raw_disk, raw_path, status, received_at,
			processed_at)
			VALUES ($1, 'test', '{}', 'bulk@example.com', 4, $2, $3, 'ignored', $5, $5),
			($1, 'test', '{}', 'twice@example.com', 4, $2, $4, 'ignored', $5, $5),
			($1, 'test', '{}', 'twice@example.com', 4, $2, $4, 'ignored', $6, $6)`,
			ws, disk, ignored, shared, long, short)
		require.NoError(t, err)

		purged := purgeRetention(t, d.TC)

		assert.Equal(t, retention.Purged{RawEmails: 3}, purged)
		d.TC.Table("inbound_emails").AssertCount(2)
		d.TC.Table("inbound_emails").AssertCount(1, testkit.Row{"conversation_id": conversation, "raw_path": nil})
		d.TC.Table("inbound_emails").AssertCount(1, testkit.Row{"raw_path": shared})
		assert.False(t, exists(t, d.TC, raw))
		assert.False(t, exists(t, d.TC, ignored))
		assert.True(t, exists(t, d.TC, shared), "a file another record still points at stays")
		d.TC.Table("messages").AssertHas(testkit.Row{"conversation_id": conversation})
	})

	t.Run("leaves a conversation the retention guard holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = holdAll{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		closed(t, d, "ann")
		_, err = tc.Operator().Exec(t.Context(), `UPDATE conversations SET deleted_at = $1`,
			tc.Clock.Now().Add(-40*24*time.Hour))
		require.NoError(t, err)

		assert.Equal(t, retention.Purged{}, purgeRetention(t, tc))
		tc.Table("conversations").AssertCount(1)
	})
}
