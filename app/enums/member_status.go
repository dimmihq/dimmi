/*
|--------------------------------------------------------------------------
| Member Status
|--------------------------------------------------------------------------
|
| Members are never deleted: a member who leaves or is removed is
| deactivated, so conversations and audit entries keep pointing at them,
| and an invitation later reactivates the same row.
|
*/

package enums

/*
MemberStatus says whether a member may still act in their workspace.
*/
type MemberStatus string

/*
The member statuses.
*/
const (
	MemberStatusActive      MemberStatus = "active"
	MemberStatusDeactivated MemberStatus = "deactivated"
)

/*
MemberStatuses returns every status, in the order of the database check.
*/
func MemberStatuses() []MemberStatus {
	return []MemberStatus{MemberStatusActive, MemberStatusDeactivated}
}

/*
Valid reports whether the status is one of the statuses.
*/
func (s MemberStatus) Valid() bool {
	switch s {
	case MemberStatusActive, MemberStatusDeactivated:
		return true
	}

	return false
}
