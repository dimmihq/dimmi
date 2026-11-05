/*
|--------------------------------------------------------------------------
| SAML Provider Of A Connection
|--------------------------------------------------------------------------
|
| Each SAML connection talks to its identity provider with the key pair
| made for it and the certificates the owner pasted or the metadata
| published. Building the provider opens the sealed key, so it happens
| per sign-in and the key never sits in memory longer than that.
|
*/

package sso

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/saml"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
NameIDEmail is the SAML name id format whose value is an email address.
*/
const NameIDEmail = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"

/*
SAMLProviders builds the SAML provider of a connection.
*/
type SAMLProviders struct {
	Keys  encryption.KeyManager
	Links appauth.Links
	Clock clock.Clock
}

/*
For returns the provider of the SAML connection.
*/
func (p SAMLProviders) For(ctx context.Context, conn Connection) (*saml.Provider, error) {
	secret, err := OpenSecret(ctx, p.Keys, conn.Connection)
	if err != nil {
		return nil, err
	}

	key, err := saml.ParsePrivateKey(secret.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("read connection key: %w", err)
	}

	own, err := saml.ParseCertificate(conn.Config.SPCertificate)
	if err != nil {
		return nil, fmt.Errorf("read connection certificate: %w", err)
	}

	certificates, err := saml.ParseCertificates(conn.Config.IDPCertificates)
	if err != nil {
		return nil, fmt.Errorf("read identity provider certificates: %w", err)
	}

	id := conn.Connection.ID

	return saml.New(saml.Config{
		SP: saml.ServiceProvider{
			EntityID: p.Links.SAMLMetadata(id), ACSURL: p.Links.SAMLACS(id), Certificate: own, PrivateKey: key,
		},
		IDP: saml.IdentityProvider{
			EntityID: conn.Config.IDPEntityID, SSOURL: conn.Config.IDPSSOURL, Certificates: certificates,
		},
		NameIDFormat: conn.Config.NameIDFormat,
		Clock:        p.Clock,
	})
}
