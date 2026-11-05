/*
|--------------------------------------------------------------------------
| Workspace Security Settings
|--------------------------------------------------------------------------
|
| What a workspace asks of the people working in it, kept in its settings
| document under security. Every request decodes it once from the
| workspace it already loaded, so a change applies to the very next
| request and enforcing it costs no query. A document that cannot be read
| asks nothing, so a damaged setting can never lock a company out, and
| every key a change does not touch is kept as it was.
|
*/

package models

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
AllowedAddress is one entry of an address allowlist: a range in CIDR
form, or a single address, and what the workspace calls it.
*/
type AllowedAddress struct {
	CIDR  string `json:"cidr"`
	Label string `json:"label"`
}

/*
SecuritySettings is the security area of a workspace's settings.

AllowedSignInMethods lists the ways of signing in a session may have
used, empty for every way. GoogleAllowedDomains lists the Google hosted
domains a Google sign-in may come from, empty for any. SessionMaxAge and
SessionIdleTimeout are in minutes, nil for no limit. DashboardAllowlist
and APIAllowlist bind browsers and apps, and tokens, to address ranges,
empty for anywhere. SSOConnectionIDs and SSOEnforcedConnectionIDs mirror
the organization's active and enforced connections, so a rule can read
them without a query; only the actions that change connections write
them.
*/
type SecuritySettings struct {
	RequireTwoFactor         bool             `json:"require_two_factor"`
	AllowedSignInMethods     []string         `json:"allowed_sign_in_methods"`
	GoogleAllowedDomains     []string         `json:"google_allowed_domains"`
	SessionMaxAge            *int             `json:"session_max_age_minutes"`
	SessionIdleTimeout       *int             `json:"session_idle_timeout_minutes"`
	DashboardAllowlist       []AllowedAddress `json:"dashboard_ip_allowlist"`
	APIAllowlist             []AllowedAddress `json:"api_ip_allowlist"`
	SSOConnectionIDs         []ids.UUID       `json:"sso_connection_ids"`
	SSOEnforcedConnectionIDs []ids.UUID       `json:"sso_enforced_connection_ids"`
}

/*
Security decodes the security area of the workspace's settings. A
document that does not read asks nothing.
*/
func (w Workspace) Security() SecuritySettings {
	var document struct {
		Security SecuritySettings `json:"security"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &document) != nil {
		return SecuritySettings{}
	}

	return document.Security
}

/*
AccessPolicyChange holds the access rules to write. Keys holds each
setting's name and new value; settings left out stay as stored.
*/
type AccessPolicyChange map[string]any

/*
WithAccessPolicy returns the settings document with the change applied,
keeping every other key.
*/
func WithAccessPolicy(settings json.RawMessage, change AccessPolicyChange) (json.RawMessage, error) {
	return withSecurityKeys(settings, change)
}

/*
WithSSOConnections returns the settings document with the mirrors of the
organization's active and enforced connections replaced.
*/
func WithSSOConnections(settings json.RawMessage, active, enforced []ids.UUID) (json.RawMessage, error) {
	return withSecurityKeys(settings, map[string]any{
		"sso_connection_ids":          slices.Clone(active),
		"sso_enforced_connection_ids": slices.Clone(enforced),
	})
}

/*
withSecurityKeys returns the settings document with the keys of its
security area replaced, every other key kept byte for byte.
*/
func withSecurityKeys(settings json.RawMessage, keys map[string]any) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	security := map[string]json.RawMessage{}
	if raw, ok := values["security"]; ok {
		if err := json.Unmarshal(raw, &security); err != nil {
			return nil, fmt.Errorf("read workspace security settings: %w", err)
		}
	}

	for key, value := range keys {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("write workspace security setting %s: %w", key, err)
		}

		security[key] = raw
	}

	raw, err := json.Marshal(security)
	if err != nil {
		return nil, fmt.Errorf("write workspace security settings: %w", err)
	}

	values["security"] = raw

	return json.Marshal(values)
}

/*
ParseAllowedAddress parses one allowlist entry into a range, an address
without a mask becoming a range of its one address. Mapped IPv4 forms
are unmapped, so a list and a caller always compare the same way.
*/
func ParseAllowedAddress(cidr string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(cidr); err == nil {
		if addr := prefix.Addr(); addr.Is4In6() {
			prefix = netip.PrefixFrom(addr.Unmap(), max(prefix.Bits()-96, 0))
		}

		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(cidr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parse address %q: %w", cidr, err)
	}

	addr = addr.Unmap().WithZone("")

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

/*
AddressAllowed reports whether the list lets the address in: an empty
list lets everyone in, an entry that does not parse matches nothing, and
an address the request could not tell matches no entry.
*/
func AddressAllowed(list []AllowedAddress, addr netip.Addr) bool {
	if len(list) == 0 {
		return true
	}

	if !addr.IsValid() {
		return false
	}

	addr = addr.Unmap().WithZone("")

	return slices.ContainsFunc(list, func(entry AllowedAddress) bool {
		prefix, err := ParseAllowedAddress(entry.CIDR)

		return err == nil && prefix.Contains(addr)
	})
}
