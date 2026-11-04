package inbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
benchConversations is how many conversations each workspace of the
benchmark holds, enough that the list reads an index rather than the
whole table.
*/
const benchConversations = 20000

/*
seedConversations fills the workspace's default inbox with conversations
from one contact on its API connection, a minute apart, as the tables'
owner.
*/
func seedConversations(b *testing.B, tc *tests.TestCase, workspace ids.UUID) {
	b.Helper()

	_, err := tc.Operator().Exec(b.Context(), `
		WITH contact AS (
			INSERT INTO contacts (workspace_id, kind, name) VALUES ($1, 'lead', 'Ada') RETURNING id
		),
		connection AS (
			SELECT id, inbox_id FROM channel_connections WHERE workspace_id = $1 AND type = 'api'
		)
		INSERT INTO conversations (
			workspace_id, number, inbox_id, channel_connection_id, contact_id, status, last_message_at,
			created_at, updated_at
		)
		SELECT $1, n, connection.inbox_id, connection.id, contact.id, 'open', stamp.at AS last_message_at,
			stamp.at AS created_at, stamp.at AS updated_at
		FROM generate_series(1, $2::int) AS n, connection, contact,
			LATERAL (SELECT now() - n * interval '1 minute' AS at) AS stamp`, workspace, benchConversations)
	require.NoError(b, err)
}

/*
listFirstPage reads the first page of the workspace's conversations,
in a transaction scoped to it, the way the inbox opens.
*/
func listFirstPage(b *testing.B, db *database.DB, workspace ids.UUID) {
	b.Helper()

	ctx := tenancy.With(b.Context(), workspace.String())
	err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		page, err := models.New(tx).ListConversations(ctx, models.ListConversationsParams{
			WorkspaceID: workspace, RowLimit: 50,
		})
		if len(page) != 50 {
			b.Fatalf("listed %d conversations, want 50", len(page))
		}

		return err
	})
	require.NoError(b, err)
}

/*
ownerPool connects to the test's database as the tables' owner, with the
same pool the application role gets, so the two differ only in the
policy and not in the work the application's own pool shares.
*/
func ownerPool(b *testing.B, tc *tests.TestCase) *database.DB {
	b.Helper()

	db, err := database.Open(b.Context(), database.Config{
		URL: tc.Database.URL, MaxConns: 2, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
	})
	require.NoError(b, err)
	b.Cleanup(db.Close)

	return db
}

/*
BenchmarkConversationListTenantPolicy measures what the tenant policy
costs the inbox's busiest read: the first page of a workspace's
conversations, out of 20,000, beside another workspace's 20,000. The
same query runs as the tables' owner, whom no policy applies to, and
as the application role, whom the tenant policy filters. Both run in a
transaction scoped to the workspace.

Run it with:

	go test -run '^$' -bench ConversationListTenantPolicy ./tests/feature/inbox/

On an Apple M3 Max against the compose PostgreSQL 18, both read the
page in 0.6 to 1 ms, nearly all of it the transaction's round trips,
and the two ranges overlap from run to run, so the policy's cost stays
well inside the 5 percent the inbox allows. The plans show why: the
policy's check folds into the condition on the same workspace index,
both read the same five buffers, and it adds about 5 µs of planning
to a statement that runs in under 20 µs. It is a benchmark, never run
by go test without -bench, since seeding takes seconds and timing on
a shared machine is too noisy to assert.
*/
func BenchmarkConversationListTenantPolicy(b *testing.B) {
	tc := tests.New(b)
	workspace := tc.Workspace.Workspace.ID
	other, err := factories.Workspace(b.Context(), tc.App, tc.Owner)
	require.NoError(b, err)
	seedConversations(b, tc, workspace)
	seedConversations(b, tc, other.Workspace.ID)
	_, err = tc.Operator().Exec(b.Context(), "ANALYZE conversations")
	require.NoError(b, err)

	for _, run := range []struct {
		name string
		db   *database.DB
	}{
		{"without policy", ownerPool(b, tc)},
		{"with policy", tc.ApplicationRole()},
	} {
		listFirstPage(b, run.db, workspace)
		b.Run(run.name, func(b *testing.B) {
			for b.Loop() {
				listFirstPage(b, run.db, workspace)
			}
		})
	}
}
