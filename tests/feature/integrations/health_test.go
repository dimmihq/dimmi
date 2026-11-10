package integrations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
authTest is what Slack's auth.test answers for a bot of the team.
*/
func authTest(team string) httpclient.Response {
	return httpclient.Response{
		Header: fhttp.Header{"X-Oauth-Scopes": {"chat:write, channels:read"}},
		Body: []byte(`{"ok":true,"url":"https://acme.slack.com/","team":"Acme ` + team + `","team_id":"` + team +
			`","user_id":"UB9","bot_id":"B9"}`),
	}
}

/*
stubSlack answers auth.test and bots.info.
*/
func stubSlack(s slackCase, authAnswers ...httpclient.Response) *httpclienttest.Fake {
	return httpclienttest.New(s.tc.App.HTTP).
		StubSequence("POST https://slack.com/api/auth.test", authAnswers...).
		Stub("POST https://slack.com/api/bots.info",
			httpclient.Response{Body: []byte(`{"ok":true,"bot":{"app_id":"A9"}}`)})
}

func failure(code string) httpclient.Response {
	return httpclient.Response{Body: []byte(`{"ok":false,"error":"` + code + `"}`)}
}

/*
owner is the owner's membership with every permission.
*/
func (s slackCase) ownerMembership() authz.Membership {
	return authz.NewMembership(s.tc.Workspace.Workspace, s.tc.Workspace.Owner, appauth.RolePermissions(enums.RoleOwner))
}

func TestHealth(t *testing.T) {
	t.Parallel()

	t.Run("marks a refused installation broken once and brings it back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		stubSlack(s, failure("invalid_auth"), failure("token_revoked"), authTest("T1"))
		errored := tests.Record[appevents.IntegrationErrored](s.tc)
		check := s.tc.App.Actions.Integrations.CheckHealth

		require.NoError(t, check.Due(t.Context()))
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.status", "error").AssertJSONPath("data.last_error_code", "slack_auth_failed")
		require.NoError(t, check.Due(t.Context()))
		s.tc.Clock.Advance(25 * time.Hour)
		require.NoError(t, check.Due(t.Context()))
		require.NoError(t, check.Due(t.Context()))

		assert.Len(t, errored.All(), 1)
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.status", "error")
		s.tc.Clock.Advance(25 * time.Hour)
		require.NoError(t, check.Due(t.Context()))
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.status", "active").AssertJSONPath("data.name", "Acme T1").
			AssertJSONPath("data.metadata.team_domain", "acme")
	})

	t.Run("moves on when slack does not answer", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		stubSlack(s, httpclient.Response{Status: fhttp.StatusBadGateway})

		require.Error(t, s.tc.App.Actions.Integrations.CheckHealth.Due(t.Context()))
		require.NoError(t, s.tc.App.Actions.Integrations.CheckHealth.Due(t.Context()))
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().AssertJSONPath("data.status", "active")
	})

	t.Run("skips installations that are gone", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		require.NoError(t, s.tc.App.Actions.Integrations.CheckHealth.Handle(t.Context(),
			s.tc.Workspace.Workspace.ID, ids.New()))
	})
}

func TestManualInstallation(t *testing.T) {
	t.Parallel()

	t.Run("connects slack from a custom app's pasted tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		stubSlack(s, authTest("T9"))
		connected := tests.Record[appevents.IntegrationConnected](tc)

		in := s.manual("xoxb-custom")
		in.Features = []string{"channels"}
		row, err := tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), in)

		require.NoError(t, err)
		assert.Equal(t, "manual", row.Source)
		assert.Equal(t, "A9", *row.ExternalAppID)
		assert.Equal(t, []string{"chat:write", "channels:read"}, row.Scopes)
		require.Len(t, connected.All(), 1)
		assert.Equal(t, "manual", connected.All()[0].Source)
		token, err := tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), row.ID)
		require.NoError(t, err)
		assert.Equal(t, "xoxb-custom", token)
	})

	t.Run("rotates tokens only for the same team", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		stubSlack(s, authTest("T9"), authTest("T9"), authTest("T8"))
		create := tc.App.Actions.Integrations.CreateManual
		in := s.manual("xoxb-1")

		row, err := create.Handle(t.Context(), in)
		require.NoError(t, err)
		in.InstallationID, in.Token = row.ID, "xoxb-2"
		_, err = create.Handle(t.Context(), in)
		require.NoError(t, err)
		_, err = create.Handle(t.Context(), in)
		require.ErrorIs(t, err, exceptions.ErrIntegrationAccountMismatch)

		tc.Table("integration_installations").AssertCount(1)
	})

	t.Run("refuses a token slack refuses and teammates without the right", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := slackCase{tc: tc, owner: tc.In(tc.Owner)}
		stubSlack(s, failure("invalid_auth"), httpclient.Response{Status: fhttp.StatusBadGateway})
		create := tc.App.Actions.Integrations.CreateManual
		in := s.manual("xoxb-x")

		_, err := create.Handle(t.Context(), in)
		require.ErrorIs(t, err, exceptions.ErrIntegrationAuthFailed)
		_, err = create.Handle(t.Context(), in)
		require.ErrorIs(t, err, exceptions.ErrIntegrationProvider)
		in.Provider = "linear"
		_, err = create.Handle(t.Context(), in)
		require.ErrorIs(t, err, exceptions.ErrIntegrationUnavailable)
		in.Membership = authz.NewMembership(tc.Workspace.Workspace, models.WorkspaceMember{}, nil)
		_, err = create.Handle(t.Context(), in)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		tc.Table("integration_installations").AssertCount(0)
	})
}
