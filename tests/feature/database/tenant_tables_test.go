package database_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/tests"
)

/*
inboxTables lists the shared inbox's tenant tables.
*/
func inboxTables() []string {
	return []string{
		"inboxes", "channel_connections", "companies", "contacts", "contact_identities", "conversations",
		"conversation_participants", "messages", "attachments", "tags", "conversation_tag", "conversation_reads",
		"saved_replies",
	}
}

/*
seedInbox fills every inbox table of the workspace with one row, as the
tables' owner, and returns the workspace's id.
*/
func seedInbox(t *testing.T, db *database.DB) string {
	t.Helper()

	workspace := seedWorkspace(t, db)
	user := seedUser(t, db, "agent")
	var id string
	err := db.QueryRow(t.Context(), `
		WITH
		member AS (INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member') RETURNING id),
		inbox AS (INSERT INTO inboxes (workspace_id, name) VALUES ($1, 'Support') RETURNING id),
		connection AS (
			INSERT INTO channel_connections (workspace_id, inbox_id, type, name)
			SELECT $1, id, 'widget', 'Website' FROM inbox RETURNING id, inbox_id
		),
		company AS (INSERT INTO companies (workspace_id, name) VALUES ($1, 'Acme') RETURNING id),
		contact AS (
			INSERT INTO contacts (workspace_id, kind, company_id) SELECT $1, 'lead', id FROM company RETURNING id
		),
		identity AS (
			INSERT INTO contact_identities (workspace_id, contact_id, type, value)
			SELECT $1, id, 'widget_visitor', 'v1' FROM contact
		),
		conversation AS (
			INSERT INTO conversations (workspace_id, number, inbox_id, channel_connection_id, contact_id)
			SELECT $1, 1, connection.inbox_id, connection.id, contact.id FROM connection, contact RETURNING id
		),
		participant AS (
			INSERT INTO conversation_participants (workspace_id, conversation_id, contact_id, role)
			SELECT $1, conversation.id, contact.id, 'cc' FROM conversation, contact
		),
		message AS (
			INSERT INTO messages (workspace_id, conversation_id, type, author_type, body)
			SELECT $1, id, 'message', 'contact', 'Hello' FROM conversation RETURNING id
		),
		attachment AS (
			INSERT INTO attachments (
				workspace_id, message_id, uploaded_by_type, disk, path, filename, content_type, size_bytes
			) SELECT $1, id, 'contact', 'local', 'a/b.png', 'b.png', 'image/png', 10 FROM message
		),
		tag AS (INSERT INTO tags (workspace_id, name) VALUES ($1, 'billing') RETURNING id),
		tagged AS (
			INSERT INTO conversation_tag (workspace_id, conversation_id, tag_id, created_by_type)
			SELECT $1, conversation.id, tag.id, 'member' FROM conversation, tag
		),
		reads AS (
			INSERT INTO conversation_reads (workspace_id, conversation_id, workspace_member_id, last_read_at)
			SELECT $1, conversation.id, member.id, now() FROM conversation, member
		),
		reply AS (INSERT INTO saved_replies (workspace_id, title, body) VALUES ($1, 'Hi', 'Hello there'))
		SELECT id::text FROM conversation`, workspace, user).Scan(&id)
	require.NoError(t, err)

	return workspace
}

/*
countRows counts the rows of each table the querier sees, of the
workspace only when one is given.
*/
func countRows(ctx context.Context, t *testing.T, q database.Querier, workspace string) map[string]int {
	t.Helper()

	counts := map[string]int{}
	for _, table := range inboxTables() {
		var n int
		err := q.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE $1 = '' OR workspace_id::text = $1",
			workspace).Scan(&n)
		require.NoError(t, err, table)
		counts[table] = n
	}

	return counts
}

/*
filled returns the count every inbox table has when it holds n rows.
*/
func filled(n int) map[string]int {
	counts := map[string]int{}
	for _, table := range inboxTables() {
		counts[table] = n
	}

	return counts
}

func TestTenantTables(t *testing.T) {
	t.Parallel()

	t.Run("give every table with a workspace row level security through the visible workspaces", func(t *testing.T) {
		t.Parallel()
		db := open(t, tests.MigratedDatabase(t).URL)

		var unprotected []string
		err := db.QueryRow(t.Context(), `
			SELECT coalesce(array_agg(c.relname::text), '{}') FROM pg_class c
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'workspace_id' AND NOT a.attisdropped
			JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
			WHERE c.relkind = 'r' AND c.relname <> 'workspaces' AND NOT (
				c.relrowsecurity AND EXISTS (
					SELECT 1 FROM pg_policies p
					WHERE p.tablename = c.relname AND p.qual LIKE '%dimmi_visible_workspaces()%'
				)
			)`).Scan(&unprotected)
		require.NoError(t, err)

		assert.Empty(t, unprotected)
	})

	t.Run("show the application role its own workspace's rows and none of another's", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		owner := open(t, migrated.URL)
		ours, theirs := seedInbox(t, owner), seedInbox(t, owner)
		app := open(t, appRole(t, migrated))

		var visible, leaked map[string]int
		err := app.Transaction(tenant(ctx, ours), func(ctx context.Context, tx *database.Tx) error {
			visible = countRows(ctx, t, tx, "")
			leaked = countRows(ctx, t, tx, theirs)

			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, filled(1), visible)
		assert.Equal(t, filled(0), leaked)
		assert.Equal(t, filled(2), countRows(ctx, t, owner, ""))
	})

	t.Run("show the application role nothing while no workspace is set", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		seedInbox(t, open(t, migrated.URL))
		app := open(t, appRole(t, migrated))

		var inTransaction map[string]int
		err := app.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			inTransaction = countRows(ctx, t, tx, "")

			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, filled(0), countRows(ctx, t, app, ""))
		assert.Equal(t, filled(0), inTransaction)
	})

	t.Run("refuse the application role a row written into another workspace", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		owner := open(t, migrated.URL)
		ours, theirs := seedWorkspace(t, owner), seedWorkspace(t, owner)
		role := appRole(t, migrated)
		u, err := url.Parse(role)
		require.NoError(t, err)
		require.NoError(t, exec(ctx, open(t, migrated.AdminURL), "GRANT INSERT ON tags TO "+u.User.Username()))
		app := open(t, role)

		insert := func(workspace string) error {
			return app.Transaction(tenant(ctx, ours), func(ctx context.Context, tx *database.Tx) error {
				return exec(ctx, tx, "INSERT INTO tags (workspace_id, name) VALUES ($1, 'vip')", workspace)
			})
		}

		require.NoError(t, insert(ours))
		require.ErrorContains(t, insert(theirs), "row-level security")
	})
}
