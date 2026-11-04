/*
|--------------------------------------------------------------------------
| Invitation Preview Resources
|--------------------------------------------------------------------------
|
| How one invitation looks to whoever holds its link, and how one more
| sending of an invitation looks to the admin who sent it. The holder of
| a link sees the workspace, who invited them and as what, but only a
| masked address, since links get forwarded.
|
*/

package resources

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
InvitationDelivery is one more sending of an invitation: when it went
out and the invitation as it now is.
*/
type InvitationDelivery struct {
	Object     string     `json:"object"`
	SentAt     time.Time  `json:"sent_at"`
	Invitation Invitation `json:"invitation"`
}

/*
NewInvitationDelivery shapes the sending of an invitation.
*/
func NewInvitationDelivery(sent invitations.Sent) InvitationDelivery {
	return InvitationDelivery{
		Object: "invitation_delivery", SentAt: sent.Invitation.LastSentAt, Invitation: NewSentInvitation(sent),
	}
}

/*
InvitationPreview is an invitation as whoever holds its link sees it.
Inviter is null when who sent it is gone, and HasAccount says whether
an account uses the invited address, so the page asks to sign in or to
sign up.
*/
type InvitationPreview struct {
	Object      string                    `json:"object"`
	Workspace   InvitationPreviewSpace    `json:"workspace"`
	Inviter     *InvitationPreviewInviter `json:"inviter"`
	Role        string                    `json:"role"`
	EmailMasked string                    `json:"email_masked"`
	Status      string                    `json:"status"`
	ExpiresAt   time.Time                 `json:"expires_at"`
	HasAccount  bool                      `json:"has_account"`
}

/*
InvitationPreviewSpace is the workspace an invitation invites to.
*/
type InvitationPreviewSpace struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
InvitationPreviewInviter is who sent an invitation, by name.
*/
type InvitationPreviewInviter struct {
	Name string `json:"name"`
}

/*
NewInvitationPreview shapes the invitation for whoever holds its link.
*/
func NewInvitationPreview(p invitations.Preview) InvitationPreview {
	out := InvitationPreview{
		Object: "invitation_preview",
		Workspace: InvitationPreviewSpace{
			ID: models.PublicID(models.PrefixWorkspace, p.Invitation.WorkspaceID), Name: p.Invitation.WorkspaceName,
		},
		Role:        p.Invitation.Role,
		EmailMasked: MaskEmail(p.Invitation.Email),
		Status:      string(p.Status),
		ExpiresAt:   p.Invitation.ExpiresAt,
		HasAccount:  p.HasAccount,
	}

	if p.Invitation.InviterName != "" {
		out.Inviter = &InvitationPreviewInviter{Name: p.Invitation.InviterName}
	}

	return out
}

/*
MaskEmail keeps the first character of the address and its domain,
"grace@acme.com" becoming "g***@acme.com".
*/
func MaskEmail(email string) string {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" {
		return "***"
	}

	first, _ := utf8.DecodeRuneInString(local)

	return string(first) + "***@" + domain
}
