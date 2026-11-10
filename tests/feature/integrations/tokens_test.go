package integrations_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

func TestTokens(t *testing.T) {
	t.Parallel()

	t.Run("hands out the app token of a working installation", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		grant := slackGrant("T1", s.allScopes())
		id := s.id(t, s.install(t, grant))

		token, err := s.tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), id)

		require.NoError(t, err)
		assert.Equal(t, grant.Access.Value, token)
		assert.Empty(t, s.fake.Refreshed())
	})

	t.Run("renews an expiring token once for concurrent callers", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		id := s.id(t, s.quietInstall(t, s.expiring("xoxe-1")))
		fresh := &oauthclient.Token{Value: "xoxb-fresh", Expiry: s.tc.Clock.Now().Add(12 * time.Hour)}
		s.fake.AcceptRefresh("xoxe-1", oauthclient.Grant{Access: fresh, Refresh: &oauthclient.Token{Value: "xoxe-2"}})

		var wg sync.WaitGroup
		tokens := make([]string, 4)
		errs := make([]error, 4)
		for i := range tokens {
			wg.Go(func() { tokens[i], errs[i] = s.tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), id) })
		}
		wg.Wait()

		for i := range tokens {
			require.NoError(t, errs[i])
			assert.Equal(t, "xoxb-fresh", tokens[i])
		}
		assert.Equal(t, []string{"xoxe-1"}, s.fake.Refreshed())
	})

	t.Run("marks the installation broken when the renewal is refused", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		errored := tests.Record[appevents.IntegrationErrored](s.tc)
		public := s.quietInstall(t, s.expiring("xoxe-dead"))
		id := s.id(t, public)
		tokens := s.tc.App.Actions.Integrations.Tokens

		_, err := tokens.Bot(s.scoped(t), id)
		require.ErrorIs(t, err, integrations.ErrIntegrationBroken)
		_, err = tokens.Bot(s.scoped(t), id)
		require.ErrorIs(t, err, integrations.ErrIntegrationBroken)

		require.Len(t, errored.All(), 1)
		assert.Equal(t, "refresh_invalid_grant", errored.All()[0].Code)
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.status", "error").AssertJSONPath("data.last_error_code", "refresh_invalid_grant")
	})

	t.Run("says an installation that is gone is disconnected", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(202)
		tokens := s.tc.App.Actions.Integrations.Tokens

		_, err := tokens.Bot(s.scoped(t), s.id(t, public))
		require.ErrorIs(t, err, integrations.ErrIntegrationDisconnected)
		_, err = tokens.Bot(s.scoped(t), ids.New())
		require.ErrorIs(t, err, integrations.ErrIntegrationDisconnected)
		_, _, err = tokens.User(s.scoped(t), s.id(t, public), s.tc.Workspace.Owner.ID)
		require.ErrorIs(t, err, integrations.ErrIntegrationDisconnected)
	})

	t.Run("needs the workspace in the context", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		_, err := s.tc.App.Actions.Integrations.Tokens.Bot(t.Context(), ids.New())
		require.Error(t, err)
	})
}

func TestUserTokens(t *testing.T) {
	t.Parallel()

	t.Run("hands out a member's own token and who they are", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		grant := userGrant("T1", "U1")
		s.grantAs(t, s.owner, public, grant)

		token, user, err := s.tc.App.Actions.Integrations.Tokens.User(s.scoped(t), s.id(t, public),
			s.tc.Workspace.Owner.ID)

		require.NoError(t, err)
		assert.Equal(t, grant.UserAccess.Value, token)
		assert.Equal(t, "U1", user)
	})

	t.Run("says a member without a grant has none", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))

		_, _, err := s.tc.App.Actions.Integrations.Tokens.User(s.scoped(t), s.id(t, public), s.tc.Workspace.Owner.ID)
		require.ErrorIs(t, err, integrations.ErrNoMemberGrant)
	})

	t.Run("renews a member's expiring token, and drops a refused one", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		grant := userGrant("T1", "U1")
		grant.UserAccess.Expiry = s.tc.Clock.Now().Add(time.Minute)
		grant.UserRefresh = &oauthclient.Token{Value: "xoxe-u1"}
		s.grantAs(t, s.owner, public, grant)
		fresh := oauthclient.Grant{
			UserAccess: &oauthclient.Token{Value: "xoxp-fresh", Expiry: s.tc.Clock.Now().Add(time.Minute)},
		}
		s.fake.AcceptRefresh("xoxe-u1", fresh)
		tokens := s.tc.App.Actions.Integrations.Tokens
		owner := s.tc.Workspace.Owner.ID

		token, _, err := tokens.User(s.scoped(t), s.id(t, public), owner)
		require.NoError(t, err)
		assert.Equal(t, "xoxp-fresh", token)

		_, _, err = tokens.User(s.scoped(t), s.id(t, public), owner)
		require.ErrorIs(t, err, integrations.ErrNoMemberGrant)
		s.tc.Table("integration_member_grants").AssertHas(map[string]any{"status": "error"})
	})
}
