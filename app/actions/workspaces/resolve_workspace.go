/*
|--------------------------------------------------------------------------
| Resolve Workspace
|--------------------------------------------------------------------------
|
| Every workspace route works inside one workspace, decided before the
| route runs. A browser or the user's own app names it in the
| Dimmi-Workspace header, and may leave it out only while the user
| belongs to a single workspace; a token is bound to its own and may not
| name another. Nothing about a workspace the caller cannot use leaks:
| an unknown, foreign or deleted workspace is simply not found, while a
| member whose access ended is told so.
|
| A suspended workspace can still be read but not changed. Opening a
| workspace records when the member last used it, at most once every few
| minutes, so the workspace used last opens first next time.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
accessInterval is how often opening a workspace updates when the member
last used it.
*/
const accessInterval = 5 * time.Minute

/*
ResolveWorkspaceInput is the caller, the workspace their request names,
"" for none, and whether the request changes anything.
*/
type ResolveWorkspaceInput struct {
	Principal *appauth.Principal
	Named     string
	Writes    bool
}

/*
ResolveWorkspace decides the workspace a request works in.

Rules:
  - A token works in its own workspace; naming another is refused.
  - A browser or app names the workspace, or leaves it out when the
    user belongs to exactly one.
  - Unknown, foreign and deleted workspaces are not found; a
    deactivated membership is refused.
  - A suspended workspace refuses changes.
  - The member's last access is recorded at most every five minutes.

Errors: [exceptions.ErrWorkspaceMismatch],
[exceptions.ErrWorkspaceRequired], [exceptions.ErrWorkspaceNotFound],
[exceptions.ErrMembershipDeactivated],
[exceptions.ErrWorkspaceSuspended], or a wrapped database error.
*/
type ResolveWorkspace struct {
	DB          *database.DB
	Clock       clock.Clock
	IDs         *ids.Registry
	Permissions *appauth.Catalog
}

/*
Handle returns the caller's membership of the workspace in play.
*/
func (a *ResolveWorkspace) Handle(ctx context.Context, in ResolveWorkspaceInput) (authz.Membership, error) {
	workspaceID, err := a.workspaceOf(ctx, in)
	if err != nil {
		return authz.Membership{}, err
	}

	m, custom, err := a.load(ctx, in.Principal, workspaceID)
	if err != nil {
		return authz.Membership{}, err
	}

	if in.Writes && m.Workspace.SuspendedAt != nil {
		return authz.Membership{}, exceptions.ErrWorkspaceSuspended
	}

	m.Principal = authz.Principal{
		Role:        enums.Role(m.Member.Role),
		Permissions: authz.PrincipalPermissions(a.Permissions, enums.Role(m.Member.Role), custom, in.Principal),
	}

	return m, nil
}

/*
workspaceOf returns the workspace the request works in.
*/
func (a *ResolveWorkspace) workspaceOf(ctx context.Context, in ResolveWorkspaceInput) (ids.UUID, error) {
	if !in.Principal.IsFirstParty() {
		return a.tokenWorkspace(in)
	}

	if in.Named != "" {
		id, err := a.IDs.Decode(string(models.PrefixWorkspace), in.Named)
		if err != nil {
			return ids.Nil(), exceptions.ErrWorkspaceNotFound
		}

		return id, nil
	}

	rows, err := models.New(a.DB).ListUserWorkspaces(ctx, in.Principal.User.ID)
	if err != nil {
		return ids.Nil(), fmt.Errorf("list workspaces of user: %w", err)
	}

	if len(rows) != 1 {
		return ids.Nil(), exceptions.ErrWorkspaceRequired
	}

	return rows[0].WorkspaceID, nil
}

/*
tokenWorkspace returns a token's own workspace, refusing a request that
names another.
*/
func (a *ResolveWorkspace) tokenWorkspace(in ResolveWorkspaceInput) (ids.UUID, error) {
	if in.Named == "" {
		return in.Principal.WorkspaceID, nil
	}

	id, err := a.IDs.Decode(string(models.PrefixWorkspace), in.Named)
	if err != nil || id != in.Principal.WorkspaceID {
		return ids.Nil(), exceptions.ErrWorkspaceMismatch
	}

	return id, nil
}

/*
load reads the workspace, the caller's membership inside it, the teams
their conversation access names and what their custom role lists, nil
without one, permissions aside, and records the access of a member
opening it.
*/
func (a *ResolveWorkspace) load(
	ctx context.Context, p *appauth.Principal, workspaceID ids.UUID,
) (authz.Membership, []string, error) {
	var (
		m      authz.Membership
		custom []string
	)

	scoped := tenancy.With(ctx, workspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		q := models.New(tx)

		m.Workspace, err = q.FindWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}

		m.Member, err = findCaller(ctx, q, p, workspaceID)
		if err != nil {
			return err
		}

		if custom, err = q.CustomPermissions(ctx, m.Member); err != nil {
			return err
		}

		if m.AccessTeams, err = authz.LoadAccessTeams(ctx, q, m.Member); err != nil {
			return err
		}

		if m.HiddenInboxes, err = authz.LoadHiddenInboxes(ctx, q, m.Member); err != nil || !p.IsFirstParty() {
			return err
		}

		now := a.Clock.Now()

		return q.TouchWorkspaceMemberAccess(ctx, models.TouchWorkspaceMemberAccessParams{
			Now: now, Since: now.Add(-accessInterval), WorkspaceID: workspaceID, ID: m.Member.ID,
		})
	})

	return m, custom, checkAccess(m.Workspace, m.Member, err)
}

/*
findCaller returns the caller's membership of the workspace: a token's
own member, or the user's membership for a browser or app.
*/
func findCaller(
	ctx context.Context, q *models.Queries, p *appauth.Principal, workspaceID ids.UUID,
) (models.WorkspaceMember, error) {
	if !p.IsFirstParty() {
		return q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: workspaceID, ID: p.MemberID})
	}

	return q.FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
		WorkspaceID: workspaceID, UserID: p.User.ID,
	})
}

/*
checkAccess turns what loading found into the refusal the caller gets,
if any.
*/
func checkAccess(workspace models.Workspace, member models.WorkspaceMember, err error) error {
	if errors.Is(err, database.ErrNotFound) || (err == nil && workspace.DeletedAt != nil) {
		return exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return fmt.Errorf("load workspace membership: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) {
		return exceptions.ErrMembershipDeactivated
	}

	return nil
}
