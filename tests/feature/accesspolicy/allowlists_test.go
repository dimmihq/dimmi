package accesspolicy_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestAllowlists(t *testing.T) {
	t.Parallel()

	allow := func(cidrs ...string) []map[string]string {
		entries := make([]map[string]string, 0, len(cidrs))
		for _, cidr := range cidrs {
			entries = append(entries, map[string]string{"cidr": cidr, "label": "Office"})
		}

		return entries
	}

	t.Run("an address outside the dashboard list is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := from(in(tc, tc.Owner), "198.51.100.7")
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow("203.0.113.0/24")})

		owner.GetJSON("/api/v1/workspace").AssertForbidden().
			AssertJSONPath("code", "ip_not_allowed").
			AssertJSONPath("requirement", "ip_not_allowed").
			AssertJSONPath("action_url", nil).
			AssertJSONPath("ip", "198.51.100.7")
	})

	t.Run("an address inside the list works", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := from(in(tc, tc.Owner), "203.0.113.9")
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow("203.0.113.0/24")})

		owner.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("ipv6 ranges match and a mapped address compares as ipv4", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow("2001:db8::/32", "203.0.113.0/24")})

		from(owner, "2001:db8::1").GetJSON("/api/v1/workspace").AssertOK()
		from(owner, "2001:db9::1").GetJSON("/api/v1/workspace").AssertForbidden()
		from(owner, "::ffff:203.0.113.5").GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("mapped and bare forms parse the same", func(t *testing.T) {
		t.Parallel()

		mapped, err := models.ParseAllowedAddress("::ffff:203.0.113.0/120")
		require.NoError(t, err)
		assert.Equal(t, "203.0.113.0/24", mapped.String())

		single, err := models.ParseAllowedAddress("203.0.113.4")
		require.NoError(t, err)
		assert.Equal(t, "203.0.113.4/32", single.String())

		_, err = models.ParseAllowedAddress("nope")
		require.Error(t, err)

		list := []models.AllowedAddress{{CIDR: "nope"}}
		assert.False(t, models.AddressAllowed(list, netip.MustParseAddr("203.0.113.4")))
		assert.False(t, models.AddressAllowed([]models.AllowedAddress{{CIDR: "0.0.0.0/0"}}, netip.Addr{}))
		assert.True(t, models.AddressAllowed(nil, netip.Addr{}))
	})

	t.Run("a caller cannot exclude their own address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := from(in(tc, tc.Owner), "198.51.100.7")

		owner.PatchJSON(policyPath, testkit.JSON{"dashboard_ip_allowlist": allow("203.0.113.0/24")}).
			AssertStatus(422).
			AssertJSONPath("code", "ip_allowlist_excludes_caller").
			AssertJSONPath("ip", "198.51.100.7")
		owner.PatchJSON(policyPath, testkit.JSON{"dashboard_ip_allowlist": allow("198.51.100.7")}).AssertOK().
			AssertJSONPath("data.dashboard_ip_allowlist.0.cidr", "198.51.100.7/32").
			AssertJSONPath("data.dashboard_ip_allowlist.0.label", "Office")
	})

	t.Run("an entry that does not parse is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"api_ip_allowlist": allow("not an address")}).
			AssertValidationError("api_ip_allowlist")
		owner.PatchJSON(policyPath, testkit.JSON{"dashboard_ip_allowlist": allow("not an address")}).
			AssertValidationError("dashboard_ip_allowlist")
	})

	t.Run("a token from outside the api list is refused with its address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		token := tc.ActingAsToken(tc.Workspace.Owner, "settings:read", "members:read")
		ruled(t, tc, map[string]any{"api_ip_allowlist": allow("203.0.113.0/24")})

		res := from(token, "198.51.100.7").GetJSON("/api/v1/members").AssertForbidden().
			AssertJSONPath("code", "ip_not_allowed").AssertJSONPath("ip", "198.51.100.7")
		assert.Contains(t, res.String("message"), "198.51.100.7")
		assert.Contains(t, res.String("message"), "allowlist")

		from(token, "203.0.113.7").GetJSON("/api/v1/members").AssertOK()
	})

	t.Run("the dashboard list does not bind tokens and the api list does not bind browsers", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow("203.0.113.0/24")})

		from(token, "198.51.100.7").GetJSON("/api/v1/members").AssertOK()

		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": []string{}, "api_ip_allowlist": allow("203.0.113.0/24")})
		from(owner, "198.51.100.7").GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("a refused member can still reach their own account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := from(in(tc, tc.Owner), "198.51.100.7")
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow("203.0.113.0/24")})

		owner.GetJSON("/api/v1/me").AssertOK()
		owner.GetJSON("/api/v1/sessions").AssertOK()
		owner.GetJSON("/api/v1/user/memberships").AssertOK()
	})

	t.Run("saving an api list reports live tokens it would stop", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")
		from(token, "198.51.100.7").GetJSON("/api/v1/members").AssertOK()
		tc.ActingAsToken(tc.Workspace.Owner, "members:read")
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"api_ip_allowlist": allow("203.0.113.0/24")}).AssertOK().
			AssertJSONPath("data.tokens_outside", 1.0)
		owner.PatchJSON(policyPath, testkit.JSON{"api_ip_allowlist": allow("198.51.100.0/24")}).AssertOK().
			AssertJSONPath("data.tokens_outside", 0.0)
		owner.GetJSON(policyPath).AssertOK().AssertJSONPath("data.tokens_outside", 0.0)
	})
}
