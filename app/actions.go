/*
|--------------------------------------------------------------------------
| Actions
|--------------------------------------------------------------------------
|
| Every business action, built once at boot with exactly the services it
| uses. Controllers, jobs and commands call these, never build their own,
| so an action behaves the same whoever runs it.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/framework/mailbox"

	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/actions/factors"
	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/actions/memberimports"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/roles"
	"github.com/dimmihq/dimmi/app/actions/sso"
	"github.com/dimmihq/dimmi/app/actions/ssologin"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/actions/tokens"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
)

/*
Actions holds the business actions.
*/
type Actions struct {
	Widget                  *WidgetActions
	CreateUser              *users.CreateUser
	UpdateProfile           *users.UpdateProfile
	UpdatePassword          *users.UpdatePassword
	UpdateAvatar            *users.UpdateAvatar
	DeleteAvatar            *users.DeleteAvatar
	RequestEmailChange      *users.RequestEmailChange
	ConfirmEmailChange      *users.ConfirmEmailChange
	PreviewAccountDeletion  *members.PreviewAccountDeletion
	DeleteAccount           *users.DeleteAccount
	CreateWorkspace         *workspaces.CreateWorkspace
	StartSession            *auth.StartSession
	CompletePrimaryAuth     *auth.CompletePrimaryAuth
	LoadMe                  *auth.LoadMe
	AttemptLogin            *auth.AttemptLogin
	RegisterUser            *auth.RegisterUser
	ListSessions            *auth.ListSessions
	EndSession              *auth.EndSession
	EndOtherSessions        *auth.EndOtherSessions
	SendEmailVerification   *auth.SendEmailVerification
	ResendEmailVerification *auth.ResendEmailVerification
	VerifyEmail             *auth.VerifyEmail
	SendPasswordResetLink   *auth.SendPasswordResetLink
	ResetPassword           *auth.ResetPassword
	SendMagicLink           *auth.SendMagicLink
	ConsumeMagicLink        *auth.ConsumeMagicLink
	ConfirmUser             *auth.ConfirmUser
	CheckConfirmation       *auth.CheckConfirmation
	SendConfirmCode         *auth.SendConfirmCode
	LinkOAuthIdentity       *auth.LinkOAuthIdentity
	HandleOAuthCallback     *auth.HandleOAuthCallback
	ListOAuthIdentities     *auth.ListOAuthIdentities
	UnlinkOAuthIdentity     *auth.UnlinkOAuthIdentity
	ResolveWorkspace        *workspaces.ResolveWorkspace
	UpdateWorkspace         *workspaces.UpdateWorkspace
	UpdateOnboarding        *workspaces.UpdateOnboarding
	UpdateLogo              *workspaces.UpdateLogo
	DeleteLogo              *workspaces.DeleteLogo
	ListMemberships         *workspaces.ListMemberships
	RepairWorkspaces        *workspaces.RepairWorkspaces
	ListMembers             *members.ListMembers
	ShowMember              *members.ShowMember
	UpdateMember            *members.UpdateMember
	ListAccessTeams         *members.ListAccessTeams
	StartTwoFactorReset     *members.StartTwoFactorReset
	CompleteTwoFactorReset  *members.CompleteTwoFactorReset
	ClearSignInLock         *members.ClearSignInLock
	LeaveWorkspace          *members.LeaveWorkspace
	TrashWorkspace          *workspaces.TrashWorkspace
	DeleteWorkspace         *workspaces.DeleteWorkspace
	RestoreWorkspace        *workspaces.RestoreWorkspace
	EnsureWorkspaceHasOwner *members.EnsureWorkspaceHasOwner
	CreateInvitations       *invitations.CreateInvitations
	ListInvitations         *invitations.ListInvitations
	ResendInvitation        *invitations.ResendInvitation
	RevokeInvitation        *invitations.RevokeInvitation
	PreviewInvitation       *invitations.PreviewInvitation
	AcceptInvitation        *invitations.AcceptInvitation

	CompleteTwoFactorChallenge *auth.CompleteTwoFactorChallenge
	ShowAccessPolicy           *accesspolicy.ShowAccessPolicy
	UpdateAccessPolicy         *accesspolicy.UpdateAccessPolicy
	ClearAccessPolicy          *accesspolicy.ClearAccessPolicy
	ListRoles                  *roles.ListRoles
	ShowRole                   *roles.ShowRole
	CreateRole                 *roles.CreateRole
	UpdateRole                 *roles.UpdateRole
	DeleteRole                 *roles.DeleteRole
	AssignRole                 *roles.AssignRole
	RoleNames                  *roles.RoleNames
	ListPermissions            *roles.ListPermissions
	CreateSSOConnection        *sso.CreateConnection
	UpdateSSOConnection        *sso.UpdateConnection
	ListSSOConnections         *sso.ListConnections
	ShowSSOConnection          *sso.ShowConnection
	DeleteSSOConnection        *sso.DeleteConnection
	OrganizationDomains        *sso.Domains
	SSOOperator                *sso.Operator
	StartSSOSignIn             *ssologin.StartSignIn
	ConsumeSAML                *ssologin.ConsumeSAML
	ConsumeOIDC                *ssologin.ConsumeOIDC
	CompleteSSOLogin           *ssologin.CompleteSSOLogin
	SSOMetadata                *ssologin.Metadata
	StartSSOTest               *ssologin.StartTest
	LookupSSO                  *ssologin.Lookup
	ListAuthFactors            *factors.ListAuthFactors
	AddAuthFactor              *factors.AddAuthFactor
	ActivateAuthFactor         *factors.ActivateAuthFactor
	UpdateAuthFactor           *factors.UpdateAuthFactor
	RemoveAuthFactor           *factors.RemoveAuthFactor
	RemoveAllAuthFactors       *factors.RemoveAllAuthFactors
	RegenerateRecoveryCodes    *factors.RegenerateRecoveryCodes
	DisableTwoFactor           *factors.DisableTwoFactor

	ListTeams        *teams.ListTeams
	ShowTeam         *teams.ShowTeam
	ListMemberTeams  *teams.ListMemberTeams
	CreateTeam       *teams.CreateTeam
	UpdateTeam       *teams.UpdateTeam
	DeleteTeam       *teams.DeleteTeam
	PurgeTeam        *teams.PurgeTeam
	AddTeamMembers   *teams.AddTeamMembers
	RemoveTeamMember *teams.RemoveTeamMember
	ImportMembers    *memberimports.ImportMembers

	RecordAuditLog     *audit.RecordAuditLog
	ListAuditLogs      *audit.ListAuditLogs
	ShowAuditLog       *audit.ShowAuditLog
	ExportAuditLogs    *audit.ExportAuditLogs
	ExportMembers      *members.ExportMembers
	ListSecurityEvents *auth.ListSecurityEvents

	CreatePersonalAccessToken *tokens.CreatePersonalAccessToken
	ListPersonalAccessTokens  *tokens.ListPersonalAccessTokens
	ShowPersonalAccessToken   *tokens.ShowPersonalAccessToken
	RenamePersonalAccessToken *tokens.RenamePersonalAccessToken
	RevokePersonalAccessToken *tokens.RevokePersonalAccessToken
	ListTokenAbilities        *tokens.ListTokenAbilities

	Inbox *InboxActions

	Assignment       *assignment.Assigner
	AssignmentStatus *assignment.ShowAssignmentStatus

	RecordTyping *conversations.RecordTyping

	CreateEmailAddress   *email.CreateEmailAddress
	ListEmailAddresses   *email.ListEmailAddresses
	ShowEmailAddress     *email.ShowEmailAddress
	RecommendGuide       *email.RecommendGuide
	UpdateEmailAddress   *email.UpdateEmailAddress
	DeleteEmailAddress   *email.DeleteEmailAddress
	SendEmailAddressTest *email.SendEmailAddressTest
	ListInboundEmails    *email.ListInboundEmails
	ReceiveInboundEmail  *email.ReceiveInboundEmail
	InboundHealth        *email.InboundHealth
	InboundDoctor        *email.InboundDoctor
	BackfillResend       *email.BackfillResend
	InboundBudget        *mailbox.Budget
	InboundMetrics       *email.InboundMetrics
	ProcessInboundEmail  *email.ProcessInboundEmail
	SendEmailMessage     *email.SendEmailMessage
}
