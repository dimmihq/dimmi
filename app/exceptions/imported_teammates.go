/*
|--------------------------------------------------------------------------
| Imported Teammate Errors
|--------------------------------------------------------------------------
|
| What can go wrong while bringing teammates over from another tool, and
| what a teammate brought over hears while they wait to be enabled.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
ErrImportedEmailInvalid refuses a teammate whose address is not one.
ErrImportedSourceInvalid refuses a source that is not a short lowercase
name. ErrAccessAwaitingEnable refuses a person brought over from another
tool until an admin enables them.
*/
var (
	ErrImportedEmailInvalid = fault.New(fault.Invalid, "invalid_email",
		"The email must be a valid email address.").OnField("email")
	ErrImportedSourceInvalid = fault.New(fault.Invalid, "invalid_import_source",
		"The source must be a short lowercase name, such as intercom.").OnField("source")
	ErrAccessAwaitingEnable = fault.New(fault.Forbidden, "access_awaiting_enable",
		"Waiting for an admin to enable your access.")
)
