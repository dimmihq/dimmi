/*
|--------------------------------------------------------------------------
| Sending Paused Mail
|--------------------------------------------------------------------------
|
| The mail that tells a workspace's owners and admins its sending paused
| because too much of its email bounced or was reported as spam, with
| the numbers and where to resume it.
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
EmailSendingPausedMail tells an owner or admin that Workspace's sending
paused for Reason, bounce_rate or complaint_rate, after Sent emails of
which HardBounced bounced and Complained were reported as spam.
*/
type EmailSendingPausedMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Reason      string `json:"reason"`
	Sent        int64  `json:"sent"`
	HardBounced int64  `json:"hard_bounced"`
	Complained  int64  `json:"complained"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (EmailSendingPausedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the workspace.
*/
func (m EmailSendingPausedMail) Content(ctx context.Context) (mail.Content, error) {
	failed := m.HardBounced
	if m.Reason == "complaint_rate" {
		failed = m.Complained
	}

	c := mail.View(accounts.EmailSendingPaused(m.Name, m.Workspace, m.Reason, m.Sent, failed, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.email_sending_paused.subject", i18n.Args{"workspace": m.Workspace})

	return c, nil
}

/*
RegisterSendingNotices lets the mail be queued that tells admins the
workspace's sending paused.
*/
func RegisterSendingNotices(m *mail.Manager) {
	mail.Register[EmailSendingPausedMail](m, "email_sending_paused")
}
