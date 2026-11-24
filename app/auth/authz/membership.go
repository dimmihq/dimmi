/*
|--------------------------------------------------------------------------
| Membership In Play
|--------------------------------------------------------------------------
|
| Once a request's workspace is resolved, everything after it works in
| that workspace as the caller's member there, with the permissions
| worked out once for the whole request. Controllers and policies read
| it from the context and never resolve the workspace again.
|
*/

package authz

import (
	"context"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Membership is the workspace a request works in and the caller's active
membership of it, with Principal holding the caller's effective
permissions there, AccessTeams the teams their conversation access
names, read only for a mode that uses them, and HiddenInboxes the
restricted inboxes they may not open.
*/
type Membership struct {
	Workspace     models.Workspace
	Member        models.WorkspaceMember
	Principal     Principal
	AccessTeams   []ids.UUID
	HiddenInboxes []ids.UUID
}

/*
NewMembership returns the membership with the permissions given.
*/
func NewMembership(w models.Workspace, m models.WorkspaceMember, permissions []string) Membership {
	return Membership{
		Workspace: w,
		Member:    m,
		Principal: Principal{Role: enums.Role(m.Role), Permissions: permissions},
	}
}

/*
Can reports whether the caller holds the permission in the workspace.
*/
func (m Membership) Can(permission string) bool {
	return Can(m.Principal, permission)
}

type membershipKey struct{}

/*
WithMembership returns a copy of the context working in the membership.
*/
func WithMembership(ctx context.Context, m Membership) context.Context {
	return context.WithValue(ctx, membershipKey{}, m)
}

/*
CurrentMembership returns the membership the workspace middleware
resolved, and reports whether there is one.
*/
func CurrentMembership(ctx context.Context) (Membership, bool) {
	m, ok := ctx.Value(membershipKey{}).(Membership)

	return m, ok
}

type narrowedKey struct{}

/*
WithNarrowed returns a copy of the context in which a request served
within this process for the same teammate works with no more than the
given membership: only the permissions it holds and none of the inboxes
it hides. Operator's routines read the API this way, as a narrowed copy
of their owner. Nothing outside the process can set it.
*/
func WithNarrowed(ctx context.Context, m Membership) context.Context {
	return context.WithValue(ctx, narrowedKey{}, m)
}

/*
Narrowed returns the membership narrowed to what the context allows:
unchanged when the context narrows nothing or names another teammate
or workspace, else holding only the permissions both hold and hiding
every inbox either hides.
*/
func Narrowed(ctx context.Context, m Membership) Membership {
	to, ok := ctx.Value(narrowedKey{}).(Membership)
	if !ok || to.Member.ID != m.Member.ID || to.Workspace.ID != m.Workspace.ID {
		return m
	}

	held := []string{}
	for _, permission := range to.Principal.Permissions {
		if m.Can(permission) {
			held = append(held, permission)
		}
	}

	m.Principal.Permissions = held
	m.HiddenInboxes = slices.Clone(m.HiddenInboxes)

	for _, inbox := range to.HiddenInboxes {
		if !slices.Contains(m.HiddenInboxes, inbox) {
			m.HiddenInboxes = append(m.HiddenInboxes, inbox)
		}
	}

	return m
}
