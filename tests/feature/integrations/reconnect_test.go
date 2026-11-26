package integrations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
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

	t.Run("connecting the same workspace again renews its connection instead of adding one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_SERVICES__NOTION__CLIENT_ID=notion-id", "DIMMI_SERVICES__NOTION__CLIENT_SECRET=notion-secret",
		))
		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().Stub("GET https://api.notion.com/v1/users/me",
			httpclient.Response{
				Status: 200, Header: fhttp.Header{"Content-Type": {"application/json"}},
				Body: []byte(`{"bot":{"workspace_id":"ws-acme","workspace_name":"Acme"}}`),
			})
		fake, owner := tc.App.OAuthClient.Fake(), tc.In(tc.Owner)

		connect := func() string {
			started := owner.PostJSON("/api/v1/integrations/notion/installations", testkit.JSON{
				"features": []string{"knowledge"},
			}).AssertOK()
			callback, err := fake.Approve(started.String("data.authorize_url"), oauthclient.Grant{
				Access:  &oauthclient.Token{Value: "secret_" + ids.New().String()},
				Account: "ws-acme", AccountName: "Acme",
			})
			require.NoError(t, err)

			return follow(t, owner, callback).Query().Get("connected")
		}

		first, second := connect(), connect()
		require.NotEmpty(t, first)
		require.Equal(t, first, second)
		tc.Table("integration_installations").AssertCount(1, testkit.Row{"provider": "notion"})
	})
}
