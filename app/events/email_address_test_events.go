/*
|--------------------------------------------------------------------------
| Forwarding Test Events
|--------------------------------------------------------------------------
|
| Someone who sends a forwarding test watches for it to come back, so
| every step of the test refreshes the open settings page on its own.
| The event only names the test; the page reads it again through the API.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
EmailAddressTestUpdated is dispatched in the transaction that moved a
forwarding test of an address to Status.
*/
type EmailAddressTestUpdated struct {
	Base
	AddressID ids.UUID
	TestID    ids.UUID
	Status    string
}

/*
EventName names the event.
*/
func (EmailAddressTestUpdated) EventName() string {
	return "email_address_test.updated"
}
