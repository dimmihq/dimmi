/*
|--------------------------------------------------------------------------
| Principal
|--------------------------------------------------------------------------
|
| Once a guard has decided who is calling, every later layer asks the
| same question the same way: which user, through which kind of
| credential, bound to which tenant, holding which abilities, and how
| recently they proved who they are. The answer is a Principal stored in
| the request context.
|
*/

package auth

import (
	"context"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Principal is the authenticated caller of a request. An application may
store its own type, carrying more about the caller, as long as it
implements Principal.
*/
type Principal interface {
	/*
		ID returns the user the caller acts as.
	*/
	ID() ids.UUID

	/*
		Guard returns the kind of credential the caller presented, such as
		"session" or "token".
	*/
	Guard() string

	/*
		Tenant returns the tenant the credential is bound to, or "" when it
		reaches every tenant the user belongs to.
	*/
	Tenant() string

	/*
		Abilities returns the abilities the credential carries.
	*/
	Abilities() []string

	/*
		ConfirmedAt returns the last time the caller proved their identity,
		or the zero time when the credential cannot tell.
	*/
	ConfirmedAt() time.Time
}

/*
Identity is the Principal the framework's guards build.

User is the user, Via the kind of credential and Credential the id of
the session or token presented, so it can be listed or revoked.
TenantID is the tenant the credential is bound to, Granted its
abilities and Confirmed the last time the caller proved who they are.
*/
type Identity struct {
	User       ids.UUID
	Via        string
	Credential ids.UUID
	TenantID   string
	Granted    []string
	Confirmed  time.Time
}

var _ Principal = (*Identity)(nil)

/*
ID returns the user.
*/
func (i *Identity) ID() ids.UUID {
	return i.User
}

/*
Guard returns the kind of credential.
*/
func (i *Identity) Guard() string {
	return i.Via
}

/*
Tenant returns the tenant the credential is bound to.
*/
func (i *Identity) Tenant() string {
	return i.TenantID
}

/*
Abilities returns a copy of the abilities granted.
*/
func (i *Identity) Abilities() []string {
	return slices.Clone(i.Granted)
}

/*
ConfirmedAt returns the last time the caller proved who they are.
*/
func (i *Identity) ConfirmedAt() time.Time {
	return i.Confirmed
}

/*
From returns the principal a guard stored in the context.
*/
func From(ctx context.Context) (Principal, bool) {
	return http.Principal[Principal](ctx)
}
