/*
|--------------------------------------------------------------------------
| Identity Linked Mail
|--------------------------------------------------------------------------
|
| Linking an account at a sign-in provider adds a way into the account,
| whether the user linked it or a provider vouching for the address
| linked it on its own. The owner hears of every one, so a link they did
| not make is noticed and removed.
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
IdentityLinkedMail tells a user an account at Provider, named as people
know it, was linked to theirs, and links their security settings.
*/
type IdentityLinkedMail struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (IdentityLinkedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m IdentityLinkedMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.IdentityLinked(m.Name, m.Provider, m.SecurityURL))
	c.Subject = i18n.T(ctx, "mail.identity_linked.subject", i18n.Args{"provider": m.Provider})

	return c, nil
}
