/*
|--------------------------------------------------------------------------
| Auth Factor Status
|--------------------------------------------------------------------------
|
| A factor is pending while its user finishes setting it up and active
| once they proved it works. Pending factors never count at sign-in.
|
*/

package enums

/*
AuthFactorStatus is where an auth factor stands.
*/
type AuthFactorStatus string

/*
The auth factor statuses.
*/
const (
	AuthFactorStatusPending AuthFactorStatus = "pending"
	AuthFactorStatusActive  AuthFactorStatus = "active"
)

/*
AuthFactorStatuses returns every status, in the order of the database
check.
*/
func AuthFactorStatuses() []AuthFactorStatus {
	return []AuthFactorStatus{AuthFactorStatusPending, AuthFactorStatusActive}
}

/*
Valid reports whether the status is one of the statuses.
*/
func (s AuthFactorStatus) Valid() bool {
	switch s {
	case AuthFactorStatusPending, AuthFactorStatusActive:
		return true
	}

	return false
}
