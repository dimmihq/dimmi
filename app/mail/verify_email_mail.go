/*
|--------------------------------------------------------------------------
| Verify Email Mail
|--------------------------------------------------------------------------
|
| A new account proves its address by opening the link in this mail. It
| works from any device for a day.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
VerifyEmailMail asks a user to prove they read mail at Email by opening
URL.
*/
type VerifyEmailMail struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (VerifyEmailMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m VerifyEmailMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.verify_email.subject", accounts.VerifyEmail(m.Name, m.Email, m.URL)), nil
}
