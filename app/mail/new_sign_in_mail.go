/*
|--------------------------------------------------------------------------
| New Sign-In Mail
|--------------------------------------------------------------------------
|
| Tells a user their account was signed in from a device it had not seen,
| with where it was, so a stranger's sign-in never goes unnoticed.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
NewSignInMail tells a user their account was signed in from Device at
IP, a device it had not seen.
*/
type NewSignInMail struct {
	Name        string `json:"name"`
	Device      string `json:"device"`
	IP          string `json:"ip"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (NewSignInMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m NewSignInMail) Content(ctx context.Context) (mail.Content, error) {
	view := accounts.NewSignIn(m.Name, deviceName(ctx, m.Device), m.IP, m.SecurityURL)

	return content(ctx, "mail.new_sign_in.subject", view), nil
}
