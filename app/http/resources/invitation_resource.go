/*
|--------------------------------------------------------------------------
| Invitation Resources
|--------------------------------------------------------------------------
|
| How invitations look to the admins who send them, one by one and as
| the answers to a batch. The link itself is shown only when it was just
| issued and the server cannot mail it, so the admin can pass it on; it
| is never stored readable, so it can never be shown again.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Invitation is an invitation to the workspace. Status reads expired once
a pending invitation runs out. InvitedBy is who sent it, null once they
are gone, and AcceptURL the link to pass on by hand, null unless it was
just issued on a server that cannot mail it.
*/
type Invitation struct {
	ID         string          `json:"id"`
	Object     string          `json:"object"`
	Email      string          `json:"email"`
	Role       string          `json:"role"`
	Status     string          `json:"status"`
	TeamIDs    []string        `json:"team_ids"`
	ExpiresAt  time.Time       `json:"expires_at"`
	LastSentAt time.Time       `json:"last_sent_at"`
	SendCount  int16           `json:"send_count"`
	InvitedBy  *InvitationUser `json:"invited_by"`
	AcceptURL  *string         `json:"accept_url"`
	CreatedAt  time.Time       `json:"created_at"`
}

/*
InvitationUser is who sent an invitation: their membership and name.
*/
type InvitationUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
NewInvitation shapes the invitation, with the name of who sent it and
its status as of now.
*/
func NewInvitation(inv models.Invitation, inviter, status string) Invitation {
	out := Invitation{
		ID:         models.PublicID(models.PrefixInvitation, inv.ID),
		Object:     "invitation",
		Email:      inv.Email,
		Role:       inv.Role,
		Status:     status,
		TeamIDs:    publicIDs(models.PrefixTeam, inv.TeamIds),
		ExpiresAt:  inv.ExpiresAt,
		LastSentAt: inv.LastSentAt,
		SendCount:  inv.SendCount,
		CreatedAt:  inv.CreatedAt,
	}

	if inv.InvitedByMemberID != nil && inviter != "" {
		out.InvitedBy = &InvitationUser{ID: models.PublicID(models.PrefixMember, *inv.InvitedByMemberID), Name: inviter}
	}

	return out
}

/*
NewSentInvitation shapes an invitation that just went out, with the
link to pass on when the server cannot mail it.
*/
func NewSentInvitation(sent invitations.Sent) Invitation {
	out := NewInvitation(sent.Invitation, sent.InviterName, sent.Invitation.Status)
	if sent.AcceptURL != "" {
		out.AcceptURL = &sent.AcceptURL
	}

	return out
}

/*
NewInvitations shapes a page of invitations, in the order given.
*/
func NewInvitations(rows []models.ListInvitationsRow) []Invitation {
	out := make([]Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewInvitation(row.Invitation, row.InviterName, row.CurrentStatus))
	}

	return out
}

/*
InvitationResult is what became of one address of a batch: the address
as given, the result, the invitation when one went out, and why the
address was refused when it was.
*/
type InvitationResult struct {
	Email      string           `json:"email"`
	Result     string           `json:"result"`
	Invitation *Invitation      `json:"invitation"`
	Error      *InvitationError `json:"error,omitempty"`
}

/*
InvitationError is why one address of a batch was refused.
*/
type InvitationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
NewInvitationResults shapes the outcomes of a batch, in order. Message
turns a refusal into its code and message for the reader.
*/
func NewInvitationResults(outcomes []invitations.Outcome, message func(error) InvitationError) []InvitationResult {
	out := make([]InvitationResult, 0, len(outcomes))

	for _, outcome := range outcomes {
		result := InvitationResult{Email: outcome.Email, Result: string(outcome.Result)}

		if outcome.Sent != nil {
			inv := NewSentInvitation(*outcome.Sent)
			result.Invitation = &inv
		}

		if outcome.Err != nil {
			refusal := message(outcome.Err)
			result.Error = &refusal
		}

		out = append(out, result)
	}

	return out
}

/*
publicIDs returns the public ids of the rows, never nil.
*/
func publicIDs(prefix models.Prefix, rows []ids.UUID) []string {
	out := make([]string, 0, len(rows))
	for _, id := range rows {
		out = append(out, models.PublicID(prefix, id))
	}

	return out
}
