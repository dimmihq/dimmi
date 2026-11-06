/*
|--------------------------------------------------------------------------
| Create Connection
|--------------------------------------------------------------------------
|
| An owner starts connecting the company's identity provider. The
| connection begins as a draft that signs nobody in, with a key pair of
| its own for SAML, whose certificate the owner hands to the identity
| provider. Secrets are sealed the moment they arrive and never shown
| again.
|
*/

package sso

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/saml"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateConnectionInput is who creates a connection, through their
membership, and what it is.
*/
type CreateConnectionInput struct {
	Principal    *appauth.Principal
	Membership   authz.Membership
	Protocol     string
	Name         string
	Config       Config
	ClientSecret string
}

/*
CreateConnection adds a draft connection to the organization of the
workspace in play.

Rules:
  - The caller needs sso:manage.
  - An organization has at most ten connections.
  - SAML settings, when given, must read; OpenID Connect needs an
    issuer, a client id and a client secret.
  - A SAML connection gets a key pair of its own.

Events: [appevents.SSOConnectionChanged] created.

Errors: [exceptions.ErrForbidden], [exceptions.ErrConnectionLimitReached],
[exceptions.ErrInvalidConnectionConfig], or a wrapped error.
*/
type CreateConnection struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Keys   encryption.KeyManager
	Links  appauth.Links
}

/*
Handle creates the connection and returns it.
*/
func (a *CreateConnection) Handle(ctx context.Context, in CreateConnectionInput) (Connection, error) {
	if !in.Membership.Can(managePermission) {
		return Connection{}, exceptions.ErrForbidden
	}

	if err := checkConfig(in.Protocol, in.Config, in.ClientSecret); err != nil {
		return Connection{}, err
	}

	var out Connection

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		organizationID := in.Membership.Workspace.OrganizationID

		count, err := q.CountSSOConnections(ctx, organizationID)
		if err != nil {
			return fmt.Errorf("count connections: %w", err)
		}

		if count >= maxConnections {
			return exceptions.ErrConnectionLimitReached.With("limit", maxConnections)
		}

		row, err := q.CreateSSOConnection(ctx, models.CreateSSOConnectionParams{
			OrganizationID: organizationID, Protocol: in.Protocol, Name: str.Clean(in.Name), Config: []byte("{}"),
			CreatedByUserID: &in.Principal.User.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create connection: %w", err)
		}

		if out, err = a.complete(ctx, q, row, in); err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.SSOConnectionChanged{
			WorkspaceID: in.Membership.Workspace.ID, ConnectionID: row.ID, Name: row.Name, Change: "created",
			Changes: map[string]any{"protocol": map[string]any{"to": row.Protocol}}, ChangedBy: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
complete writes the connection's settings and sealed secret, now that
its id, which its addresses and key are bound to, exists.
*/
func (a *CreateConnection) complete(
	ctx context.Context, q *models.Queries, row models.SSOConnection, in CreateConnectionInput,
) (Connection, error) {
	cfg, secret := in.Config, Secret{ClientSecret: in.ClientSecret}

	if in.Protocol == ProtocolSAML {
		pair, err := saml.NewKeyPair(a.Links.SAMLMetadata(row.ID), a.Clock.Now())
		if err != nil {
			return Connection{}, err
		}

		cfg.SPCertificate, secret = pair.Certificate, Secret{PrivateKey: pair.PrivateKey}
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return Connection{}, fmt.Errorf("write connection config: %w", err)
	}

	sealed, err := seal(ctx, a.Keys, row, secret)
	if err != nil {
		return Connection{}, err
	}

	updated, err := q.UpdateSSOConnection(ctx, models.UpdateSSOConnectionParams{
		OrganizationID: row.OrganizationID, ID: row.ID, Name: row.Name, Status: row.Status, Config: raw,
		SecretEncrypted: sealed, JitEnabled: row.JitEnabled, DefaultRole: row.DefaultRole,
		DefaultTeamIds: row.DefaultTeamIds, GroupRoleMap: row.GroupRoleMap, Enforced: row.Enforced,
		DefaultCustomRoleID: row.DefaultCustomRoleID,
		Now:                 a.Clock.Now(),
	})
	if err != nil {
		return Connection{}, fmt.Errorf("complete connection: %w", err)
	}

	return Decode(updated)
}

/*
checkConfig refuses settings the protocol cannot use: certificates that
do not read, or an OpenID Connect connection without its issuer, client
or secret, "" for none.
*/
func checkConfig(protocol string, cfg Config, secret string) error {
	if protocol == ProtocolOIDC {
		if cfg.Issuer == "" || cfg.ClientID == "" || secret == "" {
			return exceptions.ErrInvalidConnectionConfig
		}

		return nil
	}

	for _, certificate := range cfg.IDPCertificates {
		if _, err := saml.ParseCertificate(certificate); err != nil {
			return exceptions.ErrInvalidConnectionConfig
		}
	}

	return nil
}

/*
secretOf stands in for a stored secret, "" when there is none, so a
check needs no key to see that one is there.
*/
func secretOf(sealed []byte) string {
	if len(sealed) == 0 {
		return ""
	}

	return "sealed"
}
