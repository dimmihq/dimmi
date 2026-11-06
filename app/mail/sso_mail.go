/*
|--------------------------------------------------------------------------
| Single Sign-On Mail
|--------------------------------------------------------------------------
|
| What owners hear about their company's single sign-on when something
| changes that they did not do themselves, such as a domain losing its
| proof, which quietly stops enforcement until it is fixed.
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
DomainVerificationLostMail tells an owner a Domain lost its proof, and
links the page where they check it again.
*/
type DomainVerificationLostMail struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (DomainVerificationLostMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the domain.
*/
func (m DomainVerificationLostMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.DomainVerificationLost(m.Name, m.Domain, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.domain_verification_lost.subject", i18n.Args{"domain": m.Domain})

	return c, nil
}

/*
SSOBypassUsedMail tells an owner that Owner signed in without the
company's identity provider.
*/
type SSOBypassUsedMail struct {
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (SSOBypassUsedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m SSOBypassUsedMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.SSOBypassUsed(m.Name, m.Owner, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.sso_bypass_used.subject", nil)

	return c, nil
}

/*
SSOCertificateExpiringMail tells an owner the identity provider
certificate of Connection expires on Date.
*/
type SSOCertificateExpiringMail struct {
	Name        string `json:"name"`
	Connection  string `json:"connection"`
	Date        string `json:"date"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (SSOCertificateExpiringMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the connection.
*/
func (m SSOCertificateExpiringMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.SSOCertificateExpiring(m.Name, m.Connection, m.Date, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.sso_certificate_expiring.subject", i18n.Args{"connection": m.Connection})

	return c, nil
}
