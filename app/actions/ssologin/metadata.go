/*
|--------------------------------------------------------------------------
| Service Provider Metadata
|--------------------------------------------------------------------------
|
| An identity provider can read everything it needs about this side of a
| SAML connection from one public address: the entity id, where answers
| go, and the certificate requests are signed with. It says nothing
| secret.
|
*/

package ssologin

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Metadata describes this side of a SAML connection.

Errors: [exceptions.ErrNotFound] for an unknown or OpenID Connect
connection, or a wrapped error.
*/
type Metadata struct {
	DB   *database.DB
	SAML sso.SAMLProviders
}

/*
Handle returns the connection's metadata document.
*/
func (a *Metadata) Handle(ctx context.Context, connectionID ids.UUID) ([]byte, error) {
	conn, err := findConnection(ctx, a.DB, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.Connection.Protocol != sso.ProtocolSAML {
		return nil, exceptions.ErrNotFound
	}

	provider, err := a.SAML.For(ctx, conn)
	if err != nil {
		return nil, err
	}

	document, err := provider.Metadata()
	if err != nil {
		return nil, fmt.Errorf("write sp metadata: %w", err)
	}

	return document, nil
}
