/*
|--------------------------------------------------------------------------
| Visitor Code Mail
|--------------------------------------------------------------------------
|
| The code a messenger visitor types to prove they own an email address.
| It goes to whatever address the visitor typed, so it says only what
| the code is for and that nothing happens without it.
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
VisitorCodeMail gives a visitor the Code proving their address to the
Workspace they chat with.
*/
type VisitorCodeMail struct {
	Workspace string `json:"workspace"`
	Code      string `json:"code"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (VisitorCodeMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m VisitorCodeMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.VisitorCode(m.Workspace, m.Code))
	c.Subject = i18n.T(ctx, "mail.visitor_code.subject", i18n.Args{"code": m.Code, "workspace": m.Workspace})

	return c, nil
}
