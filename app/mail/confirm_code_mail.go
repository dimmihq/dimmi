/*
|--------------------------------------------------------------------------
| Confirm Code Mail
|--------------------------------------------------------------------------
|
| The code a user without a password types to confirm it is them before
| a sensitive change.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
ConfirmCodeMail gives a user the Code that confirms it is them.
*/
type ConfirmCodeMail struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ConfirmCodeMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m ConfirmCodeMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.confirm_code.subject", accounts.ConfirmCode(m.Name, m.Code)), nil
}
