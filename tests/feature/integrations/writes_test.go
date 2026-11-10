package integrations_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
grantCallback starts the owner's grant and returns the request URI the
provider sends the browser back to.
*/
func (s slackCase) grantCallback(t *testing.T, public string, grant oauthclient.Grant) string {
	t.Helper()

	res := s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()
	callback, err := s.fake.Approve(res.String("data.authorize_url"), grant)
	require.NoError(t, err)

	parsed, err := url.Parse(callback)
	require.NoError(t, err)

	return parsed.RequestURI()
}

func TestFailedWrites(t *testing.T) {
	t.Parallel()

	for name, failing := range map[string][]string{
		"grant links cannot be cleared":   {"channel_member_links", "DELETE"},
		"grant links cannot be written":   {"channel_member_links", "INSERT"},
		"older grants cannot be replaced": {"integration_member_grants", "DELETE"},
		"grants cannot be written":        {"integration_member_grants", "INSERT"},
		"grant audits cannot be written":  {"audit_logs", "INSERT"},
	} {
		t.Run("refuses a grant when "+name, func(t *testing.T) {
			t.Parallel()
			s := newSlackCase(t)
			public := s.install(t, slackGrant("T1", s.allScopes()))
			s.backChannel(t, s.id(t, public))
			s.tc.FailWrites(failing[0], failing[1])

			s.owner.Send(fhttp.MethodGet, s.grantCallback(t, public, userGrant("T1", "U1")), nil).
				AssertStatus(fhttp.StatusInternalServerError)
		})
	}

	for name, failing := range map[string][]string{
		"grants cannot be revoked":      {"integration_member_grants", "UPDATE"},
		"revocations cannot be audited": {"audit_logs", "INSERT"},
		"links cannot be removed":       {"channel_member_links", "DELETE"},
	} {
		t.Run("keeps a grant when "+name, func(t *testing.T) {
			t.Parallel()
			s := newSlackCase(t)
			public := s.install(t, slackGrant("T1", s.allScopes()))
			s.backChannel(t, s.id(t, public))
			s.grantAs(t, s.owner, public, userGrant("T1", "U1"))
			grant := s.grantOf(t, s.tc.Workspace.Owner.ID)
			s.tc.FailWrites(failing[0], failing[1])

			s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/" + grant).
				AssertStatus(fhttp.StatusInternalServerError)
			err := s.tc.App.Actions.Integrations.RevokeDeparted.Handle(t.Context(),
				s.tc.Workspace.Workspace.ID, s.tc.Workspace.Owner.ID)
			assert.Equal(t, failing[1] != "UPDATE", err != nil)
		})
	}

	t.Run("keeps a departed teammate's grant it cannot delete", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.grantAs(t, s.owner, public, userGrant("T1", "U1"))
		s.tc.FailWrites("integration_member_grants", "DELETE")

		require.Error(t, s.tc.App.Actions.Integrations.RevokeDeparted.Handle(t.Context(),
			s.tc.Workspace.Workspace.ID, s.tc.Workspace.Owner.ID))
	})

	t.Run("deletes a departed teammate's grant on a removed installation", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.grantAs(t, s.owner, public, userGrant("T1", "U1"))
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(202)

		require.NoError(t, s.tc.App.Actions.Integrations.RevokeDeparted.Handle(t.Context(),
			s.tc.Workspace.Workspace.ID, s.tc.Workspace.Owner.ID))
		s.tc.Table("integration_member_grants").AssertCount(0)
	})

	t.Run("refuses a grant on an installation that broke meanwhile", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.quietInstall(t, s.expiring("xoxe-dead"))
		uri := s.grantCallback(t, public, userGrant("T1", "U1"))
		_, err := s.tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), s.id(t, public))
		require.ErrorIs(t, err, integrations.ErrIntegrationBroken)

		landing := follow(t, s.owner, uri)
		assert.Equal(t, "integration_not_active", landing.Query().Get("error"))
	})

	t.Run("keeps an installation it cannot disconnect", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.tc.FailWrites("integration_installations", "UPDATE")

		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("reports a refused renewal it cannot record", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.quietInstall(t, s.expiring("xoxe-dead"))
		grant := userGrant("T1", "U1")
		grant.UserAccess.Expiry = s.tc.Clock.Now()
		grant.UserRefresh = &oauthclient.Token{Value: "xoxe-u-dead"}
		s.grantAs(t, s.owner, public, grant)
		s.tc.FailWrites("integration_installations", "UPDATE")
		s.tc.FailWrites("integration_member_grants", "UPDATE")
		tokens := s.tc.App.Actions.Integrations.Tokens

		_, _, err := tokens.User(s.scoped(t), s.id(t, public), s.tc.Workspace.Owner.ID)
		require.Error(t, err)
		require.NotErrorIs(t, err, integrations.ErrNoMemberGrant)
		_, err = tokens.Bot(s.scoped(t), s.id(t, public))
		require.Error(t, err)
		require.NotErrorIs(t, err, integrations.ErrIntegrationBroken)
	})

	t.Run("reports a renewed member token it cannot store", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		grant := userGrant("T1", "U1")
		grant.UserAccess.Expiry = s.tc.Clock.Now()
		grant.UserRefresh = &oauthclient.Token{Value: "xoxe-u"}
		s.grantAs(t, s.owner, public, grant)
		s.fake.AcceptRefresh("xoxe-u", userGrant("T1", "U1"))
		s.tc.FailWrites("integration_member_grants", "UPDATE")

		_, _, err := s.tc.App.Actions.Integrations.Tokens.User(s.scoped(t), s.id(t, public), s.tc.Workspace.Owner.ID)
		require.Error(t, err)
	})
}

func TestProviderDetails(t *testing.T) {
	t.Parallel()

	t.Run("keeps the grid organization of an install", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		grant := slackGrant("T1", s.allScopes())
		grant.Raw["enterprise"] = map[string]any{"id": "E1"}

		public := s.install(t, grant)

		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().AssertJSONPath("data.metadata.enterprise_id", "E1")
	})

	t.Run("reads a token without a bot and its grid organization", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		stubSlack(s, httpclient.Response{Body: []byte(
			`{"ok":true,"url":"https://grid.slack.com/","team":"","team_id":"T7","enterprise_id":"E7"}`)})

		row, err := tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), s.manual("xoxb-1"))

		require.NoError(t, err)
		assert.Nil(t, row.ExternalAppID)
		assert.Equal(t, "T7", row.Name)
		assert.Contains(t, string(row.Metadata), "E7")
	})

	t.Run("reports slack failing to name the app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		fake := stubSlack(s, authTest("T7"))
		fake.Stub("POST https://slack.com/api/bots.info", httpclient.Response{Status: fhttp.StatusInternalServerError})

		_, err := tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), s.manual("xoxb-1"))
		require.Error(t, err)
	})

	t.Run("adds to the scopes a feature registered before", func(t *testing.T) {
		t.Parallel()
		registry := oauth.NewRegistry()
		registry.RegisterScopes("slack", "channels", oauth.Scopes{Bot: []string{"b"}})
		registry.RegisterScopes("slack", "channels", oauth.Scopes{Bot: []string{"a", "b"}, User: []string{"u"}})

		assert.Equal(t, []string{"a", "b"}, registry.BotScopes("slack", []string{"channels"}))
		assert.Equal(t, []string{"u"}, registry.UserScopes("slack"))
	})

	t.Run("refuses a grant to come back elsewhere or without the shared app", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants",
			testkit.JSON{"return_to": "https://evil.test"}).AssertValidationError("return_to")

		tc := tests.New(t)
		other := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		stubSlack(other, authTest("T9"))
		row, err := tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), other.manual("xoxb-1"))
		require.NoError(t, err)
		path := "/api/v1/integrations/" + models.PublicID(models.PrefixIntegrationInstallation, row.ID)
		other.owner.PostJSON(path+"/member-grants", testkit.JSON{}).
			AssertNotFound()
	})
}
