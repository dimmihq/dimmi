/*
|--------------------------------------------------------------------------
| Sending Domain Errors
|--------------------------------------------------------------------------
|
| What can go wrong while adding and checking a sending domain. Only a
| domain a customer can own is accepted, a free mailbox domain gets a
| pointer to forwarding instead, checks are spaced out so nobody hammers
| the DNS servers, and a domain still sending for addresses is only
| removed once the admin agrees those addresses fall back.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Sending domain failures.

ErrInvalidDomain refuses a name that is no registrable domain, or one the
install uses itself. ErrPublicMailboxDomain refuses a free mailbox domain.
ErrDomainExists refuses a domain the workspace already added.
ErrDomainInUse refuses removing a domain addresses still send from.
ErrDomainCheckThrottled refuses a check asked for too soon after the last.
ErrDKIMRotationUnsupported refuses rotating a key the install does not
hold. ErrEmailProviderUnavailable reports a provider that did not answer.
ErrSendingNotConfigured refuses domains on an install whose customer mail
goes nowhere.
*/
var (
	ErrInvalidDomain = fault.New(fault.Invalid, "invalid_domain",
		"Enter a domain you own, such as yourcompany.com.").OnField("domain")
	ErrPublicMailboxDomain = fault.New(fault.Invalid, "invalid_domain",
		"This is a free mailbox domain. Forward your address to your dimmi forwarding address instead.").
		OnField("domain")
	ErrDomainExists = fault.New(fault.Conflict, "domain_exists",
		"This domain is already added to the workspace.").OnField("domain")
	ErrDomainInUse = fault.New(fault.Conflict, "domain_in_use",
		"Addresses still send from this domain. Remove it anyway to send them from your dimmi address.")
	ErrDomainCheckThrottled = fault.New(fault.RateLimited, "domain_check_throttled",
		"The domain was just checked. Try again in a few seconds.")
	ErrDKIMRotationUnsupported = fault.New(fault.Invalid, "dkim_rotation_unsupported",
		"This domain's signing key is managed by the email provider.")
	ErrEmailProviderUnavailable = fault.New(fault.Unavailable, "email_provider_unavailable",
		"The email provider did not answer. Try again in a moment.")
	ErrSendingNotConfigured = fault.New(fault.Invalid, "sending_not_configured",
		"This server does not send customer email through a provider yet.")
)
