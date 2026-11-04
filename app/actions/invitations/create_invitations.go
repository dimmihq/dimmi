/*
|--------------------------------------------------------------------------
| Create Invitations
|--------------------------------------------------------------------------
|
| Admins invite several people at once, and every address gets its own
| answer: invited, invited again, already a member, or not an address at
| all. One bad address never sinks the rest, and nothing is skipped
| without saying so.
|
| A workspace holds at most one pending invitation per address: inviting
| someone again sends them a fresh link instead of a second invitation.
| Someone whose membership was deactivated can be invited back, and
| accepting brings back their old membership with its history. Owners
| are never invited; ownership is handed over to a member.
|
*/

package invitations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
dailyWindow is the period the workspace's invitation cap counts in.
*/
const dailyWindow = 24 * time.Hour

/*
Result says what became of one address of a batch.
*/
type Result string

/*
The results of inviting one address: a new invitation, a fresh link
for a pending one, nothing for an active member, a new invitation that
brings a deactivated member back, and nothing for an address that is
not one.
*/
const (
	ResultCreated       Result = "created"
	ResultResent        Result = "resent"
	ResultAlreadyMember Result = "already_member"
	ResultReactivation  Result = "reactivation"
	ResultInvalid       Result = "invalid"
)

/*
Invitee is one address to invite, with the role and the teams to join
on accepting.
*/
type Invitee struct {
	Email   string
	Role    enums.Role
	TeamIDs []ids.UUID
}

/*
CreateInvitationsInput is who invites, through their membership of the
workspace, and whom.
*/
type CreateInvitationsInput struct {
	Membership authz.Membership
	Inviter    models.User
	Invitees   []Invitee
}

/*
Outcome is what became of one address: the address as given, the
result, the invitation for a created, resent or reactivation result,
and the reason for an invalid one.
*/
type Outcome struct {
	Email  string
	Result Result
	Sent   *Sent
	Err    error
}

/*
CreateInvitations invites people to the workspace in play.

Rules:
  - The caller needs members:write.
  - A workspace with a daily cap sends no batch that would pass it.
  - Each address is trimmed and its domain lower-cased; one that is not
    an email address is reported invalid.
  - An active member is reported and not invited; a deactivated one is
    invited back.
  - A pending invitation to the address is sent again with a fresh link,
    the role and the teams given.
  - Only teams that exist are kept.

Events: [appevents.InvitationCreated] for each new invitation.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvitationLimitReached],
or a wrapped error.
*/
type CreateInvitations struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Limiter  *ratelimit.Limiter
	Delivery Delivery
	DailyCap int
}

/*
Handle invites everyone in the batch and returns what became of each
address, in the order given.
*/
func (a *CreateInvitations) Handle(ctx context.Context, in CreateInvitationsInput) ([]Outcome, error) {
	if !in.Membership.Can("members:write") {
		return nil, exceptions.ErrForbidden
	}

	if err := a.reserve(ctx, in.Membership.Workspace.ID, len(in.Invitees)); err != nil {
		return nil, err
	}

	outcomes := make([]Outcome, 0, len(in.Invitees))

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, invitee := range in.Invitees {
			outcome, err := a.invite(ctx, tx, in, invitee)
			if err != nil {
				return err
			}

			outcomes = append(outcomes, outcome)
		}

		return nil
	})

	return outcomes, err
}

/*
reserve counts the batch against the workspace's daily cap, refusing it
whole when it would pass the cap.
*/
func (a *CreateInvitations) reserve(ctx context.Context, workspaceID ids.UUID, count int) error {
	if a.DailyCap <= 0 {
		return nil
	}

	key := "invitations:workspace:" + workspaceID.String()

	left, err := a.Limiter.Remaining(ctx, key, a.DailyCap, dailyWindow)
	if err != nil {
		return appauth.Unavailable(ctx, err)
	}

	if left < count {
		wait, _ := a.Limiter.AvailableIn(ctx, key, a.DailyCap, dailyWindow)

		return exceptions.ErrInvitationLimitReached.With("retry_after", max(int(wait.Seconds()), 1))
	}

	for range count {
		if _, err := a.Limiter.Hit(ctx, key, a.DailyCap, dailyWindow); err != nil {
			return appauth.Unavailable(ctx, err)
		}
	}

	return nil
}

/*
invite works out and carries out what one address gets.
*/
func (a *CreateInvitations) invite(
	ctx context.Context, tx *database.Tx, in CreateInvitationsInput, invitee Invitee,
) (Outcome, error) {
	email, ok := normalizeEmail(invitee.Email)
	if !ok {
		return Outcome{Email: invitee.Email, Result: ResultInvalid, Err: exceptions.ErrInvalidInvitationEmail}, nil
	}

	q := models.New(tx)
	workspace := in.Membership.Workspace

	result, err := standing(ctx, q, workspace.ID, email)
	if err != nil || result == ResultAlreadyMember {
		return Outcome{Email: invitee.Email, Result: result}, err
	}

	teams, err := q.ListExistingTeams(ctx, models.ListExistingTeamsParams{
		WorkspaceID: workspace.ID, Ids: invitee.TeamIDs,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("find teams: %w", err)
	}

	invitee.Email, invitee.TeamIDs = email, teams

	sent, resent, err := a.store(ctx, tx, in, invitee)
	if err != nil {
		return Outcome{}, err
	}

	if resent {
		result = ResultResent
	}

	return Outcome{Email: invitee.Email, Result: result, Sent: &sent}, nil
}

/*
store sends a fresh link to the pending invitation to the address, or
creates one, and reports whether it was sent again.
*/
func (a *CreateInvitations) store(
	ctx context.Context, tx *database.Tx, in CreateInvitationsInput, invitee Invitee,
) (Sent, bool, error) {
	q := models.New(tx)
	workspace := in.Membership.Workspace
	l := a.Delivery.issue()

	pending, err := q.LockPendingInvitationForEmail(ctx, models.LockPendingInvitationForEmailParams{
		WorkspaceID: workspace.ID, Email: invitee.Email,
	})
	if err == nil {
		again, err := q.SendInvitationAgain(ctx, models.SendInvitationAgainParams{
			WorkspaceID: workspace.ID, ID: pending.ID, TokenHash: l.digest, ExpiresAt: l.expires,
			Role: string(invitee.Role), TeamIds: invitee.TeamIDs, Now: a.Delivery.Clock.Now(),
		})
		if err != nil {
			return Sent{}, false, fmt.Errorf("send invitation again: %w", err)
		}

		sent, err := a.Delivery.send(ctx, tx, outgoing{workspace, again, in.Inviter.Name, l, false})

		return sent, true, err
	}

	if !errors.Is(err, database.ErrNotFound) {
		return Sent{}, false, fmt.Errorf("find pending invitation: %w", err)
	}

	created, err := q.CreateInvitation(ctx, models.CreateInvitationParams{
		WorkspaceID: workspace.ID, Email: invitee.Email, Role: string(invitee.Role), TeamIds: invitee.TeamIDs,
		InvitedByMemberID: &in.Membership.Member.ID, TokenHash: l.digest, ExpiresAt: l.expires,
		Now: a.Delivery.Clock.Now(),
	})
	if err != nil {
		return Sent{}, false, fmt.Errorf("create invitation: %w", err)
	}

	err = a.Events.Dispatch(ctx, tx, appevents.InvitationCreated{
		WorkspaceID: workspace.ID, InvitationID: created.ID, Role: created.Role, InvitedBy: in.Membership.Member.ID,
	})
	if err != nil {
		return Sent{}, false, err
	}

	sent, err := a.Delivery.send(ctx, tx, outgoing{workspace, created, in.Inviter.Name, l, false})

	return sent, false, err
}

/*
standing returns how the address stands in the workspace: an active
member, a deactivated one to invite back, or someone new.
*/
func standing(ctx context.Context, q *models.Queries, workspaceID ids.UUID, email string) (Result, error) {
	member, err := q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
		WorkspaceID: workspaceID, Email: email,
	})
	if errors.Is(err, database.ErrNotFound) {
		return ResultCreated, nil
	}

	if err != nil {
		return "", fmt.Errorf("find member by email: %w", err)
	}

	if member.Status == string(enums.MemberStatusActive) {
		return ResultAlreadyMember, nil
	}

	return ResultReactivation, nil
}
