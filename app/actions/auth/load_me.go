/*
|--------------------------------------------------------------------------
| Load Me
|--------------------------------------------------------------------------
|
| Clients learn who they are, and what they may do, from one answer: the
| account, the credential and how it was earned, the workspace in play
| with the caller's effective permissions there, and the other workspaces
| to switch to. Apps gate what they show on the permissions, never on
| the role, so a role can change meaning without breaking them.
|
| A browser or the user's own app names the workspace it is working in;
| a token is bound to one and may not name another.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"

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
LoadMeInput is the caller and the workspace they name, ids.Nil() for
none.
*/
type LoadMeInput struct {
	Principal   *appauth.Principal
	WorkspaceID ids.UUID
}

/*
Me is everything a client learns about its caller.

Workspace and Member are set when a workspace is in play, and
Permissions then holds what the caller may do there, with Abilities the
ability groups those permissions reach and AccessTeams the teams their
conversation access names. Token is the personal access token the
request came with, nil for other credentials. Workspaces
lists the user's workspaces for credentials that can switch between
them, and is nil for tokens.
*/
type Me struct {
	Principal   *appauth.Principal
	Workspace   *models.Workspace
	Member      *models.WorkspaceMember
	Token       *models.FindWorkspaceTokenRow
	Permissions []string
	Abilities   []string
	AccessTeams []ids.UUID
	Workspaces  []models.ListUserWorkspacesRow
}

/*
LoadMe describes the caller.

Rules:
  - A token is always in its own workspace; naming another one is
    refused.
  - A workspace the caller is not a member of, or that is deleted, is
    not found; one where their membership is deactivated is refused.

Errors: [exceptions.ErrWorkspaceMismatch],
[exceptions.ErrWorkspaceNotFound], [exceptions.ErrMembershipDeactivated],
or a wrapped database error.
*/
type LoadMe struct {
	DB          *database.DB
	Permissions *appauth.Catalog
}

/*
Handle loads the caller's picture.
*/
func (a *LoadMe) Handle(ctx context.Context, in LoadMeInput) (Me, error) {
	me := Me{Principal: in.Principal}

	workspaceID, err := a.workspaceOf(in)
	if err != nil {
		return Me{}, err
	}

	if workspaceID != ids.Nil() {
		if err := a.loadMembership(ctx, &me, workspaceID); err != nil {
			return Me{}, err
		}
	}

	if in.Principal.IsFirstParty() {
		rows, err := models.New(a.DB).ListUserWorkspaces(ctx, in.Principal.User.ID)
		if err != nil {
			return Me{}, fmt.Errorf("list workspaces of user: %w", err)
		}

		me.Workspaces = append([]models.ListUserWorkspacesRow{}, rows...)
	}

	return me, nil
}

/*
workspaceOf returns the workspace in play: a token's own, or the one a
first-party caller named.
*/
func (a *LoadMe) workspaceOf(in LoadMeInput) (ids.UUID, error) {
	if in.Principal.IsFirstParty() {
		return in.WorkspaceID, nil
	}

	if in.WorkspaceID != ids.Nil() && in.WorkspaceID != in.Principal.WorkspaceID {
		return ids.Nil(), exceptions.ErrWorkspaceMismatch
	}

	return in.Principal.WorkspaceID, nil
}

/*
loadMembership reads the workspace and the caller's membership of it,
scoped to that workspace, and works out the caller's permissions, as
narrowed by their custom role.
*/
func (a *LoadMe) loadMembership(ctx context.Context, me *Me, workspaceID ids.UUID) error {
	var (
		workspace models.Workspace
		member    models.WorkspaceMember
		custom    []string
	)

	err := a.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error

		q := models.New(tx)

		workspace, err = q.FindWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}

		member, custom, err = findMember(ctx, q, workspaceID, me.Principal.User.ID)
		if err != nil {
			return err
		}

		if me.AccessTeams, err = authz.LoadAccessTeams(ctx, q, member); err != nil {
			return err
		}

		return loadToken(ctx, q, me, workspaceID)
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && workspace.DeletedAt != nil) {
		return exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return fmt.Errorf("load membership: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) {
		return exceptions.ErrMembershipDeactivated
	}

	me.Workspace, me.Member = &workspace, &member
	me.Permissions = authz.PrincipalPermissions(a.Permissions, enums.Role(member.Role), custom, me.Principal)
	me.Abilities = authz.EffectiveAbilities(a.Permissions, me.Permissions)

	return nil
}

/*
findMember returns the user's membership of the workspace and what their
custom role lists, nil without one.
*/
func findMember(
	ctx context.Context, q *models.Queries, workspaceID, userID ids.UUID,
) (models.WorkspaceMember, []string, error) {
	member, err := q.FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
		WorkspaceID: workspaceID, UserID: userID,
	})
	if err != nil {
		return member, nil, err
	}

	custom, err := q.CustomPermissions(ctx, member)

	return member, custom, err
}

/*
loadToken reads the personal access token the request came with, if it
came with one.
*/
func loadToken(ctx context.Context, q *models.Queries, me *Me, workspaceID ids.UUID) error {
	if me.Principal.Kind != appauth.KindToken {
		return nil
	}

	token, err := q.FindWorkspaceToken(ctx, models.FindWorkspaceTokenParams{
		WorkspaceID: workspaceID, ID: me.Principal.CredentialID,
	})
	me.Token = &token

	return err
}
