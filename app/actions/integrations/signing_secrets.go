/*
|--------------------------------------------------------------------------
| Signing Secrets
|--------------------------------------------------------------------------
|
| A tool connected with an API key, such as incident.io, signs what it
| delivers with a secret of the endpoint the workspace created in that
| tool, pointed at the installation's own address. The secret is pasted
| after connecting, since the address exists only once the installation
| does, and is sealed beside the key. Only the delivery check reads it.
|
| The context must carry the workspace the installation belongs to.
|
*/

package integrations

import (
	"context"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
ErrSigningSecretInvalid refuses an empty or oversized secret.
*/
var ErrSigningSecretInvalid = fault.New(fault.Invalid, "signing_secret_invalid",
	"Signing secrets are 1 to 500 characters.").OnField("signing_secret")

/*
SigningSecret returns the secret the installation's deliveries are
signed with, empty when none was pasted.
*/
func (t *Tokens) SigningSecret(ctx context.Context, installationID ids.UUID) (string, error) {
	_, creds, err := t.installation(ctx, installationID)

	return creds.SigningSecret, err
}

/*
HasSigningSecret reports whether a secret was pasted for the
installation.
*/
func (t *Tokens) HasSigningSecret(ctx context.Context, installationID ids.UUID) bool {
	secret, err := t.SigningSecret(ctx, installationID)

	return err == nil && secret != ""
}

/*
SetSigningSecret seals a new signing secret beside the installation's
key. Teammates who manage integrations may.
*/
func (t *Tokens) SetSigningSecret(ctx context.Context, m authz.Membership, installationID ids.UUID, secret string,
) error {
	if err := requireManage(m); err != nil {
		return err
	}

	if n := len(secret); n < 1 || n > 500 {
		return ErrSigningSecretInvalid
	}

	row, creds, err := t.installation(ctx, installationID)
	if err != nil {
		return err
	}

	creds.SigningSecret = secret

	return t.storeInstallation(ctx, row, creds)
}
