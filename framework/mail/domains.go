/*
|--------------------------------------------------------------------------
| Provider Domains
|--------------------------------------------------------------------------
|
| Some providers can set up a sending domain over their API and report
| the state of every DNS record, so nobody has to open the provider's
| dashboard. Drivers offer this through an optional interface, built with
| an account credential that is separate from the sending key. The
| provider's answer is translated into one shape, so the caller shows
| records and decides on retries without knowing the provider.
|
| Tracking stays off for every domain created here, because mail must
| read as written by a person. Where a provider keeps tracking in a
| separate call that can fail after the domain exists, the caller gets
| the domain together with ErrDomainTrackingLeftOn, so the domain's id is
| never lost.
|
*/

package mail

import (
	"context"
	"errors"
)

/*
Errors a domain manager returns, beside ErrPermanent, ErrTemporary and
*RateLimitedError, one of which every failure also matches.

ErrDomainNotFound means the provider has no such domain, for instance
because it was deleted there by hand. It also matches ErrPermanent.
ErrDomainAccessDenied means the provider refused the credential; it also
matches ErrTemporary, since the call passes once the operator fixes it.
ErrDomainTrackingLeftOn comes from CreateDomain with a usable domain when
the domain was created but its tracking could not be turned off; the
cause stays in the chain, so test for this error first.
ErrDomainsUnsupported means a driver has no domain API, or no credential
was given for it.
*/
var (
	ErrDomainNotFound       = errors.New("mail: provider domain not found")
	ErrDomainAccessDenied   = errors.New("mail: provider refused the domain credential")
	ErrDomainTrackingLeftOn = errors.New("mail: provider domain tracking left on")
	ErrDomainsUnsupported   = errors.New("mail: provider domains not supported")
)

/*
The purposes a DomainRecord can serve.

PurposeDKIM publishes a signing key. PurposeReturnPath points the bounce
address at the provider. PurposeSPF allows the provider to send.
PurposeMX receives the provider's bounces and feedback. PurposeTracking
serves rewritten links; it is reported as the provider lists it, but no
mail depends on it because tracking stays off. PurposeOther is any
record the driver cannot place.
*/
const (
	PurposeDKIM       = "dkim"
	PurposeReturnPath = "return_path"
	PurposeSPF        = "spf"
	PurposeMX         = "mx"
	PurposeTracking   = "tracking"
	PurposeOther      = "other"
)

/*
ProviderDomain is a sending domain as the provider sees it. ID is what
the other DomainManager methods take. Verified means the provider will
send as this domain. Records are the DNS records it asks for.
*/
type ProviderDomain struct {
	ID       string
	Verified bool
	Records  []DomainRecord
}

/*
DomainRecord is one DNS record a provider asks for. Purpose is one of the
Purpose constants. Type is the upper case record type, such as CNAME.
Host is fully qualified, without a trailing dot. Priority is set for MX
records only. Valid means the provider found the record in DNS.
*/
type DomainRecord struct {
	Purpose  string
	Type     string
	Host     string
	Value    string
	Priority int
	Valid    bool
}

/*
DomainOptions configure a domain manager. Token is the credential for the
provider's domain API, not the sending key. Endpoint replaces the API
base URL when set, for a provider's EU region.
*/
type DomainOptions struct {
	Token    string
	Endpoint string
}

/*
DomainManager sets up sending domains at a provider. Implementations are
safe for concurrent use.
*/
type DomainManager interface {
	/*
		CreateDomain registers the domain with tracking off and returns the
		records to publish.
	*/
	CreateDomain(ctx context.Context, domain string) (ProviderDomain, error)

	/*
		GetDomain returns the state the provider last recorded, without
		asking it to look at DNS again.
	*/
	GetDomain(ctx context.Context, id string) (ProviderDomain, error)

	/*
		VerifyDomain asks the provider to look at DNS now and returns the
		resulting state. Some providers check in the background, so the
		state may lag behind until a later GetDomain.
	*/
	VerifyDomain(ctx context.Context, id string) (ProviderDomain, error)

	/*
		DeleteDomain removes the domain from the provider.
	*/
	DeleteDomain(ctx context.Context, id string) error
}
