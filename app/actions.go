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
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/actions/factors"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
)

/*
Actions holds the business actions.
*/
type Actions struct {
	CreateUser              *users.CreateUser
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
	ListMemberships         *workspaces.ListMemberships
	RepairWorkspaces        *workspaces.RepairWorkspaces
	ListMembers             *members.ListMembers
	ShowMember              *members.ShowMember
	UpdateMember            *members.UpdateMember
	LeaveWorkspace          *members.LeaveWorkspace

	CompleteTwoFactorChallenge *auth.CompleteTwoFactorChallenge
	ListAuthFactors            *factors.ListAuthFactors
	AddAuthFactor              *factors.AddAuthFactor
	ActivateAuthFactor         *factors.ActivateAuthFactor
	UpdateAuthFactor           *factors.UpdateAuthFactor
	RemoveAuthFactor           *factors.RemoveAuthFactor
	RemoveAllAuthFactors       *factors.RemoveAllAuthFactors
	RegenerateRecoveryCodes    *factors.RegenerateRecoveryCodes
	DisableTwoFactor           *factors.DisableTwoFactor

	CreateEmailAddress   *email.CreateEmailAddress
	ListEmailAddresses   *email.ListEmailAddresses
	ShowEmailAddress     *email.ShowEmailAddress
	UpdateEmailAddress   *email.UpdateEmailAddress
	DeleteEmailAddress   *email.DeleteEmailAddress
	SendEmailAddressTest *email.SendEmailAddressTest
	ListInboundEmails    *email.ListInboundEmails
	ReceiveInboundEmail  *email.ReceiveInboundEmail
	ProcessInboundEmail  *email.ProcessInboundEmail
	SendEmailMessage     *email.SendEmailMessage
}
