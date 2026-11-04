/*
|--------------------------------------------------------------------------
| Single Sign-On Rules
|--------------------------------------------------------------------------
|
| What every connection and domain action shares. Connections and domains
| belong to the organization behind the workspace in play, and only its
| owners shape them: a wrong setting locks everyone out. Whenever the set
| of active or enforced connections changes, the rules of every workspace
| of the organization are rewritten in the same transaction, so the very
| next request is judged by them without a query.
|
*/

package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
managePermission is the permission to shape a company's sign-in.
*/
const managePermission = "sso:manage"

/*
Protocols a connection speaks.
*/
const (
	ProtocolSAML = "saml"
	ProtocolOIDC = "oidc"
)

/*
Statuses a connection moves through.
*/
const (
	StatusDraft    = "draft"
	StatusTested   = "tested"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

/*
Limits of an organization.
*/
const (
	maxConnections = 10
	maxDomains     = 20
)

/*
scopeOrganization is the kind of the data key scope a connection's secret
is sealed under.
*/
const scopeOrganization = "organization"

/*
AttributeMap names the identity provider's attributes that carry each
part of a person.
*/
type AttributeMap struct {
	Email       string `json:"email,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Groups      string `json:"groups,omitempty"`
}

/*
Config is a connection's settings. The identity provider fields belong to
SAML, the issuer, client and scopes to OpenID Connect; SPCertificate is
the service provider's own certificate, made with the connection.
*/
type Config struct {
	IDPEntityID     string       `json:"idp_entity_id,omitempty"`
	IDPSSOURL       string       `json:"idp_sso_url,omitempty"`
	IDPCertificates []string     `json:"idp_certificates,omitempty"`
	IDPMetadataURL  string       `json:"idp_metadata_url,omitempty"`
	NameIDFormat    string       `json:"name_id_format,omitempty"`
	SPCertificate   string       `json:"sp_certificate,omitempty"`
	Issuer          string       `json:"issuer,omitempty"`
	ClientID        string       `json:"client_id,omitempty"`
	Scopes          []string     `json:"scopes,omitempty"`
	AttributeMap    AttributeMap `json:"attribute_map"`
}

/*
GroupRole gives the members of an identity provider group a role, built
in or custom.
*/
type GroupRole struct {
	Group        string    `json:"group"`
	Role         string    `json:"role,omitempty"`
	CustomRoleID *ids.UUID `json:"custom_role_id,omitempty"`
}

/*
Connection is a connection with its decoded settings.
*/
type Connection struct {
	Connection   models.SSOConnection
	Config       Config
	GroupRoleMap []GroupRole
}

/*
Secret is what a connection keeps sealed: the service provider's private
key for SAML, the client secret for OpenID Connect.
*/
type Secret struct {
	PrivateKey   string `json:"private_key,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

/*
decode reads a connection's stored settings.
*/
func decode(row models.SSOConnection) (Connection, error) {
	c := Connection{Connection: row}

	if err := unmarshal(row.Config, &c.Config); err != nil {
		return c, fmt.Errorf("read connection config: %w", err)
	}

	if err := unmarshal(row.GroupRoleMap, &c.GroupRoleMap); err != nil {
		return c, fmt.Errorf("read connection group roles: %w", err)
	}

	return c, nil
}

/*
seal seals the connection's secret for its organization, bound to the
connection.
*/
func seal(ctx context.Context, keys encryption.KeyManager, row models.SSOConnection, s Secret) ([]byte, error) {
	sealed, err := encryption.Seal(ctx, keys, slot(row.OrganizationID, row.ID), s)
	if err != nil {
		return nil, fmt.Errorf("seal connection secret: %w", err)
	}

	return sealed, nil
}

/*
OpenSecret opens the connection's sealed secret.
*/
func OpenSecret(ctx context.Context, keys encryption.KeyManager, row models.SSOConnection) (Secret, error) {
	if len(row.SecretEncrypted) == 0 {
		return Secret{}, nil
	}

	secret, err := encryption.Open[Secret](ctx, keys, row.SecretEncrypted, slot(row.OrganizationID, row.ID).AAD)
	if err != nil {
		return Secret{}, fmt.Errorf("open connection secret: %w", err)
	}

	return secret, nil
}

/*
slot is where a connection's secret is sealed: its organization's keys,
bound to the connection's row.
*/
func slot(organizationID, connectionID ids.UUID) encryption.Slot {
	return encryption.Slot{
		Scope: encryption.Scope{Kind: scopeOrganization, ID: organizationID.String()},
		AAD:   []byte("sso_connections.secret_encrypted:" + connectionID.String()),
	}
}

/*
lockConnection returns the organization's connection and holds its row,
or exceptions.ErrNotFound.
*/
func lockConnection(
	ctx context.Context, q *models.Queries, organizationID, id ids.UUID,
) (models.SSOConnection, error) {
	row, err := q.LockSSOConnection(ctx, models.LockSSOConnectionParams{OrganizationID: organizationID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("lock connection: %w", err)
	}

	return row, nil
}

/*
Mirror rewrites the rules of every workspace of the organization with
its active and enforced connections.
*/
func Mirror(ctx context.Context, q *models.Queries, organizationID ids.UUID, now time.Time) error {
	rows, err := q.ListActiveSSOConnectionIDs(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("list active connections: %w", err)
	}

	active, enforced := []ids.UUID{}, []ids.UUID{}

	for _, row := range rows {
		active = append(active, row.ID)
		if row.Enforced {
			enforced = append(enforced, row.ID)
		}
	}

	workspaces, err := q.ListOrganizationWorkspaces(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("list organization workspaces: %w", err)
	}

	for _, w := range workspaces {
		settings, err := models.WithSSOConnections(w.Settings, active, enforced)
		if err != nil {
			return err
		}

		if err := q.SetWorkspaceSettings(ctx, models.SetWorkspaceSettingsParams{
			ID: w.ID, Settings: settings, Now: now,
		}); err != nil {
			return fmt.Errorf("mirror connections: %w", err)
		}
	}

	return nil
}

/*
unmarshal decodes a stored JSON document, leaving the value as it is
for an empty one.
*/
func unmarshal(raw []byte, v any) error {
	if len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, v)
}
