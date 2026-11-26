/*
|--------------------------------------------------------------------------
| Imported Teammate Mails
|--------------------------------------------------------------------------
|
| A teammate brought over from another tool waits, without access, until
| an admin enables them. Admins hear when one of them signs up, so nobody
| waits unseen, and the teammate hears the moment they are let in.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
ImportedTeammateWaitingMail tells an admin an imported teammate signed
up and waits to be enabled, and links the members page.
*/
type ImportedTeammateWaitingMail struct {
	Name       string `json:"name"`
	Workspace  string `json:"workspace"`
	Teammate   string `json:"teammate"`
	Email      string `json:"email"`
	MembersURL string `json:"members_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ImportedTeammateWaitingMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the teammate.
*/
func (m ImportedTeammateWaitingMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.ImportedTeammateWaiting(m.Name, m.Workspace, m.Teammate, m.Email, m.MembersURL))
	c.Subject = i18n.T(ctx, "mail.imported_teammate_waiting.subject",
		i18n.Args{"workspace": m.Workspace, "teammate": m.Teammate})

	return c, nil
}

/*
AccessEnabledMail tells an imported teammate an admin enabled them, and
links the app.
*/
type AccessEnabledMail struct {
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
	AppURL    string `json:"app_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (AccessEnabledMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the workspace.
*/
func (m AccessEnabledMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.AccessEnabled(m.Name, m.Workspace, m.AppURL))
	c.Subject = i18n.T(ctx, "mail.access_enabled.subject", i18n.Args{"workspace": m.Workspace})

	return c, nil
}

/*
registerMembershipMails lets the mails about joining a workspace be
queued: invitations, and teammates brought over from another tool.
*/
func registerMembershipMails(m *mail.Manager) {
	mail.Register[InvitationMail](m, "invitation")
	mail.Register[InvitationAcceptedMail](m, "invitation_accepted")
	mail.Register[ImportedTeammateWaitingMail](m, "imported_teammate_waiting")
	mail.Register[AccessEnabledMail](m, "access_enabled")
}
