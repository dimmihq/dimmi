/*
|--------------------------------------------------------------------------
| Me Resource
|--------------------------------------------------------------------------
|
| The answer to "who am I": the user, the workspace in play and what the
| caller may do there, how the credential was earned, and facts about
| the server. Clients decide what to show from member.permissions, never
| from the role, so the meaning of a role can change without breaking
| any app.
|
*/

package resources

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Me is the caller's picture. Workspace and Member are null when no
workspace is in play; Workspaces is null for tokens, which cannot
switch. Locale is the language the caller reads in: their own choice,
else the workspace's, else the one the request was served in.
*/
type Me struct {
	Object     string             `json:"object"`
	User       User               `json:"user"`
	Workspace  *Workspace         `json:"workspace"`
	Member     *Member            `json:"member"`
	Auth       Auth               `json:"auth"`
	Workspaces []WorkspaceSummary `json:"workspaces"`
	Instance   Instance           `json:"instance"`
	Locale     string             `json:"locale"`
}

/*
Member is the caller's membership of the workspace in play, with the
permissions they hold there.
*/
type Member struct {
	ID           string   `json:"id"`
	Object       string   `json:"object"`
	Role         string   `json:"role"`
	Status       string   `json:"status"`
	Availability string   `json:"availability"`
	Permissions  []string `json:"permissions"`
}

/*
Auth describes the credential. SessionID is set for sessions and Token
for personal access tokens; the sign-in fields are left out for tokens,
which carry no sign-in.
*/
type Auth struct {
	Type               string        `json:"type"`
	SessionID          string        `json:"session_id,omitempty"`
	Token              *TokenSummary `json:"token,omitempty"`
	Abilities          []string      `json:"abilities"`
	EffectiveAbilities []string      `json:"effective_abilities"`
	AuthMethods        []string      `json:"auth_methods,omitempty"`
	TwoFactorUsed      *bool         `json:"two_factor_used,omitempty"`
	AuthenticatedAt    *time.Time    `json:"authenticated_at,omitempty"`
}

/*
Instance tells a client about the server it talks to. AuditRetention
is how many days audit entries are kept, 0 for ever, so a client never
offers to look further back.
*/
type Instance struct {
	Edition        string    `json:"edition"`
	Version        string    `json:"version"`
	APIVersion     string    `json:"api_version"`
	Region         string    `json:"region"`
	MailConfigured bool      `json:"mail_configured"`
	AuditRetention int       `json:"audit_retention_days"`
	ServerTime     time.Time `json:"server_time"`
}

/*
NewMe shapes the caller's picture.
*/
func NewMe(ctx context.Context, me auth.Me, instance Instance) Me {
	out := Me{
		Object:   "me",
		User:     NewUser(me.Principal.User),
		Auth:     newAuth(me),
		Instance: instance,
		Locale:   i18n.Locale(ctx),
	}

	var workspaceLocale *string
	if me.Workspace != nil {
		workspace := NewWorkspace(*me.Workspace)
		out.Workspace = &workspace
		out.Member = newMember(*me.Member, me.Permissions)
		workspaceLocale = &me.Workspace.Locale
	}

	if locale, ok := appauth.PreferredLocale(i18n.From(ctx), me.Principal.User.Locale, workspaceLocale); ok {
		out.Locale = locale
	}

	if me.Workspaces != nil {
		out.Workspaces = NewWorkspaceSummaries(me.Workspaces)
	}

	return out
}

func newMember(m models.WorkspaceMember, permissions []string) *Member {
	return &Member{
		ID:           models.PublicID(models.PrefixMember, m.ID),
		Object:       "member",
		Role:         m.Role,
		Status:       m.Status,
		Availability: m.Availability,
		Permissions:  nonNil(permissions),
	}
}

func newAuth(me auth.Me) Auth {
	p := me.Principal
	out := Auth{Type: credentialType(p), Abilities: p.Abilities(), EffectiveAbilities: nonNil(me.Abilities)}

	if me.Token != nil {
		out.Token = &TokenSummary{
			ID: models.PublicID(models.PrefixPersonalToken, me.Token.ID), Object: "personal_access_token",
			Name: me.Token.Name, Prefix: me.Token.Hint, ExpiresAt: utc(me.Token.ExpiresAt),
		}
	}

	if p.Kind == appauth.KindSession {
		used := p.SignIn.UsedSecondFactor()
		authenticatedAt := p.AuthenticatedAt.UTC()
		out.SessionID = models.PublicID(models.PrefixSession, p.CredentialID)
		out.AuthMethods = p.SignIn.Methods
		out.TwoFactorUsed = &used
		out.AuthenticatedAt = &authenticatedAt
	}

	return out
}

/*
credentialType names the credential the way clients see it.
*/
func credentialType(p *appauth.Principal) string {
	if p.Kind == appauth.KindToken {
		return "personal_access_token"
	}

	return p.Kind
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
