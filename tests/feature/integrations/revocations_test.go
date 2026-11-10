package integrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

func TestDisconnect(t *testing.T) {
	t.Parallel()

	t.Run("gives every token back and removes the installation", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		grant := slackGrant("T1", s.allScopes())
		public := s.install(t, grant)
		user := userGrant("T1", "U1")
		s.grantAs(t, s.owner, public, user)
		disconnected := tests.Record[appevents.IntegrationDisconnected](s.tc)

		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(202)

		assert.ElementsMatch(t, []string{grant.Access.Value, user.UserAccess.Value}, s.fake.Revoked())
		require.Len(t, disconnected.All(), 1)
		assert.Equal(t, integrations.ReasonDisconnected, disconnected.All()[0].Reason)
		s.owner.GetJSON("/api/v1/integrations").AssertOK().AssertJSONPath("data.#", float64(0))
		s.owner.GetJSON("/api/v1/integrations/" + public).AssertNotFound()
		s.tc.Table("integration_member_grants").AssertHas(testkit.Row{"status": "revoked"})
		s.tc.Table("audit_logs").AssertHas(testkit.Row{"event": "integration.disconnected"})
	})

	t.Run("refuses teammates who do not manage integrations", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		member, _ := s.join(t, enums.RoleMember)

		member.Delete("/api/v1/integrations/" + public).AssertForbidden()
		s.owner.Delete("/api/v1/integrations/itg_nope").AssertNotFound()
	})

	t.Run("reconnects the same team on the row it had", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		err := s.tc.App.Actions.Integrations.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", Reason: integrations.ReasonUninstalled, Bot: true,
		})
		require.NoError(t, err)
		connected := tests.Record[appevents.IntegrationConnected](s.tc)

		again := s.install(t, slackGrant("T1", s.allScopes()))

		assert.Equal(t, public, again)
		require.Len(t, connected.All(), 1)
		assert.True(t, connected.All()[0].Reconnected)
	})
}

func TestProviderRevocation(t *testing.T) {
	t.Parallel()

	t.Run("disconnects an uninstalled app and its grants at once", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.grantAs(t, s.owner, public, userGrant("T1", "U1"))
		disconnected := tests.Record[appevents.IntegrationDisconnected](s.tc)

		err := s.tc.App.Actions.Integrations.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", Reason: integrations.ReasonUninstalled, Bot: true,
		})

		require.NoError(t, err)
		require.Len(t, disconnected.All(), 1)
		assert.Equal(t, integrations.ReasonUninstalled, disconnected.All()[0].Reason)
		assert.Empty(t, s.fake.Revoked())
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().AssertJSONPath("data.status", "disconnected")
		s.tc.Table("integration_member_grants").AssertHas(testkit.Row{"status": "revoked"})
		_, err = s.tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), s.id(t, public))
		require.ErrorIs(t, err, integrations.ErrIntegrationDisconnected)
	})

	t.Run("revokes only the grants of the slack users named", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		member, row := s.join(t, enums.RoleMember)
		s.grantAs(t, s.owner, public, userGrant("T1", "U1"))
		s.grantAs(t, member, public, userGrant("T1", "U2"))
		revoke := s.tc.App.Actions.Integrations.ApplyRevocation

		require.NoError(t, revoke.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", Reason: integrations.ReasonRevoked,
			UserIDs: []string{"U2"},
		}))
		require.NoError(t, revoke.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", Reason: integrations.ReasonRevoked,
		}))

		s.tc.Table("integration_member_grants").AssertHas(testkit.Row{"member_id": row.ID, "status": "revoked"})
		s.tc.Table("integration_member_grants").AssertHas(testkit.Row{"external_user_id": "U1", "status": "active"})
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().AssertJSONPath("data.status", "active")
	})

	t.Run("ignores a report about a team nobody installed", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		require.NoError(t, s.tc.App.Actions.Integrations.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T404", Bot: true,
		}))
	})
}
