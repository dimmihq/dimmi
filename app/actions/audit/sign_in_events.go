/*
|--------------------------------------------------------------------------
| Account Audit Events
|--------------------------------------------------------------------------
|
| A teammate's own account, how they sign in and how they protect it,
| decides who can act in every workspace they belong to. The workspace's
| admins therefore see these account events of their members, and every
| export of the log leaves its own entry.
|
*/

package audit

/*
The audit events of members' sign-ins and account security, and of
exports of the audit log.
*/
const (
	EventMemberSignInFailed      = "member.sign_in_failed"
	EventMemberSignInLocked      = "member.sign_in_locked"
	EventMemberSignedIn          = "member.signed_in"
	EventMemberTwoFactorEnabled  = "member.two_factor_enabled"
	EventMemberTwoFactorDisabled = "member.two_factor_disabled"
	EventMemberAuthFactorAdded   = "member.auth_factor_added"
	EventMemberAuthFactorRemoved = "member.auth_factor_removed"
	EventMemberRecoveryCodeUsed  = "member.recovery_code_used"
	EventMemberPasswordChanged   = "member.password_changed"
	EventMemberPasswordReset     = "member.password_reset"
	EventMemberEmailChanged      = "member.email_changed"
	EventMemberIdentityLinked    = "member.identity_linked"
	EventMemberIdentityUnlinked  = "member.identity_unlinked"
	EventMemberSessionsRevoked   = "member.sessions_revoked"
	EventAuditLogsExported       = "audit_logs.exported"
)
