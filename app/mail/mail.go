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
	mail.Register[IdentityLinkedMail](m, "identity_linked")
	mail.Register[TwoFactorEnabledMail](m, "two_factor_enabled")
	mail.Register[TwoFactorDisabledMail](m, "two_factor_disabled")
	mail.Register[RecoveryCodeUsedMail](m, "recovery_code_used")
	mail.Register[TwoFactorRequiredMail](m, "two_factor_required")
	mail.Register[AccessPolicyChangedMail](m, "access_policy_changed")
	registerSSO(m)
	mail.Register[WorkspaceDeletionScheduledMail](m, "workspace_deletion_scheduled")
	mail.Register[WorkspaceDeletionReminderMail](m, "workspace_deletion_reminder")
	mail.Register[WorkspaceRestoredMail](m, "workspace_restored")
	mail.Register[OwnerPromotedBySystemMail](m, "owner_promoted_by_system")
	mail.Register[InvitationMail](m, "invitation")
	mail.Register[InvitationAcceptedMail](m, "invitation_accepted")
	mail.Register[TokenCreatedMail](m, "token_created")
	mail.Register[TokenExpiringMail](m, "token_expiring")
	mail.Register[EmailChangeVerificationMail](m, "email_change_verification")
	mail.Register[EmailChangeRequestedMail](m, "email_change_requested")
	mail.Register[EmailChangedMail](m, "email_changed")
	mail.Register[AccountDeletedMail](m, "account_deleted")
	mail.Register[InvitationPendingMail](m, "invitation_pending")
	mail.Register[TwoFactorResetMail](m, "two_factor_reset")
	mail.Register[TwoFactorResetCompletedMail](m, "two_factor_reset_completed")
}

/*
registerSSO lets the mail about a company's single sign-on be queued.
*/
func registerSSO(m *mail.Manager) {
	mail.Register[DomainVerificationLostMail](m, "domain_verification_lost")
	mail.Register[SSOBypassUsedMail](m, "sso_bypass_used")
	mail.Register[SSOCertificateExpiringMail](m, "sso_certificate_expiring")
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
