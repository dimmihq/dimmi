/*
|--------------------------------------------------------------------------
| Sending Domain Events
|--------------------------------------------------------------------------
|
| A domain changing state changes who mail comes from, so admins hear of
| it on screen and through webhooks. Every check also refreshes what an
| open domain page shows, without anyone reloading it.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
EmailDomainStatusChanged is dispatched in the transaction that moved a
domain from one state to another. Reason is the failure reason's code
when the domain failed or degraded, empty otherwise.
*/
type EmailDomainStatusChanged struct {
	Base
	DomainID ids.UUID
	Domain   string
	From     string
	To       string
	Reason   string
}

/*
EventName names the event.
*/
func (EmailDomainStatusChanged) EventName() string {
	return "email_domain.status_changed"
}

/*
EmailDomainRecordsUpdated is dispatched in the transaction that stored a
check of the domain's records.
*/
type EmailDomainRecordsUpdated struct {
	Base
	DomainID ids.UUID
}

/*
EventName names the event.
*/
func (EmailDomainRecordsUpdated) EventName() string {
	return "email_domain.records_updated"
}
