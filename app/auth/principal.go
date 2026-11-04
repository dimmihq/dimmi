/*
|--------------------------------------------------------------------------
| Principal
|--------------------------------------------------------------------------
|
| Every guard answers with the same picture of the caller: which user,
| through which kind of credential, and how that credential was earned:
| the methods of the last full sign-in, when it happened, when the user
| last confirmed who they are and from which device. Policies and
| step-up checks read this picture and never the credential itself, so
| a new kind of credential changes a guard, not every check.
|
| The kind of caller is decided by looking the credential up, never by
| anything the client says about itself.
|
*/

package auth

import (
	"context"
	"slices"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of caller. A session is the user's browser, an app is the
user's own installed app, and a token is a personal access token or a
third-party OAuth token bound to one workspace.
*/
const (
	KindSession = "session"
	KindApp     = "app"
	KindToken   = "token"
)

/*
Principal is the caller of a request.

User is their account. Kind is the kind of credential and CredentialID
the session or token it was. WorkspaceID is the workspace a token is
bound to, ids.Nil() for sessions, which reach every workspace of the
user. MemberID is the member a token acts as. Granted lists the
credential's abilities, and MintedAsRole the role a token's member held
when it was minted, the most the token may ever do, empty when there is
no such ceiling. SignIn says how a session was earned, with
AuthenticatedAt the last full sign-in and Confirmed the last time the
user proved who they are; tokens carry neither.
*/
type Principal struct {
	User            models.User
	Kind            string
	CredentialID    ids.UUID
	WorkspaceID     ids.UUID
	MemberID        ids.UUID
	Granted         []string
	MintedAsRole    enums.Role
	SignIn          SignIn
	AuthenticatedAt time.Time
	Confirmed       time.Time
}

var _ fauth.Principal = (*Principal)(nil)

/*
ID returns the user the caller acts as.
*/
func (p *Principal) ID() ids.UUID {
	return p.User.ID
}

/*
Guard returns the kind of credential.
*/
func (p *Principal) Guard() string {
	return p.Kind
}

/*
Tenant returns the workspace a token is bound to, or "" for credentials
that reach every workspace of the user.
*/
func (p *Principal) Tenant() string {
	if p.WorkspaceID == ids.Nil() {
		return ""
	}

	return p.WorkspaceID.String()
}

/*
Abilities returns a copy of the credential's abilities.
*/
func (p *Principal) Abilities() []string {
	return slices.Clone(p.Granted)
}

/*
ConfirmedAt returns the last time the caller proved who they are. An
app proves it by signing its user in again in the browser as well as
by confirming, so for an app the later of the two counts.
*/
func (p *Principal) ConfirmedAt() time.Time {
	if p.Kind == KindApp && p.AuthenticatedAt.After(p.Confirmed) {
		return p.AuthenticatedAt
	}

	return p.Confirmed
}

/*
IsFirstParty reports whether the caller is the user themselves, through
their browser or their own app, rather than a token they handed out.
*/
func (p *Principal) IsFirstParty() bool {
	return p.Kind == KindSession || p.Kind == KindApp
}

/*
CurrentPrincipal returns the caller a guard stored in the context.
*/
func CurrentPrincipal(ctx context.Context) (*Principal, bool) {
	p, _ := fauth.From(ctx)
	principal, ok := p.(*Principal)

	return principal, ok
}
