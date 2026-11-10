package integrations_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestInstallation(t *testing.T) {
	t.Parallel()

	t.Run("installs slack with every feature's scopes and announces it", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		connected := tests.Record[appevents.IntegrationConnected](s.tc)

		authURL := start(t, s.owner, testkit.JSON{"features": []string{"notifications"}})
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, strings.Join(s.allScopes(), " "), parsed.Query().Get("scope"))
		assert.Contains(t, parsed.Query().Get("redirect_uri"), "/integrations/slack/callback")

		callback, err := s.fake.Approve(authURL, slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		landing := follow(t, s.owner, callback)

		assert.Equal(t, "/d/settings/integrations", landing.Path)
		public := landing.Query().Get("connected")
		require.True(t, strings.HasPrefix(public, "itg_"))
		events := connected.All()
		require.Len(t, events, 1)
		assert.Equal(t, []string{"notifications"}, events[0].Features)
		assert.Equal(t, "T1", events[0].ExternalAccountID)
		assert.False(t, events[0].Reconnected)
		s.tc.Table("audit_logs").AssertHas(map[string]any{"event": "integration.connected"})

		res := s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.status", "active").AssertJSONPath("data.name", "Team T1").
			AssertJSONPath("data.metadata.bot_user_id", "UB1").AssertJSONPath("data.features.0.status", "ready")
		assert.False(t, hasToken(string(res.Body)))
	})

	t.Run("lists installations and providers", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		s.install(t, slackGrant("T1", s.allScopes()))

		s.owner.GetJSON("/api/v1/integrations?filter[provider]=slack").AssertOK().
			AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.provider", "slack")
		s.owner.GetJSON("/api/v1/integration-providers").AssertOK().
			AssertJSONPath("data.0.name", "slack").AssertJSONPath("data.0.installable", true).
			AssertJSONPath("data.0.features.#", float64(3))
	})

	t.Run("lands straight back when the installation holds every scope", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		connected := tests.Record[appevents.IntegrationConnected](s.tc)

		authURL := start(t, s.owner, testkit.JSON{"features": []string{"channels"}, "return_to": "/d/channels"})

		assert.Equal(t, "/d/channels?connected="+public, authURL)
		require.Len(t, connected.All(), 1)
		assert.Equal(t, []string{"channels"}, connected.All()[0].Features)
	})

	t.Run("upgrades an installation missing scopes", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", []string{"chat:write"}))
		s.owner.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.features.0.status", "blocked_missing_scope")
		changed := tests.Record[appevents.IntegrationScopesChanged](s.tc)

		again := s.install(t, slackGrant("T1", s.allScopes()))

		assert.Equal(t, public, again)
		require.Len(t, changed.All(), 1)
		assert.NotEmpty(t, changed.All()[0].Added)
		s.tc.Table("integration_installations").AssertCount(1)
		s.tc.Table("audit_logs").AssertHas(map[string]any{"event": "integration.scopes_changed"})
	})

	t.Run("refuses a slack team another workspace installed", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		s.install(t, slackGrant("T1", s.allScopes()))
		other := s.tc.CreateUser()
		workspace, err := factories.Workspace(t.Context(), s.tc.App, other)
		require.NoError(t, err)
		client := s.tc.ActingAs(other).
			WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, workspace.Workspace.ID))

		callback, err := s.fake.Approve(start(t, client, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)

		assert.Equal(t, "integration_account_taken", follow(t, client, callback).Query().Get("error"))
	})

	t.Run("comes back without a row when the admin cancels", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		callback, err := s.fake.Deny(start(t, s.owner, testkit.JSON{"return_to": "/d/x?tab=1"}))
		require.NoError(t, err)
		landing := follow(t, s.owner, callback)

		assert.Equal(t, "/d/x", landing.Path)
		assert.Equal(t, "1", landing.Query().Get("tab"))
		assert.Equal(t, "denied", landing.Query().Get("error"))
		s.tc.Table("integration_installations").AssertCount(0)
	})

	t.Run("refuses an enterprise install and gives its token back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		grant := slackGrant("T1", s.allScopes())
		grant.Raw["is_enterprise_install"] = true

		callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), grant)
		require.NoError(t, err)

		assert.Equal(t, "enterprise_install_unsupported", follow(t, s.owner, callback).Query().Get("error"))
		assert.Equal(t, []string{grant.Access.Value}, s.fake.Revoked())
		s.tc.Table("integration_installations").AssertCount(0)
	})

	t.Run("reports a grant without an app token", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), userGrant("T1", "U1"))
		require.NoError(t, err)

		assert.Equal(t, "integration_provider_error", follow(t, s.owner, callback).Query().Get("error"))
	})
}

func TestInstallationCallback(t *testing.T) {
	t.Parallel()

	t.Run("refuses a callback used twice", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		follow(t, s.owner, callback)

		parsed, _ := url.Parse(callback)
		s.owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("refuses a callback opened in another browser", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		parsed, _ := url.Parse(callback)

		s.tc.In(s.tc.Owner).Send(fhttp.MethodGet, parsed.RequestURI(), nil).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		s.tc.Table("integration_installations").AssertCount(0)
	})

	t.Run("refuses a teammate who no longer manages integrations", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		admin, member := s.join(t, enums.RoleAdmin)
		callback, err := s.fake.Approve(start(t, admin, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		s.owner.PatchJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, member.ID),
			testkit.JSON{"role": "member"}).AssertOK()

		assert.Equal(t, "forbidden", follow(t, admin, callback).Query().Get("error"))
	})
}

func TestInstallationRequests(t *testing.T) {
	t.Parallel()

	t.Run("refuses members, tokens and bad input", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		member, _ := s.join(t, enums.RoleMember)

		member.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{}).AssertForbidden()
		s.tc.ActingAsToken(s.tc.Workspace.Owner, "settings:write").
			PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{}).AssertForbidden()
		s.owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{"return_to": "https://evil.test"}).
			AssertValidationError("return_to")
		s.owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{"return_to": "//evil.test"}).
			AssertValidationError("return_to")
		s.owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{"features": []string{"nope"}}).
			AssertValidationError("features")
		s.owner.PostJSON("/api/v1/integrations/linear/installations", testkit.JSON{}).AssertNotFound()
		s.owner.GetJSON("/api/v1/integrations/itg_nope").AssertNotFound()
		member.GetJSON("/api/v1/integrations").AssertOK()
	})

	t.Run("offers slack only from pasted tokens without the shared app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.In(tc.Owner)

		owner.GetJSON("/api/v1/integration-providers").AssertOK().AssertJSONPath("data.0.installable", false)
		owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{}).AssertNotFound()
	})
}
