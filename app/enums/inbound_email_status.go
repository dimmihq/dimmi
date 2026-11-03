/*
|--------------------------------------------------------------------------
| Inbound Email Status
|--------------------------------------------------------------------------
|
| Every received email is kept with where it got to: just received,
| being processed, turned into a message, deliberately ignored, set
| aside as spam, or failed with a reason an admin can read.
|
*/

package enums

/*
InboundEmailStatus is where a received email stands.
*/
type InboundEmailStatus string

/*
The statuses, in the order of the database check.
*/
const (
	InboundEmailStatusReceived    InboundEmailStatus = "received"
	InboundEmailStatusProcessing  InboundEmailStatus = "processing"
	InboundEmailStatusProcessed   InboundEmailStatus = "processed"
	InboundEmailStatusIgnored     InboundEmailStatus = "ignored"
	InboundEmailStatusFailed      InboundEmailStatus = "failed"
	InboundEmailStatusQuarantined InboundEmailStatus = "quarantined"
)

/*
InboundEmailStatuses returns every status, in the order of the database
check.
*/
func InboundEmailStatuses() []InboundEmailStatus {
	return []InboundEmailStatus{
		InboundEmailStatusReceived, InboundEmailStatusProcessing, InboundEmailStatusProcessed,
		InboundEmailStatusIgnored, InboundEmailStatusFailed, InboundEmailStatusQuarantined,
	}
}

/*
Valid reports whether the status is one of the statuses.
*/
func (s InboundEmailStatus) Valid() bool {
	switch s {
	case InboundEmailStatusReceived, InboundEmailStatusProcessing, InboundEmailStatusProcessed,
		InboundEmailStatusIgnored, InboundEmailStatusFailed, InboundEmailStatusQuarantined:
		return true
	}

	return false
}
