/*
|--------------------------------------------------------------------------
| Integration Credentials
|--------------------------------------------------------------------------
|
| An installation's tokens and a member grant's token are sealed with
| the workspace's data keys, bound to the row they belong to: the
| installation's id, or the installation and member of a grant, which
| stay the same when the grant is replaced. A disconnected row keeps a
| sealed empty value, so its tokens are gone at once.
|
*/

package integrations

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
)

/*
credentials are the secrets of an installation or a grant. Access acts
as the app or the member until AccessExpiry, and Refresh renews it.
SigningSecret and AppToken come only with a custom app's pasted tokens.
*/
type credentials struct {
	Access        string     `json:"access"`
	AccessExpiry  *time.Time `json:"access_expiry,omitempty"`
	Refresh       string     `json:"refresh,omitempty"`
	RefreshExpiry *time.Time `json:"refresh_expiry,omitempty"`
	SigningSecret string     `json:"signing_secret,omitempty"`
	AppToken      string     `json:"app_token,omitempty"`
}

/*
expiring reports whether the access token stops working within the skew
and can be renewed.
*/
func (c credentials) expiring(now time.Time, skew time.Duration) bool {
	return c.Refresh != "" && c.AccessExpiry != nil && !now.Add(skew).Before(*c.AccessExpiry)
}

/*
botCredentials reads the app tokens of a grant.
*/
func botCredentials(grant oauthclient.Grant) credentials {
	return credentials{
		Access: value(grant.Access), AccessExpiry: expiry(grant.Access),
		Refresh: value(grant.Refresh), RefreshExpiry: expiry(grant.Refresh),
	}
}

/*
userCredentials reads the member's own tokens of a grant.
*/
func userCredentials(grant oauthclient.Grant) credentials {
	return credentials{
		Access: value(grant.UserAccess), AccessExpiry: expiry(grant.UserAccess),
		Refresh: value(grant.UserRefresh), RefreshExpiry: expiry(grant.UserRefresh),
	}
}

/*
renewed applies a refreshed grant, keeping the refresh token when the
provider did not rotate it.
*/
func (c credentials) renewed(access, refresh *oauthclient.Token) credentials {
	c.Access, c.AccessExpiry = value(access), expiry(access)
	if refresh != nil {
		c.Refresh, c.RefreshExpiry = refresh.Value, expiry(refresh)
	}

	return c
}

func installationAAD(id ids.UUID) []byte {
	return []byte("integration_installations.credentials_encrypted:" + id.String())
}

func grantAAD(installationID, memberID ids.UUID) []byte {
	return []byte("integration_member_grants.credentials_encrypted:" + installationID.String() + ":" +
		memberID.String())
}

/*
seal encrypts the credentials for the workspace, bound to the data.
*/
func (s *Services) seal(ctx context.Context, workspaceID ids.UUID, aad []byte, c credentials,
) (encryption.Sealed, error) {
	sealed, err := encryption.Seal(ctx, s.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: workspaceID.String()}, AAD: aad,
	}, c)
	if err != nil {
		return nil, fmt.Errorf("seal integration credentials: %w", err)
	}

	return sealed, nil
}

/*
open decrypts credentials sealed for the data.
*/
func (s *Services) open(ctx context.Context, sealed encryption.Sealed, aad []byte) (credentials, error) {
	c, err := encryption.Open[credentials](ctx, s.Keys, sealed, aad)
	if err != nil {
		return credentials{}, fmt.Errorf("open integration credentials: %w", err)
	}

	return c, nil
}
