/*
|--------------------------------------------------------------------------
| Invitation Mail
|--------------------------------------------------------------------------
|
| An invitation reaches someone who may have no account yet, so it says
| plainly who invites them, to which workspace and as what, and until
| when the link works. The link carries its secret after the #, like
| every mailed link. Whoever sent the invitation hears when it is
| accepted.
|
*/

package mail

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
InvitationMail asks someone to join Workspace as Role, from Inviter,
through AcceptURL until ExpiresAt.
*/
type InvitationMail struct {
	Workspace string    `json:"workspace"`
	Inviter   string    `json:"inviter"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	AcceptURL string    `json:"accept_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (InvitationMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail. An invitation whose sender left the
workspace names the workspace as the one inviting.
*/
func (m InvitationMail) Content(ctx context.Context) (mail.Content, error) {
	inviter := m.Inviter
	if inviter == "" {
		inviter = m.Workspace
	}

	role := i18n.T(ctx, "mail.roles."+m.Role, nil)
	body := accounts.Invitation(m.Workspace, inviter, role, day(m.ExpiresAt), m.AcceptURL)

	args := i18n.Args{"workspace": m.Workspace, "inviter": inviter}

	return contentAbout(ctx, "mail.invitation.subject", args, body), nil
}

/*
InvitationAcceptedMail tells Name, who sent an invitation, that Member
joined Workspace, linking its members at MembersURL.
*/
type InvitationAcceptedMail struct {
	Name       string `json:"name"`
	Workspace  string `json:"workspace"`
	Member     string `json:"member"`
	MembersURL string `json:"members_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (InvitationAcceptedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m InvitationAcceptedMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.InvitationAccepted(m.Name, m.Workspace, m.Member, m.MembersURL)
	args := i18n.Args{"workspace": m.Workspace, "member": m.Member}

	return contentAbout(ctx, "mail.invitation_accepted.subject", args, body), nil
}

/*
InvitationPendingMail reminds Name, who invited Email to Workspace on
SentOn, a date already written for them, that nobody answered yet, and
links the workspace's members at MembersURL.
*/
type InvitationPendingMail struct {
	Name       string `json:"name"`
	Workspace  string `json:"workspace"`
	Email      string `json:"email"`
	SentOn     string `json:"sent_on"`
	MembersURL string `json:"members_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (InvitationPendingMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m InvitationPendingMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.InvitationPending(m.Name, m.Workspace, m.Email, m.SentOn, m.MembersURL)
	args := i18n.Args{"workspace": m.Workspace, "email": m.Email}

	return contentAbout(ctx, "mail.invitation_pending.subject", args, body), nil
}
