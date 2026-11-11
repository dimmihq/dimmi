/*
|--------------------------------------------------------------------------
| Tracked Events
|--------------------------------------------------------------------------
|
| A customer did something in the team's product and the event was
| recorded on their summary. It is dispatched in the transaction that
| recorded it, once per event: a retried event the workspace already
| saw dispatches nothing.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ContactEventTracked says the contact did the event Name. Ref names this
one event, so whatever it starts begins only once.
*/
type ContactEventTracked struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
	Name        string
	Ref         string
}

/*
EventName names the event.
*/
func (ContactEventTracked) EventName() string {
	return "contact_event.tracked"
}
