/*
|--------------------------------------------------------------------------
| Public ID Prefixes
|--------------------------------------------------------------------------
|
| Rows are keyed by UUIDv7, but clients only ever see public ids such as
| usr_01jc...: a prefix naming the kind of thing, then the UUID in
| Crockford base32. The prefix makes a pasted id readable and lets the
| server refuse an id of the wrong kind before any query, as 404.
|
| Every prefix in use is listed here, together with the ones reserved
| for tables that do not exist yet, so a new table can never take a
| prefix another one owns or will own. Registering them all is part of
| boot; a clash stops it.
|
*/

package models

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Prefix names the kind of row a public id points at.
*/
type Prefix string

/*
The prefixes of the account tables and of the tables other parts of the
product read here.
*/
const (
	PrefixUser               Prefix = "usr"
	PrefixWorkspace          Prefix = "ws"
	PrefixOrganization       Prefix = "org"
	PrefixMember             Prefix = "mem"
	PrefixTeam               Prefix = "team"
	PrefixInvitation         Prefix = "inv"
	PrefixSession            Prefix = "ses"
	PrefixPersonalToken      Prefix = "tok"
	PrefixAuditLog           Prefix = "aud"
	PrefixOAuthIdentity      Prefix = "idn"
	PrefixSecurityEvent      Prefix = "sev"
	PrefixOneTimeToken       Prefix = "ott"
	PrefixPasswordReset      Prefix = "prs"
	PrefixAuthFactor         Prefix = "fac"
	PrefixOAuthGrant         Prefix = "ogr"
	PrefixOAuthClient        Prefix = "ocl"
	PrefixWorkspaceDomain    Prefix = "dom"
	PrefixSSOConnection      Prefix = "ssoc"
	PrefixOrganizationDomain Prefix = "odom"
	PrefixRole               Prefix = "rol"
	PrefixUserDevice         Prefix = "dev"
)

/*
PublicID returns the public id of a row of the kind.
*/
func PublicID(prefix Prefix, id ids.UUID) string {
	return ids.Encode(string(prefix), id)
}

/*
RegisterPublicIDs adds every prefix of the catalog to the registry, the
reserved ones included, and reports every prefix already taken.
*/
func RegisterPublicIDs(registry *ids.Registry) error {
	var errs []error

	for _, entry := range publicIDCatalog() {
		errs = append(errs, registry.Register(string(entry.prefix), entry.table))
	}

	return errors.Join(errs...)
}

/*
publicIDCatalog lists every prefix with the table it names. Tables of
other product areas appear as they are built; a reserved prefix names
the table that will own it.
*/
func publicIDCatalog() []struct {
	prefix Prefix
	table  string
} {
	return []struct {
		prefix Prefix
		table  string
	}{
		{PrefixUser, "users"},
		{PrefixWorkspace, "workspaces"},
		{PrefixOrganization, "organizations"},
		{PrefixMember, "workspace_members"},
		{PrefixTeam, "teams"},
		{PrefixInvitation, "invitations"},
		{PrefixSession, "sessions"},
		{PrefixPersonalToken, "personal_access_tokens"},
		{PrefixAuditLog, "audit_logs"},
		{PrefixOAuthIdentity, "oauth_identities"},
		{PrefixSecurityEvent, "security_events"},
		{PrefixOneTimeToken, "one_time_tokens"},
		{PrefixPasswordReset, "password_reset_tokens"},
		{PrefixAuthFactor, "user_auth_factors"},
		{PrefixOAuthGrant, "oauth_grants"},
		{PrefixOAuthClient, "oauth_clients"},
		{PrefixWorkspaceDomain, "reserved: workspace_domains"},
		{PrefixSSOConnection, "reserved: sso_connections"},
		{PrefixOrganizationDomain, "reserved: organization_domains"},
		{PrefixRole, "reserved: roles"},
		{PrefixUserDevice, "reserved: user_devices"},
	}
}
