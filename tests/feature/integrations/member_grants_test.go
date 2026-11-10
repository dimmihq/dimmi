package integrations_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
backChannel adds a channel connection backed by the installation and
returns its id.
*/
func (s slackCase) backChannel(t *testing.T, installation ids.UUID) ids.UUID {
	t.Helper()

	inbox, err := factories.DefaultInbox(t.Context(), s.tc.App, s.tc.Workspace.Workspace)
	require.NoError(t, err)

	var id ids.UUID
	err = s.tc.Operator().QueryRow(t.Context(), `
		INSERT INTO channel_connections (workspace_id, inbox_id, type, name, integration_installation_id)
		VALUES ($1, $2, 'slack', 'Slack', $3) RETURNING id`,
		s.tc.Workspace.Workspace.ID, inbox.Inbox.ID, installation).Scan(&id)
	require.NoError(t, err)

	return id
}

func TestMemberGrants(t *testing.T) {
	t.Parallel()

	t.Run("lets any teammate connect their own account on the installation's team", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		channel := s.backChannel(t, s.id(t, public))
		member, row := s.join(t, enums.RoleMember)

		res := member.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()
		parsed, err := url.Parse(res.String("data.authorize_url"))
		require.NoError(t, err)
		assert.Equal(t, "chat:write reactions:write", parsed.Query().Get("user_scope"))
		assert.Empty(t, parsed.Query().Get("scope"))
		assert.Equal(t, "T1", parsed.Query().Get("team"))

		callback, err := s.fake.Approve(res.String("data.authorize_url"), userGrant("T1", "U1"))
		require.NoError(t, err)
		landing := follow(t, member, callback)

		assert.Equal(t, public, landing.Query().Get("connected"))
		s.tc.Table("integration_member_grants").AssertHas(map[string]any{
			"member_id": row.ID, "external_user_id": "U1", "status": "active",
		})
		s.tc.Table("channel_member_links").AssertHas(map[string]any{
			"channel_connection_id": channel, "workspace_member_id": row.ID, "external_user_id": "T1:U1",
			"source": "oauth",
		})
		show := member.GetJSON("/api/v1/integrations/"+public).AssertOK().
			AssertJSONPath("data.member_grant_count", float64(1)).
			AssertJSONPath("data.own_grant.external_user_id", "U1")
		assert.Contains(t, string(show.Body), models.PublicID(models.PrefixChannelConnection, channel))
		assert.False(t, hasToken(string(show.Body)))
	})

	t.Run("replaces a teammate's older slack user and anyone else's claim on it", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		channel := s.backChannel(t, s.id(t, public))
		member, row := s.join(t, enums.RoleMember)

		s.grantAs(t, s.owner, public, userGrant("T1", "U2"))
		s.grantAs(t, member, public, userGrant("T1", "U1"))
		s.grantAs(t, member, public, userGrant("T1", "U2"))

		s.tc.Table("integration_member_grants").AssertCount(1)
		s.tc.Table("integration_member_grants").AssertHas(map[string]any{"member_id": row.ID, "external_user_id": "U2"})
		s.tc.Table("channel_member_links").AssertCount(1, testkit.Row{"channel_connection_id": channel})
		s.tc.Table("channel_member_links").AssertHas(testkit.Row{
			"channel_connection_id": channel, "workspace_member_id": row.ID, "external_user_id": "T1:U2",
		})
		s.tc.Table("audit_logs").AssertHas(map[string]any{"event": "integration.member_grant_replaced"})
	})

	t.Run("refuses a grant for another slack team and gives its token back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		grant := userGrant("T9", "U1")

		landing := s.grantAs(t, s.owner, public, grant)

		assert.Equal(t, "integration_account_mismatch", landing.Query().Get("error"))
		assert.Contains(t, s.fake.Revoked(), grant.UserAccess.Value)
		s.tc.Table("integration_member_grants").AssertCount(0)
	})

	t.Run("refuses a grant finished in another teammate's browser", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		member, _ := s.join(t, enums.RoleMember)
		res := s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()
		callback, err := s.fake.Approve(res.String("data.authorize_url"), userGrant("T1", "U1"))
		require.NoError(t, err)
		parsed, _ := url.Parse(callback)

		member.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusUnprocessableEntity)
		s.tc.Table("integration_member_grants").AssertCount(0)
	})

	t.Run("refuses a grant on a broken installation", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.quietInstall(t, s.expiring("xoxe-dead"))
		_, _ = s.tc.App.Actions.Integrations.Tokens.Bot(s.scoped(t), s.id(t, public))

		s.owner.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertStatus(409)
		s.owner.PostJSON("/api/v1/integrations/itg_nope/member-grants", testkit.JSON{}).AssertNotFound()
	})

	t.Run("lets a teammate take back their own grant, and admins anyone's", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		s.backChannel(t, s.id(t, public))
		member, row := s.join(t, enums.RoleMember)
		other, _ := s.join(t, enums.RoleMember)
		grant := userGrant("T1", "U1")
		s.grantAs(t, member, public, grant)
		grantID := s.grantOf(t, row.ID)

		other.Delete("/api/v1/integrations/" + public + "/member-grants/" + grantID).AssertForbidden()
		member.Delete("/api/v1/integrations/" + public + "/member-grants/" + grantID).AssertNoContent()

		assert.Equal(t, []string{grant.UserAccess.Value}, s.fake.Revoked())
		s.tc.Table("integration_member_grants").AssertHas(map[string]any{"status": "revoked"})
		s.tc.Table("channel_member_links").AssertCount(0)

		s.grantAs(t, member, public, userGrant("T1", "U1"))
		s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/" + grantID).AssertNoContent()
		s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/itu_nope").AssertNotFound()
		s.owner.Delete("/api/v1/integrations/" + public + "/member-grants/" +
			models.PublicID(models.PrefixIntegrationMemberGrant, ids.New())).AssertNotFound()
	})

	t.Run("takes a departed teammate's grants back", func(t *testing.T) {
		t.Parallel()
		s := newSlackCase(t)
		public := s.install(t, slackGrant("T1", s.allScopes()))
		member, row := s.join(t, enums.RoleMember)
		grant := userGrant("T1", "U1")
		s.grantAs(t, member, public, grant)

		s.owner.PatchJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, row.ID),
			testkit.JSON{"status": "deactivated"}).AssertOK()

		s.tc.Table("integration_member_grants").AssertCount(0)
		assert.Contains(t, s.fake.Revoked(), grant.UserAccess.Value)
	})
}

/*
grantOf returns the public id of the member's grant.
*/
func (s slackCase) grantOf(t *testing.T, member ids.UUID) string {
	t.Helper()

	var id ids.UUID
	err := s.tc.Operator().Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM integration_member_grants WHERE member_id = $1`, member).Scan(&id)
	})
	require.NoError(t, err)

	return models.PublicID(models.PrefixIntegrationMemberGrant, id)
}
