/*
|--------------------------------------------------------------------------
| Slack Secrets
|--------------------------------------------------------------------------
|
| A workspace's own Slack app comes with secrets pasted from Slack: the
| bot token every call is made with, the signing secret every delivery
| is checked against, and, for an app on a socket, the app token that
| opens it. They are sealed into the connection's row, bound to it, so
| a copy in another row fails to open, and never leave it in a response,
| a log line or an event.
|
| A connection made through the install's shared app keeps no tokens of
| its own: its installation holds them, and they are asked for there.
|
*/

package channels

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Secrets are a Slack app's pasted secrets.
*/
type Secrets struct {
	BotToken      string `json:"bot_token,omitempty"`
	SigningSecret string `json:"signing_secret,omitempty"`
	AppToken      string `json:"app_token,omitempty"`
}

/*
InstallationTokens hands out the tokens of a Slack installation the
integrations hold, for connections made through one.
*/
type InstallationTokens interface {
	/*
		BotToken returns the installation's bot token.
	*/
	BotToken(ctx context.Context, workspaceID, installationID ids.UUID) (string, error)

	/*
		UserToken returns the teammate's own token on the installation
		and their Slack user, reporting false when they never connected
		their account.
	*/
	UserToken(ctx context.Context, workspaceID, installationID, memberID ids.UUID) (UserGrant, bool, error)
}

/*
UserGrant is a teammate's own Slack token and the Slack user it acts
as.
*/
type UserGrant struct {
	Token  string
	UserID string
}

/*
SlackSecrets seals and opens the connection's secrets, and hands the
driver its tokens: from the installation for a connection made through
one, when Installations is set, else from the connection's own row.
*/
type SlackSecrets struct {
	Keys          *encryption.Keys
	Installations InstallationTokens
}

/*
secretsAAD names the column and row the secrets belong to.
*/
func secretsAAD(connectionID ids.UUID) []byte {
	return []byte("channel_connections.credentials_encrypted:" + connectionID.String())
}

/*
Seal seals the secrets for the connection's row.
*/
func (s *SlackSecrets) Seal(ctx context.Context, conn models.ChannelConnection, secrets Secrets) (
	encryption.Sealed, error,
) {
	sealed, err := encryption.Seal(ctx, s.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: conn.WorkspaceID.String()}, AAD: secretsAAD(conn.ID),
	}, secrets)
	if err != nil {
		return nil, fmt.Errorf("seal slack secrets: %w", err)
	}

	return sealed, nil
}

/*
Open opens the connection's secrets, empty when it has none yet.
*/
func (s *SlackSecrets) Open(ctx context.Context, conn models.ChannelConnection) (Secrets, error) {
	if len(conn.CredentialsEncrypted) == 0 {
		return Secrets{}, nil
	}

	secrets, err := encryption.Open[Secrets](ctx, s.Keys, conn.CredentialsEncrypted, secretsAAD(conn.ID))
	if err != nil {
		return Secrets{}, fmt.Errorf("open slack secrets: %w", err)
	}

	return secrets, nil
}

/*
BotToken returns the connection's bot token. A connection without one
is broken until it is reconnected.
*/
func (s *SlackSecrets) BotToken(ctx context.Context, conn models.ChannelConnection) (string, error) {
	if conn.IntegrationInstallationID != nil && s.Installations != nil {
		return s.Installations.BotToken(ctx, conn.WorkspaceID, *conn.IntegrationInstallationID)
	}

	secrets, err := s.Open(ctx, conn)
	if err != nil {
		return "", err
	}

	if secrets.BotToken == "" {
		return "", &contracts.SendError{
			Code: "slack_auth_failed", Message: "The Slack connection has no bot token. Reconnect Slack.",
			Err: contracts.ErrConnectionBroken,
		}
	}

	return secrets.BotToken, nil
}

/*
UserToken returns the teammate's own Slack token from the connection's
installation. A connection's pasted secrets hold only the app's tokens,
so without an installation replies go as the bot with the teammate's
name and picture.
*/
func (s *SlackSecrets) UserToken(ctx context.Context, conn models.ChannelConnection, memberID ids.UUID) (
	string, bool, error,
) {
	if conn.IntegrationInstallationID == nil || s.Installations == nil {
		return "", false, nil
	}

	grant, ok, err := s.Installations.UserToken(ctx, conn.WorkspaceID, *conn.IntegrationInstallationID, memberID)

	return grant.Token, ok, err
}
