package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
budgetConversations is how many conversations the list budget is held to.
*/
const budgetConversations = 100000

/*
listBudget is the longest a first page may take for a restricted member.
*/
const listBudget = 50 * time.Millisecond

/*
seedBudgetConversations fills the workspace with conversations a minute
apart, split across the team and nobody, as the tables' owner.
*/
func seedBudgetConversations(t *testing.T, d desk, team ids.UUID) {
	t.Helper()

	_, err := d.tc.App.DB.Exec(t.Context(), `
		WITH contact AS (
			INSERT INTO contacts (workspace_id, kind, name) VALUES ($1, 'lead', 'Ada') RETURNING id
		)
		INSERT INTO conversations (
			workspace_id, number, inbox_id, channel_connection_id, contact_id, status, assignee_team_id,
			last_message_at
		)
		SELECT $1, n, $2, $3, contact.id, 'open', CASE WHEN n % 2 = 0 THEN $4::uuid END,
			now() - n * interval '1 minute'
		FROM generate_series(1, $5::int) AS n, contact`,
		d.workspaceUUID(), d.inbox.Inbox.ID, d.inbox.API.ID, team, budgetConversations)
	require.NoError(t, err)

	_, err = d.tc.App.DB.Exec(t.Context(), "ANALYZE conversations")
	require.NoError(t, err)
}

/*
firstPage returns how long the quickest of ten first pages took for the
scope, the quickest because other work on a shared server only ever
slows a run down.
*/
func firstPage(t *testing.T, d desk, scope authz.ConversationScope, memberID ids.UUID) time.Duration {
	t.Helper()

	workspace := d.workspaceUUID()
	access := scope.Params()
	wall := clock.System()
	best := time.Hour

	for range 10 {
		started := wall.Now()

		err := d.tc.App.DB.Transaction(tenancy.With(t.Context(), workspace.String()),
			func(ctx context.Context, tx *database.Tx) error {
				sql, args := querybuilder.Conversations{
					WorkspaceID: workspace, MemberID: memberID, Statuses: []string{"open"},
					Sort: querybuilder.SortLatest, Limit: 50, Access: querybuilder.Access{
						Mode: access.Mode, Viewer: access.Viewer, Teams: access.Teams, Unassigned: access.Unassigned,
					},
				}.IDs()

				rows, err := tx.Query(ctx, sql, args...)
				if err != nil {
					return err
				}
				defer rows.Close()

				count := 0
				for rows.Next() {
					count++
				}

				assert.Equal(t, 50, count)

				return rows.Err()
			})
		require.NoError(t, err)

		best = min(best, wall.Since(started))
	}

	return best
}

func TestListBudgets(t *testing.T) {
	t.Parallel()

	d := open(t)
	team, err := factories.Team(t.Context(), d.tc.App, factories.TeamOf{
		Workspace: d.tc.Workspace.Workspace, Name: "Billing",
	})
	require.NoError(t, err)
	seedBudgetConversations(t, d, team.ID)

	me := d.tc.Workspace.Owner.ID

	t.Run("a member who sees everything stays within budget", func(t *testing.T) {
		took := firstPage(t, d, authz.ConversationScope{Mode: enums.ConversationAccessAll, MemberID: &me}, me)

		t.Logf("all: %s", took)
		assert.Less(t, took, listBudget)
	})

	t.Run("a restricted member stays within budget", func(t *testing.T) {
		took := firstPage(t, d, authz.ConversationScope{
			Mode: enums.ConversationAccessTeams, MemberID: &me, TeamIDs: []ids.UUID{team.ID},
		}, me)

		t.Logf("teams: %s", took)
		assert.Less(t, took, listBudget)
	})
}
