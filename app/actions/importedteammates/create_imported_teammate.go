/*
|--------------------------------------------------------------------------
| Create Imported Teammate
|--------------------------------------------------------------------------
|
| A workspace moving over from another tool brings its teammates along,
| so their old conversations keep the names of the people who wrote
| them. Each one becomes a member who has no access at all: deactivated
| and waiting until an admin enables them, which gives them the role and
| teams remembered here.
|
| A teammate already in the workspace is left exactly as they are. Someone
| who already has a dimmi account keeps it; anyone else gets a placeholder
| account with no password and an unproven address, which nobody can sign
| in to until the person signs up and makes it theirs.
|
*/

package importedteammates

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxEmailLength is the longest address a teammate may have.
*/
const maxEmailLength = 254

/*
sourcePattern is what the name of a tool teammates come from looks like.
*/
var sourcePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)

/*
CreateImportedTeammateInput is one teammate of the other tool: the
workspace they join, who they are, the role and teams enabling gives
them, and the tool they come from, such as intercom. AvatarURL is a
picture of them to copy, "" for none.
*/
type CreateImportedTeammateInput struct {
	WorkspaceID ids.UUID
	Email       string
	Name        string
	AvatarURL   string
	Role        enums.Role
	TeamIDs     []ids.UUID
	Source      string
}

/*
Imported is the member an import gave or found, with their account, and
whether the import created the membership.
*/
type Imported struct {
	Member  models.WorkspaceMember
	User    models.User
	Created bool
}

/*
CreateImportedTeammate adds a teammate brought over from another tool.

Callers decide who may import; this action runs as the system.

Rules:
  - An existing member of the workspace, of any status, is returned as
    they are.
  - Otherwise the live account with the address is used, or a placeholder
    account is created: no password, unproven address, created by import.
  - The member is deactivated, joined by import, and waits to be enabled
    with the role and teams given. An owner role becomes admin: ownership
    is only ever handed over by hand. Teams that do not exist are
    dropped.
  - A placeholder account with an avatar URL gets the picture copied.

Events: [appevents.MemberJoined] for a new member.

Errors: [exceptions.ErrImportedEmailInvalid],
[exceptions.ErrImportedSourceInvalid], or a wrapped error.
*/
type CreateImportedTeammate struct {
	DB         *database.DB
	Clock      clock.Clock
	Events     *events.Dispatcher
	Queue      *queue.Queue
	CreateUser *users.CreateUser
}

/*
Handle imports the teammate, inside the transaction in the context when
there is one.
*/
func (a *CreateImportedTeammate) Handle(ctx context.Context, in CreateImportedTeammateInput) (Imported, error) {
	email, ok := normalizeEmail(in.Email)
	if !ok {
		return Imported{}, exceptions.ErrImportedEmailInvalid
	}

	if !sourcePattern.MatchString(in.Source) {
		return Imported{}, exceptions.ErrImportedSourceInvalid
	}

	var out Imported

	scoped := tenancy.With(ctx, in.WorkspaceID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		existing, err := q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
			WorkspaceID: in.WorkspaceID, Email: email,
		})
		if err == nil {
			out.Member = existing
			out.User, err = q.FindUser(ctx, existing.UserID)

			return err
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find member by email: %w", err)
		}

		user, placeholder, err := a.account(ctx, tx, email, in)
		if err != nil {
			return err
		}

		out.User, out.Created = user, true
		out.Member, err = a.join(ctx, tx, user, in)
		if err != nil {
			return err
		}

		if placeholder && in.AvatarURL != "" && user.AvatarPath == nil {
			return a.Queue.PushTx(ctx, tx, jobs.FetchUserAvatarArgs{UserID: user.ID, URL: in.AvatarURL})
		}

		return nil
	})

	return out, err
}

/*
account returns the live account with the address, or a new placeholder
one, and reports whether it is the placeholder.
*/
func (a *CreateImportedTeammate) account(
	ctx context.Context, tx *database.Tx, email string, in CreateImportedTeammateInput,
) (models.User, bool, error) {
	user, err := models.New(tx).FindUserByEmail(ctx, email)
	if err == nil {
		return user, false, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return user, false, fmt.Errorf("find account by email: %w", err)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = email[:strings.LastIndex(email, "@")]
	}

	user, err = a.CreateUser.Handle(ctx, users.CreateUserInput{Name: name, Email: email, CreatedVia: appauth.ViaImport})

	return user, true, err
}

/*
join adds the account to the workspace, deactivated and waiting to be
enabled.
*/
func (a *CreateImportedTeammate) join(
	ctx context.Context, tx *database.Tx, user models.User, in CreateImportedTeammateInput,
) (models.WorkspaceMember, error) {
	q := models.New(tx)

	teams, err := q.ListExistingTeams(ctx, models.ListExistingTeamsParams{WorkspaceID: in.WorkspaceID, Ids: in.TeamIDs})
	if err != nil {
		return models.WorkspaceMember{}, fmt.Errorf("find teams: %w", err)
	}

	source := in.Source

	member, err := q.CreateImportedWorkspaceMember(ctx, models.CreateImportedWorkspaceMemberParams{
		WorkspaceID: in.WorkspaceID, UserID: user.ID, Role: string(enabledRole(in.Role)), ImportSource: &source,
		TeamIds: append([]ids.UUID{}, teams...), Now: a.Clock.Now(),
	})
	if err != nil {
		return member, fmt.Errorf("add imported member: %w", err)
	}

	return member, a.Events.Dispatch(ctx, tx, appevents.MemberJoined{
		WorkspaceID: in.WorkspaceID, MemberID: member.ID, UserID: user.ID, JoinedVia: appauth.ViaImport,
	})
}

/*
enabledRole returns the role an imported teammate is enabled with:
admin for an admin or an owner of the other tool, member otherwise.
*/
func enabledRole(role enums.Role) enums.Role {
	if role == appauth.ManagerRole() || appauth.IsOwnerRole(role) {
		return appauth.ManagerRole()
	}

	return enums.RoleMember
}

/*
normalizeEmail trims the address and lower-cases its domain, and reports
whether it is one.
*/
func normalizeEmail(raw string) (string, bool) {
	email := strings.TrimSpace(raw)
	if email == "" || len(email) > maxEmailLength {
		return email, false
	}

	parsed, err := fmail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Email != email {
		return email, false
	}

	at := strings.LastIndex(email, "@")

	return email[:at] + strings.ToLower(email[at:]), true
}
