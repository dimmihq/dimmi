/*
|--------------------------------------------------------------------------
| Manage Invitations
|--------------------------------------------------------------------------
|
| Admins send an invitation again when the mail went astray or the link
| ran out, and withdraw one sent by mistake. Sending again gives a fresh link and a fresh week, at most a
| few times a day per invitation, so nobody can be flooded with mail. An
| answered invitation, accepted or withdrawn, stays as it is.
|
*/

package invitations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
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
Limits on sending one invitation again: how many times in how long.
*/
const (
	maxDeliveries   = 3
	deliveryWindow  = 24 * time.Hour
	deliveriesLimit = "invitations:deliveries:"
)

/*
ChangeInvitationInput is who changes an invitation, through their
membership and account, and which one. LinkOnly sends it again without
mail: the fresh link only comes back to pass on by hand.
*/
type ChangeInvitationInput struct {
	Membership   authz.Membership
	Caller       models.User
	InvitationID ids.UUID
	LinkOnly     bool
}

/*
ResendInvitation sends a pending or expired invitation again with a
fresh link, valid for the invitation period from now, by mail or, with
LinkOnly, only back to the caller; the old link stops working either
way.

Rules:
  - The caller needs members:write.
  - Accepted and withdrawn invitations cannot be sent again.
  - One invitation is sent at most three times a day.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidTransition], [exceptions.ErrTooManyAttempts], or a
wrapped error.
*/
type ResendInvitation struct {
	DB       *database.DB
	Limiter  *ratelimit.Limiter
	Delivery Delivery
}

/*
Handle sends the invitation again and returns it as it went out.
*/
func (a *ResendInvitation) Handle(ctx context.Context, in ChangeInvitationInput) (Sent, error) {
	if !in.Membership.Can("members:write") {
		return Sent{}, exceptions.ErrForbidden
	}

	var sent Sent

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inv, err := lockAnswerable(ctx, q, in)
		if err != nil {
			return err
		}

		if err := a.count(ctx, inv.ID); err != nil {
			return err
		}

		l := a.Delivery.issue()

		again, err := q.SendInvitationAgain(ctx, models.SendInvitationAgainParams{
			WorkspaceID: inv.WorkspaceID, ID: inv.ID, TokenHash: l.digest, ExpiresAt: l.expires, Role: inv.Role,
			TeamIds: inv.TeamIds, Now: a.Delivery.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("send invitation again: %w", err)
		}

		sent, err = a.Delivery.send(ctx, tx, outgoing{in.Membership.Workspace, again, in.Caller.Name, l, in.LinkOnly})
		if err != nil {
			return err
		}

		sent.InviterName, err = inviterName(ctx, q, again)

		return err
	})

	return sent, err
}

/*
count counts one sending of the invitation, refusing it past the daily
limit.
*/
func (a *ResendInvitation) count(ctx context.Context, id ids.UUID) error {
	res, err := a.Limiter.Attempt(ctx, deliveriesLimit+id.String(), maxDeliveries, deliveryWindow)
	if err != nil {
		return appauth.Unavailable(ctx, err)
	}

	if !res.Allowed {
		return exceptions.ErrTooManyAttempts.With("retry_after", max(int(res.RetryAfter.Seconds()), 1))
	}

	return nil
}

/*
RevokeInvitation withdraws a pending invitation; its link then says it
was withdrawn.

Rules:
  - The caller needs members:write.
  - Only an invitation still waiting for an answer, expired or not, can
    be withdrawn.

Events: [appevents.InvitationRevoked].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidTransition], or a wrapped error.
*/
type RevokeInvitation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle withdraws the invitation.
*/
func (a *RevokeInvitation) Handle(ctx context.Context, in ChangeInvitationInput) error {
	if !in.Membership.Can("members:write") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inv, err := lockAnswerable(ctx, q, in)
		if err != nil {
			return err
		}

		if inv.Status != string(enums.InvitationStatusPending) {
			return transition(inv.Status, enums.InvitationStatusRevoked)
		}

		_, err = q.RevokeInvitation(ctx, models.RevokeInvitationParams{
			WorkspaceID: inv.WorkspaceID, ID: inv.ID, RevokedBy: &in.Membership.Member.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("revoke invitation: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.InvitationRevoked{
			WorkspaceID: inv.WorkspaceID, InvitationID: inv.ID, RevokedBy: in.Membership.Member.ID,
		})
	})
}

/*
lockAnswerable locks the invitation, refusing one already accepted or
withdrawn.
*/
func lockAnswerable(ctx context.Context, q *models.Queries, in ChangeInvitationInput) (models.Invitation, error) {
	inv, err := q.LockInvitation(ctx, models.LockInvitationParams{
		WorkspaceID: in.Membership.Workspace.ID, ID: in.InvitationID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return inv, exceptions.ErrNotFound
	}

	if err != nil {
		return inv, fmt.Errorf("lock invitation: %w", err)
	}

	switch enums.InvitationStatus(inv.Status) {
	case enums.InvitationStatusAccepted, enums.InvitationStatusRevoked:
		return inv, transition(inv.Status, enums.InvitationStatusPending)
	case enums.InvitationStatusPending, enums.InvitationStatusExpired:
		return inv, nil
	default:
		return inv, nil
	}
}

/*
inviterName returns the name of who sent the invitation, "" when they
are gone.
*/
func inviterName(ctx context.Context, q *models.Queries, inv models.Invitation) (string, error) {
	row, err := q.FindInvitationWithInviter(ctx, models.FindInvitationWithInviterParams{
		WorkspaceID: inv.WorkspaceID, ID: inv.ID,
	})
	if err != nil {
		return "", fmt.Errorf("find inviter: %w", err)
	}

	return row.InviterName, nil
}

/*
transition returns the refusal of a move of an invitation from one
status to another.
*/
func transition(from string, to enums.InvitationStatus) error {
	return exceptions.ErrInvalidTransition.OnField("status").With("from", from).With("to", string(to))
}
