/*
|--------------------------------------------------------------------------
| Email Channel Errors
|--------------------------------------------------------------------------
|
| What can go wrong while connecting addresses and receiving mail. An
| install that receives no mail cannot give out forwarding addresses, a
| workspace connects each public address once, and an inbound source
| the install did not enable answers as if it did not exist, so its
| routes reveal nothing about the install's setup.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Email address failures.

ErrInboundNotConfigured refuses an address on an install without an
inbound domain. ErrAddressInUse refuses a public address the workspace
already connected, and ErrInvalidEmailAddress one that is not a plain
address or sits on the inbound domain itself. ErrNoSenderAvailable
refuses mail the workspace has no address to send from, and
ErrMailUnavailable reports mail the provider would not take.
ErrDefaultAddressRequired refuses to leave the workspace without a
default address: another one is made the default instead.
*/
var (
	ErrInvalidEmailAddress = fault.New(fault.Invalid, "invalid_address",
		"Enter an email address such as support@yourcompany.com.").OnField("address")
	ErrInboundNotConfigured = fault.New(fault.Invalid, "inbound_not_configured",
		"This server does not receive email yet. Set an inbound domain first.")
	ErrAddressInUse = fault.New(fault.Conflict, "address_in_use",
		"This address is already connected to the workspace.").OnField("address")
	ErrNoSenderAvailable = fault.New(fault.Invalid, "no_sender_available",
		"There is no address to send this email from.")
	ErrMailUnavailable = fault.New(fault.Unavailable, "mail_unavailable",
		"The email could not be sent. Try again in a moment.")
	ErrDefaultAddressRequired = fault.New(fault.Invalid, "default_address_required",
		"Make another address the default instead.").OnField("is_default")
)

/*
Inbound mail failures.

ErrInboundDriverDisabled answers a delivery to an inbound source the
install did not enable. ErrInboundUnauthorized refuses a delivery
without the source's secret. ErrRawEmailPurged answers for a raw message
retention already removed.
*/
var (
	ErrInboundDriverDisabled = fault.New(fault.NotFound, "inbound_driver_disabled", "Not Found.")
	ErrInboundUnauthorized   = fault.New(fault.Unauthenticated, "unauthenticated", "Unauthenticated.")
	ErrRawEmailPurged        = fault.New(fault.Gone, "raw_email_purged", "The original email is no longer kept.")
)
