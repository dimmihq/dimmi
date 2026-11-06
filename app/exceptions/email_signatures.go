/*
|--------------------------------------------------------------------------
| Signature Errors
|--------------------------------------------------------------------------
|
| What can go wrong while saving a signature. A placeholder we cannot
| fill would reach the customer as written, so it is refused when the
| signature is saved rather than found in a sent reply.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Signature failures.

ErrUnknownSignaturePlaceholder refuses a body with a placeholder no
reply can fill; its placeholders detail lists them. ErrSignatureExists
refuses a second signature for the same teammate and address.
ErrSignatureAddressRequired refuses an address signature that names no
address.
*/
var (
	ErrUnknownSignaturePlaceholder = fault.New(fault.Invalid, "unknown_placeholder",
		"Use only {{member_name}}, {{member_title}}, {{address_name}} or {{company_name}}.").OnField("body")
	ErrSignatureExists = fault.New(fault.Conflict, "signature_exists",
		"There is already a signature for this teammate and address. Change that one instead.")
	ErrSignatureAddressRequired = fault.New(fault.Invalid, "address_required",
		"Choose the address this signature is for.").OnField("email_address_id")
)
