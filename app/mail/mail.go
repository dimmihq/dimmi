/*
|--------------------------------------------------------------------------
| Account Mail
|--------------------------------------------------------------------------
|
| Every mail about a user's own account goes out queued in the
| transaction of the change it reports, in the user's language, and
| renders when the worker sends it. Links carry their secret after the
| # of the address, which browsers never send to a server, so no log,
| proxy or link scanner ever sees it, and subjects never carry one.
|
| A queued mail stores its fields, never its Go type, under the name it
| is registered with here; renaming one strands mail already waiting.
|
*/

package mail

import (
	"cmp"
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Register lets every account mail be queued.
*/
func Register(m *mail.Manager) {
	mail.Register[VerifyEmailMail](m, "verify_email")
	mail.Register[MagicLinkMail](m, "magic_link")
	mail.Register[ResetPasswordMail](m, "reset_password")
	mail.Register[ConfirmCodeMail](m, "confirm_code")
	mail.Register[PasswordChangedMail](m, "password_changed")
	mail.Register[NewSignInMail](m, "new_sign_in")
	mail.Register[SuspiciousSignInAttemptsMail](m, "suspicious_sign_in_attempts")
}

/*
ToUser addresses a mail to the user, rendered in their language, or the
install's when they chose none.
*/
func ToUser(m *mail.Manager, user models.User) *mail.PendingMail {
	pending := m.To(mail.Address{Name: user.Name, Email: user.Email})
	if user.Locale != nil {
		pending.Locale(*user.Locale)
	}

	return pending
}

/*
content renders the component with the subject translated in the
mail's language.
*/
func content(ctx context.Context, subject string, component view.Component) mail.Content {
	c := mail.View(component)
	c.Subject = i18n.T(ctx, subject, nil)

	return c
}

/*
deviceName returns the device's label, or the reader's words for a
device that gave none, such as a script, so a mail never has a gap
where the device should be.
*/
func deviceName(ctx context.Context, label string) string {
	return cmp.Or(label, i18n.T(ctx, "mail.unknown_device", nil))
}
