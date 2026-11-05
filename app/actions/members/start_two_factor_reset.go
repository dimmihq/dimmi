/*
|--------------------------------------------------------------------------
| Start Two-Factor Reset
|--------------------------------------------------------------------------
|
| A teammate who lost their phone and their recovery codes asks an admin
| for help. The admin cannot turn the teammate's second factor off; they
| can only mail the teammate a short-lived link that does, to the address
| the teammate proved. The mailbox alone is never enough: the link asks
| for the account's password too, when it has one.
|
| An admin of one company must never weaken an account that also works
| for another company, so a teammate with an active membership in a
| workspace of another organization is refused; they use their recovery
| codes or ask support. Helping an owner takes someone who manages
| owners.
|
*/

package members

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a reset link works, what its secret stores, and how many
resets one member may be sent a day.
*/
const (
	twoFactorResetTTL       = 30 * time.Minute
	twoFactorResetsADay     = 3
	resetDataWorkspaceKey   = "workspace_id"
	resetDataMemberKey      = "member_id"
	resetPermission         = "members:reset_two_factor"
	resetOwnersPermission   = "members:manage_owners"
	twoFactorResetRateScope = "two-factor-reset:"
)

/*
StartTwoFactorResetInput is the admin's membership, the member to help
and the client asking.
*/
type StartTwoFactorResetInput struct {
	Membership authz.Membership
	MemberID   ids.UUID
	Client     appauth.Client
}

/*
TwoFactorReset is a reset link: the secret's id, the member it helps,
and until when it works.
*/
type TwoFactorReset struct {
	ID        ids.UUID
	MemberID  ids.UUID
	ExpiresAt time.Time
}

/*
StartTwoFactorReset mails a teammate the link that turns their second
factor off.

Rules:
  - The caller needs members:reset_two_factor; an owner as target also
    needs members:manage_owners.
  - Nobody helps themselves.
  - The member is active, has two-factor authentication on and a
    verified email.
  - A member with an active membership in another organization's
    workspace is refused.
  - Three links a day per member; a new one ends the one before.
  - The link works for 30 minutes.

Events: [appevents.MemberTwoFactorResetRequested].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrResetSelf], [exceptions.ErrResetTargetInactive],
[exceptions.ErrTwoFactorNotEnabled],
[exceptions.ErrResetTargetUnverified], [exceptions.ErrResetBlocked],
[exceptions.ErrTooManyAttempts], or a wrapped error.
*/
type StartTwoFactorReset struct {
	DB          *database.DB
	Events      *events.Dispatcher
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	Tokens      *appauth.OneTimeTokens
	SecurityLog *appauth.SecurityLog
}

/*
Handle mails the link and returns it.
*/
func (a *StartTwoFactorReset) Handle(ctx context.Context, in StartTwoFactorResetInput) (TwoFactorReset, error) {
	if !in.Membership.Can(resetPermission) {
		return TwoFactorReset{}, exceptions.ErrForbidden
	}

	target, err := findWithUserIn(ctx, a.DB, in.Membership.Workspace.ID, in.MemberID)
	if err != nil {
		return TwoFactorReset{}, err
	}

	if err := a.check(ctx, in, target); err != nil {
		return TwoFactorReset{}, err
	}

	err = appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: twoFactorResetRateScope + target.WorkspaceMember.ID.String(), Limit: twoFactorResetsADay,
		Window: 24 * time.Hour,
	})
	if err != nil {
		return TwoFactorReset{}, err
	}

	var issued appauth.Issued

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		issued, err = a.Tokens.Issue(ctx, appauth.Issue{
			Purpose: enums.OneTimePurposeTwoFactorReset, UserID: &target.User.ID, Email: target.User.Email,
			Data: map[string]any{
				resetDataWorkspaceKey: in.Membership.Workspace.ID.String(),
				resetDataMemberKey:    target.WorkspaceMember.ID.String(),
			},
			Client: in.Client, TTL: twoFactorResetTTL, Live: 1,
		})
		if err != nil {
			return err
		}

		return a.tell(ctx, tx, in, resetLink{target: target, issued: issued})
	})
	if err != nil {
		return TwoFactorReset{}, err
	}

	return TwoFactorReset{ID: issued.ID, MemberID: target.WorkspaceMember.ID, ExpiresAt: issued.ExpiresAt}, nil
}

/*
check refuses a member the caller may not help, or who cannot be helped
this way.
*/
func (a *StartTwoFactorReset) check(
	ctx context.Context, in StartTwoFactorResetInput, target models.FindWorkspaceMemberWithUserRow,
) error {
	member, user := target.WorkspaceMember, target.User

	switch {
	case member.ID == in.Membership.Member.ID:
		return exceptions.ErrResetSelf
	case appauth.IsOwnerRole(enums.Role(member.Role)) && !in.Membership.Can(resetOwnersPermission):
		return exceptions.ErrForbidden
	case member.Status != string(enums.MemberStatusActive) || user.IsDeleted():
		return exceptions.ErrResetTargetInactive
	case !user.TwoFactorEnabled():
		return exceptions.ErrTwoFactorNotEnabled
	case !user.IsVerified():
		return exceptions.ErrResetTargetUnverified
	}

	elsewhere, err := models.New(a.DB).CountUserMembershipsOutsideOrganization(ctx,
		models.CountUserMembershipsOutsideOrganizationParams{
			UserID: user.ID, OrganizationID: in.Membership.Workspace.OrganizationID,
		})
	if err != nil {
		return fmt.Errorf("count memberships in other organizations: %w", err)
	}

	if elsewhere > 0 {
		return exceptions.ErrResetBlocked
	}

	return nil
}

/*
resetLink is a link just issued and the member it helps.
*/
type resetLink struct {
	target models.FindWorkspaceMemberWithUserRow
	issued appauth.Issued
}

/*
tell mails the link to the member, logs it on their account and records
it in the workspace.
*/
func (a *StartTwoFactorReset) tell(
	ctx context.Context, tx *database.Tx, in StartTwoFactorResetInput, link resetLink,
) error {
	target, issued, user := link.target, link.issued, link.target.User

	admin, err := models.New(tx).FindUser(ctx, in.Membership.Member.UserID)
	if err != nil {
		return fmt.Errorf("find admin asking for reset: %w", err)
	}

	err = appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.TwoFactorResetMail{
		Name: user.Name, Admin: admin.Name, Workspace: in.Membership.Workspace.Name,
		URL: a.Links.TwoFactorReset(issued.ID, issued.Token),
	})
	if err != nil {
		return err
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventTwoFactorResetAsked, Client: in.Client,
		Metadata: map[string]any{"workspace": in.Membership.Workspace.Name, "admin": admin.Name},
	})
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.MemberTwoFactorResetRequested{
		WorkspaceID: in.Membership.Workspace.ID, MemberID: target.WorkspaceMember.ID, UserID: user.ID,
		RequestedBy: in.Membership.Member.ID,
	})
}
