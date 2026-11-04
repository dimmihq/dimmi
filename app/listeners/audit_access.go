/*
|--------------------------------------------------------------------------
| Audit Log: Ways In
|--------------------------------------------------------------------------
|
| Who can reach a workspace is as much a part of its history as who
| belongs to it: every access token minted or revoked and every
| invitation sent, accepted or withdrawn gets an entry. A token is named
| by its name, never its secret; an invitation by the address it went
| to.
|
*/

package listeners

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenAccess registers the entries of access tokens and invitations.
*/
func (l *WriteAuditLog) listenAccess(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.tokenCreated)
	events.Listen(d, events.InTx, l.tokenRevoked)
	events.Listen(d, events.InTx, l.invitationCreated)
	events.Listen(d, events.InTx, l.invitationAccepted)
	events.Listen(d, events.InTx, l.invitationRevoked)
}

/*
tokenCreated records a token minted by the member it acts as, with what
it may do.
*/
func (l *WriteAuditLog) tokenCreated(ctx context.Context, e appevents.TokenCreated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTokenCreated, ActorID: e.MemberID,
		Subject: audit.Subject{Type: subjectToken, ID: e.TokenID, Label: e.Name},
		Changes: map[string]any{"abilities": map[string]any{"to": e.Abilities}},
	})
}

/*
tokenRevoked records a token ended, by its owner or by an admin.
*/
func (l *WriteAuditLog) tokenRevoked(ctx context.Context, e appevents.TokenRevoked) error {
	token, err := models.New(l.DB).FindAuditToken(ctx, e.TokenID)
	if err != nil {
		return fmt.Errorf("find audited token: %w", err)
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTokenRevoked, ActorID: e.RevokedBy,
		Subject: audit.Subject{Type: subjectToken, ID: e.TokenID, Label: token.Name},
		Changes: map[string]any{"member_id": models.PublicID(models.PrefixMember, e.MemberID)},
	})
}

/*
invitationCreated records an invitation sent, with the role it offers.
*/
func (l *WriteAuditLog) invitationCreated(ctx context.Context, e appevents.InvitationCreated) error {
	return l.invitation(ctx, e.WorkspaceID, e.InvitationID, audit.Entry{
		Event: audit.EventInvitationCreated, ActorID: e.InvitedBy,
		Changes: map[string]any{"role": map[string]any{"to": e.Role}},
	})
}

/*
invitationAccepted records an invitation taken up, as the act of the
member it made.
*/
func (l *WriteAuditLog) invitationAccepted(ctx context.Context, e appevents.InvitationAccepted) error {
	return l.invitation(ctx, e.WorkspaceID, e.InvitationID, audit.Entry{
		Event: audit.EventInvitationAccepted, ActorID: e.MemberID,
		Changes: map[string]any{"member_id": models.PublicID(models.PrefixMember, e.MemberID)},
	})
}

/*
invitationRevoked records an invitation withdrawn.
*/
func (l *WriteAuditLog) invitationRevoked(ctx context.Context, e appevents.InvitationRevoked) error {
	return l.invitation(ctx, e.WorkspaceID, e.InvitationID, audit.Entry{
		Event: audit.EventInvitationRevoked, ActorID: e.RevokedBy,
	})
}

/*
invitation records the entry about the invitation, labeled with the
address it went to.
*/
func (l *WriteAuditLog) invitation(ctx context.Context, workspaceID, id ids.UUID, entry audit.Entry) error {
	row, err := models.New(l.DB).FindInvitationWithInviter(ctx, models.FindInvitationWithInviterParams{
		WorkspaceID: workspaceID, ID: id,
	})
	if err != nil {
		return fmt.Errorf("find audited invitation: %w", err)
	}

	entry.WorkspaceID = workspaceID
	entry.Subject = audit.Subject{Type: subjectInvitation, ID: id, Label: row.Invitation.Email}

	return l.record(ctx, entry)
}
