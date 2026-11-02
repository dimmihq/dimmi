/*
|--------------------------------------------------------------------------
| Invitation Status
|--------------------------------------------------------------------------
|
| An invitation is pending until it is accepted, revoked or runs out. It
| is marked expired lazily, when it is read or pruned, so an expired one
| may still say pending in the table until then.
|
*/

package enums

/*
InvitationStatus is where an invitation stands.
*/
type InvitationStatus string

/*
The invitation statuses.
*/
const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired"
)

/*
InvitationStatuses returns every status, in the order of the database
check.
*/
func InvitationStatuses() []InvitationStatus {
	return []InvitationStatus{
		InvitationStatusPending, InvitationStatusAccepted, InvitationStatusRevoked, InvitationStatusExpired,
	}
}

/*
Valid reports whether the status is one of the statuses.
*/
func (s InvitationStatus) Valid() bool {
	switch s {
	case InvitationStatusPending, InvitationStatusAccepted, InvitationStatusRevoked, InvitationStatusExpired:
		return true
	}

	return false
}
