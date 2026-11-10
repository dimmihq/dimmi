package integrations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestReconnect(t *testing.T) {
	t.Parallel()

	t.Run("installing again after a removal carries on the same slack connection", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.tc.Drain()

		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(202)
		s.tc.Drain()
		s.tc.Table("channel_connections").AssertHas(testkit.Row{"type": "slack", "status": "disconnected"})

		again := s.install(t, slackGrant("T1", s.allScopes()))
		require.NotEqual(t, public, again)
		s.tc.Drain()

		s.tc.Table("channel_connections").AssertCount(1, testkit.Row{"type": "slack"})
		s.tc.Table("channel_connections").AssertHas(testkit.Row{"type": "slack", "status": "active"})
	})
}
