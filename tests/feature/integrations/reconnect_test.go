package integrations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
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

	t.Run("connecting the same help desk again renews its connection instead of adding one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_SERVICES__ZENDESK__CLIENT_ID=zd-id", "DIMMI_SERVICES__ZENDESK__CLIENT_SECRET=zd-secret",
		))
		fake, owner := tc.App.OAuthClient.Fake(), tc.In(tc.Owner)

		connect := func() string {
			started := owner.PostJSON("/api/v1/integrations/zendesk/installations", testkit.JSON{
				"features": []string{"knowledge"}, "account": "acme",
			}).AssertOK()
			callback, err := fake.Approve(started.String("data.authorize_url"), oauthclient.Grant{
				Access: &oauthclient.Token{Value: "zd-" + ids.New().String()}, Account: "acme", AccountName: "acme",
			})
			require.NoError(t, err)

			return follow(t, owner, callback).Query().Get("connected")
		}

		first, second := connect(), connect()
		require.NotEmpty(t, first)
		require.Equal(t, first, second)
		tc.Table("integration_installations").AssertCount(1, testkit.Row{"provider": "zendesk"})
	})
}
