/*
|--------------------------------------------------------------------------
| Contact Erasure Events
|--------------------------------------------------------------------------
|
| A request to erase a customer moves through its life: asked for, held
| by a retention hold, canceled during its grace period, or done. The
| audit log records each step. ContactErased goes out as contact.deleted
| once nothing about the customer is left.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ContactErasureRequested is dispatched when a member asks for a customer
to be erased. Immediate says the grace period was skipped.
*/
type ContactErasureRequested struct {
	Base

	ErasureID ids.UUID
	ContactID ids.UUID
	Immediate bool
}

/*
EventName names the event.
*/
func (ContactErasureRequested) EventName() string { return "contact_erasure.requested" }

/*
ContactErasureCanceled is dispatched when a request is withdrawn during
its grace period. Conflicted lists the identities another contact took
meanwhile, which could not be given back.
*/
type ContactErasureCanceled struct {
	Base

	ErasureID  ids.UUID
	ContactID  ids.UUID
	Conflicted []string
}

/*
EventName names the event.
*/
func (ContactErasureCanceled) EventName() string { return "contact_erasure.canceled" }

/*
ContactErasureHeld is dispatched when a retention hold keeps a customer
from being erased. Nothing is removed while the hold lasts.
*/
type ContactErasureHeld struct {
	Base

	ErasureID ids.UUID
	ContactID ids.UUID
}

/*
EventName names the event.
*/
func (ContactErasureHeld) EventName() string { return "contact_erasure.held" }

/*
ContactErased is dispatched when nothing about the customer is left.
Counts says how much each step removed.
*/
type ContactErased struct {
	Base

	ErasureID ids.UUID
	ContactID ids.UUID
	Counts    map[string]int64
}

/*
EventName names the event.
*/
func (ContactErased) EventName() string { return "contact.deleted" }
