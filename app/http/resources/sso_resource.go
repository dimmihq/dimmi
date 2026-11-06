/*
|--------------------------------------------------------------------------
| Single Sign-On Resources
|--------------------------------------------------------------------------
|
| A connection as its owners see it: its settings, where it stands, what
| the identity provider needs to know about this side, and never its
| secret. A domain shows the record that proves it, so an owner can copy
| it straight into their DNS.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SSOConnection is one connection to an identity provider. SP holds what
the identity provider is told about this side.
*/
type SSOConnection struct {
	ID             string          `json:"id"`
	Object         string          `json:"object"`
	Protocol       string          `json:"protocol"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	Config         sso.Config      `json:"config"`
	HasSecret      bool            `json:"has_secret"`
	JITEnabled     bool            `json:"jit_enabled"`
	DefaultRole    string          `json:"default_role"`
	DefaultTeamIDs []string        `json:"default_team_ids"`
	GroupRoleMap   []GroupRole     `json:"group_role_map"`
	Enforced       bool            `json:"enforced"`
	TestedAt       *time.Time      `json:"tested_at"`
	LastLoginAt    *time.Time      `json:"last_login_at"`
	MetadataError  *string         `json:"metadata_error"`
	LastError      json.RawMessage `json:"last_error"`
	SP             ServiceProvider `json:"sp"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

/*
GroupRole gives an identity provider group a role.
*/
type GroupRole struct {
	Group        string  `json:"group"`
	Role         string  `json:"role,omitempty"`
	CustomRoleID *string `json:"custom_role_id,omitempty"`
}

/*
ServiceProvider is this side of a connection: for SAML its entity id,
the address answers are posted to and its metadata; for OpenID Connect
the address sign-ins come back to.
*/
type ServiceProvider struct {
	EntityID    string `json:"entity_id,omitempty"`
	ACSURL      string `json:"acs_url,omitempty"`
	MetadataURL string `json:"metadata_url,omitempty"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

/*
OrganizationDomain is a domain a company proves it owns, with the TXT
record that proves it.
*/
type OrganizationDomain struct {
	ID              string     `json:"id"`
	Object          string     `json:"object"`
	Domain          string     `json:"domain"`
	Status          string     `json:"status"`
	RecordName      string     `json:"record_name"`
	RecordValue     string     `json:"record_value"`
	VerifiedVia     *string    `json:"verified_via"`
	VerifiedAt      *time.Time `json:"verified_at"`
	LastCheckedAt   *time.Time `json:"last_checked_at"`
	SSOConnectionID *string    `json:"sso_connection_id"`
	CreatedAt       time.Time  `json:"created_at"`
}

/*
DomainCheck is what checking a domain found.
*/
type DomainCheck struct {
	Object       string             `json:"object"`
	Status       string             `json:"status"`
	FoundRecords []string           `json:"found_records"`
	Domain       OrganizationDomain `json:"domain"`
}

/*
NewSSOConnection shapes a connection, with this side's addresses.
*/
func NewSSOConnection(c sso.Connection, links appauth.Links) SSOConnection {
	row := c.Connection
	out := SSOConnection{
		ID: models.PublicID(models.PrefixSSOConnection, row.ID), Object: "sso_connection", Protocol: row.Protocol,
		Name: row.Name, Status: row.Status, Config: c.Config, HasSecret: len(row.SecretEncrypted) > 0,
		JITEnabled: row.JitEnabled, DefaultRole: row.DefaultRole,
		DefaultTeamIDs: publicIDs(models.PrefixTeam, row.DefaultTeamIds), GroupRoleMap: []GroupRole{},
		Enforced: row.Enforced, TestedAt: row.TestedAt, LastLoginAt: row.LastLoginAt,
		MetadataError: row.MetadataError, LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}

	for _, g := range c.GroupRoleMap {
		out.GroupRoleMap = append(out.GroupRoleMap, GroupRole{
			Group: g.Group, Role: g.Role, CustomRoleID: publicIDOf(models.PrefixRole, g.CustomRoleID),
		})
	}

	if row.Protocol == sso.ProtocolOIDC {
		out.SP = ServiceProvider{RedirectURI: links.SSOCallback(row.ID)}

		return out
	}

	out.SP = ServiceProvider{
		EntityID: links.SAMLMetadata(row.ID), ACSURL: links.SAMLACS(row.ID), MetadataURL: links.SAMLMetadata(row.ID),
	}

	return out
}

/*
NewOrganizationDomain shapes a domain with its proof.
*/
func NewOrganizationDomain(d models.OrganizationDomain) OrganizationDomain {
	return OrganizationDomain{
		ID: models.PublicID(models.PrefixOrganizationDomain, d.ID), Object: "organization_domain", Domain: d.Domain,
		Status: d.Status, RecordName: sso.RecordName(d.Domain), RecordValue: d.VerificationToken,
		VerifiedVia: d.VerifiedVia, VerifiedAt: d.VerifiedAt, LastCheckedAt: d.LastCheckedAt,
		SSOConnectionID: publicIDOf(models.PrefixSSOConnection, d.SsoConnectionID), CreatedAt: d.CreatedAt,
	}
}

/*
publicIDOf returns the public id of the row, nil for none.
*/
func publicIDOf(prefix models.Prefix, id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := models.PublicID(prefix, *id)

	return &public
}
