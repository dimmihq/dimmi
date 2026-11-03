/*
|--------------------------------------------------------------------------
| Suspicious Sign-In Attempts Mail
|--------------------------------------------------------------------------
|
| Warns a user that strangers keep guessing their password, and that
| their own devices still get in while the account is locked for others.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
SuspiciousSignInAttemptsMail warns a user that strangers keep guessing
their password.
*/
type SuspiciousSignInAttemptsMail struct {
	Name        string `json:"name"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (SuspiciousSignInAttemptsMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m SuspiciousSignInAttemptsMail) Content(ctx context.Context) (mail.Content, error) {
	view := accounts.SuspiciousSignInAttempts(m.Name, m.SecurityURL)

	return content(ctx, "mail.suspicious_sign_in_attempts.subject", view), nil
}
