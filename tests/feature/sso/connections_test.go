package sso_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestConnections(t *testing.T) {
	t.Parallel()

	t.Run("an owner creates a saml draft with its own certificate and never sees the key", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)

		res := in(tc, tc.Owner).PostJSON(connectionsPath, testkit.JSON{
			"protocol": "saml", "name": "Okta", "config": testkit.JSON{"attribute_map": testkit.JSON{"email": "mail"}},
		}).AssertCreated()

		res.AssertJSONPath("data.object", "sso_connection").
			AssertJSONPath("data.status", "draft").
			AssertJSONPath("data.enforced", false).
			AssertJSONPath("data.has_secret", true).
			AssertJSONPath("data.config.attribute_map.email", "mail")
		id := res.String("data.id")
		assert.Equal(t, "https://app.example.com/auth/saml/"+id+"/metadata", res.String("data.sp.entity_id"))
		assert.Equal(t, "https://app.example.com/auth/saml/"+id+"/acs", res.String("data.sp.acs_url"))
		assert.Contains(t, res.String("data.config.sp_certificate"), "BEGIN CERTIFICATE")
		assert.NotContains(t, string(res.Body), "PRIVATE KEY")

		in(tc, tc.Owner).GetJSON(connectionsPath).AssertOK().AssertJSONPath("data.0.id", id)
		in(tc, tc.Owner).GetJSON(connectionsPath+"/"+id).AssertOK().AssertJSONPath("data.name", "Okta")
	})

	t.Run("an openid connection needs its issuer, client and secret, and never shows the secret", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		owner := in(tc, tc.Owner)

		owner.PostJSON(connectionsPath, testkit.JSON{
			"protocol": "oidc", "name": "Entra", "config": testkit.JSON{"issuer": "https://login.example/v2"},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_saml_metadata")

		res := owner.PostJSON(connectionsPath, testkit.JSON{
			"protocol": "oidc", "name": "Entra", "client_secret": "s3cret-value",
			"config": testkit.JSON{"issuer": "https://login.example/v2", "client_id": "abc"},
		}).AssertCreated()
		assert.NotContains(t, string(res.Body), "s3cret-value")
		assert.Contains(t, res.String("data.sp.redirect_uri"), "/auth/sso/ssoc_")
	})

	t.Run("a certificate that does not read is refused", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)

		in(tc, tc.Owner).PostJSON(connectionsPath, testkit.JSON{
			"protocol": "saml", "name": "Okta", "config": testkit.JSON{"idp_certificates": []string{"nonsense"}},
		}).AssertStatus(422)
	})

	t.Run("only owners shape connections, and tokens never", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		admin := join(t, tc, enums.RoleAdmin)

		in(tc, admin).GetJSON(connectionsPath).AssertForbidden()
		in(tc, admin).PostJSON(connectionsPath, testkit.JSON{"protocol": "saml", "name": "x"}).AssertForbidden()
		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON(connectionsPath).AssertForbidden()
	})

	t.Run("activation needs a test, a verified routed domain and https", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)

		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).
			AssertStatus(422).AssertJSONPath("code", "invalid_transition")

		markTested(t, tc, id)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).
			AssertStatus(422).AssertJSONPath("code", "no_verified_domain")

		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK().
			AssertJSONPath("data.status", "active")

		w, err := models.New(tc.Operator()).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		assert.Len(t, w.Security().SSOConnectionIDs, 1)
		in(tc, tc.Owner).GetJSON("/api/v1/workspace/access-policy").AssertOK().
			AssertJSONPath("data.available_sign_in_methods.#(==\"sso\")", "sso")
	})

	t.Run("plain http installs cannot activate single sign-on", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		zone := tc.App.DNS.Fake()
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		in(tc, tc.Owner).PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()

		in(tc, tc.Owner).PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).
			AssertStatus(422).AssertJSONPath("code", "https_required")
	})

	t.Run("enforcing needs proof, binds the workspace, and blocks deletion", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()

		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"enforced": true}).
			AssertStatus(422).AssertJSONPath("code", "enforce_unproven")
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"enforced": true, "confirm_owners_can_sign_in": true}).
			AssertOK().AssertJSONPath("data.enforced", true)

		owner.GetJSON("/api/v1/workspace").AssertForbidden().AssertJSONPath("code", "sso_required")
		tc.ActingAsToken(tc.Workspace.Owner, "settings:read").GetJSON("/api/v1/workspace").AssertOK()

		code, out := tc.Command("sso:disable", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "Okta")
		owner.GetJSON("/api/v1/workspace").AssertOK()
		owner.GetJSON("/api/v1/audit-logs?filter[event]=sso_connection.updated").AssertOK().
			AssertJSONPath("data.0.actor.type", "system")

		code, out = tc.Command("sso:disable", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "No active connection")
	})

	t.Run("a stale test does not prove a connection can be enforced", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()
		tc.Clock.Advance(25 * time.Hour)

		in(tc, tc.Owner).PatchJSON(connectionsPath+"/"+id, testkit.JSON{
			"enforced": true, "confirm_owners_can_sign_in": true,
		}).AssertStatus(422).AssertJSONPath("code", "enforce_unproven")
	})

	t.Run("changing how it talks to the provider sends it back to draft and unenforces it", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()
		signedInThrough(t, tc, tc.Owner, id)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"enforced": true}).AssertOK()

		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{
			"config": testkit.JSON{
				"idp_entity_id": "http://idp.example/okta", "idp_sso_url": "https://idp.example/sso",
				"attribute_map": testkit.JSON{"email": "e"},
			},
		}).AssertOK().AssertJSONPath("data.status", "active")

		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{
			"config": testkit.JSON{"idp_entity_id": "http://idp.example/other"},
		}).AssertOK().AssertJSONPath("data.status", "draft").AssertJSONPath("data.enforced", false).
			AssertJSONPath("data.tested_at", nil)

		owner.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("an enforced connection cannot be deleted, a disabled one can", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()
		signedInThrough(t, tc, tc.Owner, id)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"enforced": true}).AssertOK()

		owner.Delete(connectionsPath+"/"+id).AssertStatus(409).AssertJSONPath("code", "connection_enforced")
		owner.Delete(domainsPath+"/"+domain).AssertStatus(409).AssertJSONPath("code", "domain_in_use")
	})

	t.Run("changes land in the audit log", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"name": "Okta prod"}).AssertOK()
		verifiedDomain(tc, zone)
		owner.Delete(connectionsPath + "/" + id).AssertNoContent()

		for _, event := range []string{
			"sso_connection.created", "sso_connection.updated", "sso_connection.deleted",
			"organization_domain.added", "organization_domain.verified",
		} {
			owner.GetJSON("/api/v1/audit-logs?filter[event]="+event).AssertOK().AssertJSONPath("data.0.event", event)
		}
	})

	t.Run("another organization's connection is not found", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		other, _ := secure(t)
		foreign := samlConnection(other)

		in(tc, tc.Owner).GetJSON(connectionsPath + "/" + foreign).AssertNotFound()
		in(tc, tc.Owner).PatchJSON(connectionsPath+"/"+foreign, testkit.JSON{"name": "x"}).AssertNotFound()
		in(tc, tc.Owner).Delete(connectionsPath + "/" + foreign).AssertNotFound()
	})

	t.Run("new members may start with one of the workspace's custom roles, never another's", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		other, _ := secure(t)
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		mine, err := factories.Role(t.Context(), tc.App, factories.RoleOf{
			Workspace: tc.Workspace.Workspace, Name: "Support", BaseRole: enums.RoleMember,
			Permissions: []string{"members:read"},
		})
		require.NoError(t, err)
		theirs, err := factories.Role(t.Context(), other.App, factories.RoleOf{
			Workspace: other.Workspace.Workspace, Name: "Support", BaseRole: enums.RoleMember,
			Permissions: []string{"members:read"},
		})
		require.NoError(t, err)
		public := models.PublicID(models.PrefixRole, mine.ID)

		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"default_custom_role_id": public}).AssertOK().
			AssertJSONPath("data.default_custom_role_id", public)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{
			"default_custom_role_id": models.PublicID(models.PrefixRole, theirs.ID),
		}).AssertNotFound()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"default_custom_role_id": nil}).AssertOK().
			AssertJSONPath("data.default_custom_role_id", nil)
	})
}
