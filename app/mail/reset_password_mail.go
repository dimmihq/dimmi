/*
|--------------------------------------------------------------------------
| Reset Password Mail
|--------------------------------------------------------------------------
|
| Lets a person who forgot their password choose a new one, and tells
| them to ignore it if they did not ask.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
ResetPasswordMail lets a user choose a new password at URL.
*/
type ResetPasswordMail struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ResetPasswordMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m ResetPasswordMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.reset_password.subject", accounts.ResetPassword(m.Name, m.URL)), nil
}
