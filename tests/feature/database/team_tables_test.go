package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/tests"
)

/*
seedTeam fills a new workspace with one team holding one member, as the
tables' owner, and returns the workspace's id.
*/
func seedTeam(t *testing.T, db *database.DB) string {
	t.Helper()

	workspace := seedWorkspace(t, db)
	user := seedUser(t, db, "agent")
	err := exec(t.Context(), db, `
		WITH
		member AS (INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member') RETURNING id),
		team AS (INSERT INTO teams (workspace_id, name) VALUES ($1, 'Support') RETURNING id)
		INSERT INTO team_members (team_id, workspace_id, workspace_member_id)
		SELECT team.id, $1, member.id FROM team, member`, workspace, user)
	require.NoError(t, err)

	return workspace
}

/*
countTeamRows counts the teams and team members the querier sees.
*/
func countTeamRows(ctx context.Context, t *testing.T, q database.Querier) [2]int {
	t.Helper()

	var counts [2]int
	err := q.QueryRow(ctx, "SELECT (SELECT count(*) FROM teams), (SELECT count(*) FROM team_members)").
		Scan(&counts[0], &counts[1])
	require.NoError(t, err)

	return counts
}

func TestTeamTables(t *testing.T) {
	t.Parallel()

	t.Run("show the application role only its own workspace's teams and team members", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		owner := open(t, migrated.URL)
		ours := seedTeam(t, owner)
		seedTeam(t, owner)
		app := open(t, appRole(t, migrated))

		var visible, unscoped [2]int
		err := app.Transaction(tenant(ctx, ours), func(ctx context.Context, tx *database.Tx) error {
			visible = countTeamRows(ctx, t, tx)

			return nil
		})
		require.NoError(t, err)
		err = app.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			unscoped = countTeamRows(ctx, t, tx)

			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, [2]int{1, 1}, visible)
		assert.Equal(t, [2]int{0, 0}, unscoped)
		assert.Equal(t, [2]int{2, 2}, countTeamRows(ctx, t, owner))
	})
}
