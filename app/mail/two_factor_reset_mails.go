/*
|--------------------------------------------------------------------------
| Two-Factor Reset Mail
|--------------------------------------------------------------------------
|
| A teammate who lost their second factor gets the link an admin asked
| for, naming that admin and the workspace, and once they used it, word
| that their account no longer asks for a second factor, with the way to
| set a new one up.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
TwoFactorResetMail sends the link that turns the teammate's second
factor off. Admin is the name of who asked for it, in Workspace.
*/
type TwoFactorResetMail struct {
	Name      string `json:"name"`
	Admin     string `json:"admin"`
	Workspace string `json:"workspace"`
	URL       string `json:"url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TwoFactorResetMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TwoFactorResetMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.two_factor_reset.subject",
		accounts.TwoFactorReset(m.Name, m.Admin, m.Workspace, m.URL)), nil
}

/*
TwoFactorResetCompletedMail tells the teammate the link turned their
second factor off.
*/
type TwoFactorResetCompletedMail struct {
	Name        string `json:"name"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TwoFactorResetCompletedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TwoFactorResetCompletedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.two_factor_reset_completed.subject",
		accounts.TwoFactorResetCompleted(m.Name, m.SecurityURL)), nil
}
