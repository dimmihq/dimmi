/*
|--------------------------------------------------------------------------
| Reader Code Mail
|--------------------------------------------------------------------------
|
| The code a help center reader types to sign in with their email. It
| goes to whatever address the reader typed, so it says only what the
| code is for and that nothing happens without it.
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
ReaderCodeMail gives a reader the Code that signs them in to the
HelpCenter.
*/
type ReaderCodeMail struct {
	HelpCenter string `json:"help_center"`
	Code       string `json:"code"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ReaderCodeMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m ReaderCodeMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.ReaderCode(m.HelpCenter, m.Code))
	c.Subject = i18n.T(ctx, "mail.reader_code.subject", i18n.Args{"code": m.Code, "help_center": m.HelpCenter})

	return c, nil
}
