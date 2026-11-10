package integrations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/jobs"
)

func TestWorkspacePurgeRevokesIntegrations(t *testing.T) {
	t.Parallel()

	t.Run("purging a workspace gives its Slack token back before its rows go", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		grant := slackGrant("T1", s.allScopes())
		s.quietInstall(t, grant)

		ws := s.tc.Workspace.Workspace.ID
		_, err := s.tc.Operator().Exec(t.Context(),
			"UPDATE workspaces SET deleted_at = $1, purge_after = $1 WHERE id = $2",
			s.tc.Clock.Now().Add(-time.Minute), ws)
		require.NoError(t, err)
		require.NoError(t, s.tc.App.Queue.Push(t.Context(), jobs.PurgeWorkspaceArgs{WorkspaceID: ws}))
		s.tc.Drain()

		assert.Contains(t, s.fake.Revoked(), grant.Access.Value)
		s.tc.Table("integration_installations").AssertCount(0)
	})
}
