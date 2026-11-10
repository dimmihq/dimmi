package integrations_test

import (
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestFailures(t *testing.T) {
	t.Parallel()

	t.Run("reports a broken installations table everywhere", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		id := s.id(t, public)
		acts := s.tc.App.Actions.Integrations
		s.tc.BreakTable("integration_installations")

		_, err := acts.Tokens.Bot(s.scoped(t), id)
		require.Error(t, err)
		require.NotErrorIs(t, err, integrations.ErrIntegrationDisconnected)
		require.Error(t, acts.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", Bot: true,
		}))
		require.Error(t, acts.CheckHealth.Due(t.Context()))
		require.Error(t, acts.CheckHealth.Handle(t.Context(), s.tc.Workspace.Workspace.ID, id))
		s.owner.GetJSON("/api/v1/integrations").AssertStatus(fhttp.StatusInternalServerError)
		s.owner.GetJSON("/api/v1/integrations/" + public).AssertStatus(fhttp.StatusInternalServerError)
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(fhttp.StatusInternalServerError)
		s.owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{}).
			AssertStatus(fhttp.StatusInternalServerError)
		s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).
			AssertStatus(fhttp.StatusInternalServerError)
		s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/" +
			models.PublicID(models.PrefixIntegrationMemberGrant, ids.New())).
			AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("reports a broken grants table", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		acts := s.tc.App.Actions.Integrations
		s.tc.BreakTable("integration_member_grants")

		_, _, err := acts.Tokens.User(s.scoped(t), s.id(t, public), s.tc.Workspace.Owner.ID)
		require.Error(t, err)
		require.Error(t, acts.RevokeDeparted.Handle(t.Context(), s.tc.Workspace.Workspace.ID, s.tc.Workspace.Owner.ID))
		require.Error(t, acts.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", UserIDs: []string{"U1"},
		}))
		s.owner.GetJSON("/api/v1/integrations").AssertStatus(fhttp.StatusInternalServerError)
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(fhttp.StatusInternalServerError)
		s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/" +
			models.PublicID(models.PrefixIntegrationMemberGrant, ids.New())).
			AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("reports a broken channels table", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		s.install(t, slackGrant("T1", s.allScopes()))
		s.tc.BreakTable("channel_connections")

		s.owner.GetJSON("/api/v1/integrations").AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("needs a workspace id in the context", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)

		_, err := s.tc.App.Actions.Integrations.Tokens.Bot(tenancy.With(t.Context(), "nope"), ids.New())
		require.Error(t, err)
	})

	t.Run("stores nothing when the installation cannot be written", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		s.install(t, slackGrant("T1", []string{"chat:write"}))
		s.tc.FailWrites("integration_installations", "INSERT", "UPDATE")

		for _, team := range []string{"T1", "T2"} {
			callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), slackGrant(team, s.allScopes()))
			require.NoError(t, err)
			parsed, _ := url.Parse(callback)
			s.owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusInternalServerError)
		}
	})

	t.Run("undoes an install whose audit entry cannot be written", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		s.tc.FailWrites("audit_logs", "INSERT")

		callback, err := s.fake.Approve(start(t, s.owner, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		parsed, _ := url.Parse(callback)

		s.owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusInternalServerError)
		s.tc.Table("integration_installations").AssertCount(0)
	})

	t.Run("keeps the grants when their writes fail", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.backChannel(t, s.id(t, public))
		member, row := s.join(t, enums.RoleMember)
		s.grantAs(t, member, public, userGrant("T1", "U1"))
		acts := s.tc.App.Actions.Integrations

		for _, fail := range [][]string{
			{"channel_member_links", "INSERT"},
			{"channel_member_links", "DELETE"},
			{"integration_member_grants", "INSERT"},
			{"integration_member_grants", "DELETE"},
			{"integration_member_grants", "UPDATE"},
			{"audit_logs", "INSERT"},
		} {
			s.tc.FailWrites(fail[0], fail[1])
		}

		res := s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()
		callback, err := s.fake.Approve(res.String("data.authorize_url"), userGrant("T1", "U2"))
		require.NoError(t, err)
		parsed, _ := url.Parse(callback)
		s.owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusInternalServerError)
		require.Error(t, acts.RevokeDeparted.Handle(t.Context(), s.tc.Workspace.Workspace.ID, row.ID))
		require.Error(t, acts.ApplyRevocation.Handle(t.Context(), integrations.Revocation{
			Provider: "slack", AppID: "A1", AccountID: "T1", UserIDs: []string{"U1"},
		}))
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("reports a renewal or a check that cannot be stored", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		id := s.id(t, s.quietInstall(t, s.expiring("xoxe-1")))
		s.fake.AcceptRefresh("xoxe-1", slackGrant("T1", nil))
		stubSlack(s, authTest("T1"), failure("invalid_auth"), failure("ratelimited"))
		s.tc.FailWrites("integration_installations", "UPDATE")
		acts := s.tc.App.Actions.Integrations

		_, err := acts.Tokens.Bot(s.scoped(t), id)
		require.Error(t, err)
		for range 3 {
			require.Error(t, acts.CheckHealth.Handle(t.Context(), s.tc.Workspace.Workspace.ID, id))
		}
	})

	t.Run("reads the token back when another process holds the lock", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.quietInstall(t, s.expiring("xoxe-1"))
		grant := userGrant("T1", "U1")
		grant.UserAccess.Expiry = s.tc.Clock.Now()
		grant.UserRefresh = &oauthclient.Token{Value: "xoxe-u"}
		s.grantAs(t, s.owner, public, grant)
		id, owner := s.id(t, public), s.tc.Workspace.Owner.ID
		for _, name := range []string{id.String(), id.String() + ":" + owner.String()} {
			_, release, err := s.tc.App.Cache.Lock(t.Context(), "integrations:refresh:"+name)
			require.NoError(t, err)
			t.Cleanup(release)
		}
		tokens := s.tc.App.Actions.Integrations.Tokens

		var wg sync.WaitGroup
		var botErr, userErr error
		wg.Go(func() { _, botErr = tokens.Bot(s.scoped(t), id) })
		wg.Go(func() { _, _, userErr = tokens.User(s.scoped(t), id, owner) })
		wg.Wait()

		require.NoError(t, botErr)
		require.NoError(t, userErr)
		assert.Empty(t, s.fake.Refreshed())
	})
}

func TestCallbackEdges(t *testing.T) {
	t.Parallel()

	t.Run("reports an error slack sent back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		parsed, err := url.Parse(start(t, s.owner, testkit.JSON{}))
		require.NoError(t, err)

		landing := follow(t, s.owner, "/integrations/slack/callback?"+url.Values{
			"state": {parsed.Query().Get("state")}, "error": {"server_error"},
		}.Encode())

		assert.Equal(t, "integration_provider_error", landing.Query().Get("error"))
	})

	t.Run("refuses an admin removed before coming back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		admin, member := s.join(t, enums.RoleAdmin)
		callback, err := s.fake.Approve(start(t, admin, testkit.JSON{}), slackGrant("T1", s.allScopes()))
		require.NoError(t, err)
		s.owner.PatchJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, member.ID),
			testkit.JSON{"status": "deactivated"}).AssertOK()
		parsed, _ := url.Parse(callback)

		admin.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertForbidden()
	})

	t.Run("reports a grant whose installation went away or that holds no user token", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))

		assert.Equal(t, "integration_provider_error",
			s.grantAs(t, s.owner, public, slackGrant("T1", nil)).Query().Get("error"))

		res := s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()
		callback, err := s.fake.Approve(res.String("data.authorize_url"), userGrant("T1", "U1"))
		require.NoError(t, err)
		s.owner.Delete("/api/v1/integrations/" + public).AssertStatus(202)

		assert.Equal(t, "not_found", follow(t, s.owner, callback).Query().Get("error"))
	})

	t.Run("refuses a grant for another team on a pasted-token installation", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		stubSlack(s, authTest("T9"))
		row, err := s.tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), s.manual("xoxb-custom"))
		require.NoError(t, err)
		s.tc.Table("integration_installations").AssertCount(1)

		landing := s.grantAs(t, s.owner, models.PublicID(models.PrefixIntegrationInstallation, row.ID),
			userGrant("T1", "U1"))
		assert.Equal(t, "integration_account_mismatch", landing.Query().Get("error"))
		s.owner.GetJSON("/integrations/linear/callback").AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}
