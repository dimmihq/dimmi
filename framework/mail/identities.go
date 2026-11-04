/*
|--------------------------------------------------------------------------
| Sending Identities
|--------------------------------------------------------------------------
|
| A provider that signs mail with its own keys, such as SES, holds one
| identity per sending domain: its signing tokens, the return path it
| bounces through, and whether it signs for the domain yet. The caller
| sees this one shape whichever provider holds the identity.
|
*/

package mail

import (
	"context"
	"fmt"
)

/*
ErrIdentityNotFound means the provider has no such identity. It also
matches ErrPermanent.
*/
var ErrIdentityNotFound = fmt.Errorf("mail: identity not found: %w", ErrPermanent)

/*
Identity is a sending domain as the provider sees it. Tokens are the
signing tokens whose aliases point into SigningHostedZone. DKIMStatus and
MailFromStatus are what the provider last found, in its own words, such
as SUCCESS or PENDING.
*/
type Identity struct {
	Tokens            []string
	SigningHostedZone string
	DKIMStatus        string
	MailFromStatus    string
	Verified          bool
}

/*
IdentityManager manages a provider's sending identities.
*/
type IdentityManager interface {
	/*
		Region returns the region the identities live in.
	*/
	Region() string

	/*
		Create registers the domain, or returns it when it exists.
	*/
	Create(ctx context.Context, domain string) (Identity, error)

	/*
		Get returns the identity's current state.
	*/
	Get(ctx context.Context, domain string) (Identity, error)

	/*
		SetMailFrom points the identity's return path at the name.
	*/
	SetMailFrom(ctx context.Context, domain, mailFrom string) error

	/*
		RestartDKIM makes the provider look for the signing records again.
	*/
	RestartDKIM(ctx context.Context, domain string) error

	/*
		AssociateTenant ties the resource to the tenant.
	*/
	AssociateTenant(ctx context.Context, tenant, resourceARN string) error
}
