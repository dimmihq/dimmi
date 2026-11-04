/*
|--------------------------------------------------------------------------
| Account Change Mail
|--------------------------------------------------------------------------
|
| Moving an account to a new address is told to both mailboxes. The new
| one gets the link that proves it, and the old one is warned at the
| request and told again once it is done, with the new address masked, so
| a thief who got into the account can never move it quietly. Deleting an
| account is confirmed to the address it had, the last mail it ever gets.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
EmailChangeVerificationMail sends the link that proves the new address
to that address.
*/
type EmailChangeVerificationMail struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (EmailChangeVerificationMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m EmailChangeVerificationMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.email_change_verification.subject", accounts.EmailChangeVerification(m.Name, m.Email, m.URL)), nil
}

/*
EmailChangeRequestedMail warns the old address about a request to move
the account, naming the new one masked in MaskedEmail.
*/
type EmailChangeRequestedMail struct {
	Name        string `json:"name"`
	MaskedEmail string `json:"masked_email"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (EmailChangeRequestedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m EmailChangeRequestedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.email_change_requested.subject", accounts.EmailChangeRequested(m.Name, m.MaskedEmail, m.SecurityURL)), nil
}

/*
EmailChangedMail tells the old address the account moved away from it.
*/
type EmailChangedMail struct {
	Name        string `json:"name"`
	MaskedEmail string `json:"masked_email"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (EmailChangedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m EmailChangedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.email_changed.subject", accounts.EmailChanged(m.Name, m.MaskedEmail, m.SecurityURL)), nil
}

/*
AccountDeletedMail confirms an account is gone to the address it had.
*/
type AccountDeletedMail struct {
	Name string `json:"name"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (AccountDeletedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m AccountDeletedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.account_deleted.subject", accounts.AccountDeleted(m.Name)), nil
}
