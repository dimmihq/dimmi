/*
|--------------------------------------------------------------------------
| Create Personal Access Token
|--------------------------------------------------------------------------
|
| A member mints a token for a script, the CLI or a coding agent. The
| token acts as the member in this one workspace, with only the ability
| groups they chose, and never more than their role: what it may do is
| worked out again on every request, so a demotion shrinks it at once.
|
| The secret is returned once and only its digest is kept. A token of a
| test workspace carries the dmi_test_ prefix, so a leaked one is known
| to be harmless at a glance. Its owner is mailed, so a token they did
| not make never goes unnoticed.
|
*/

package tokens

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
maxTokenLifetime is the furthest away a token's expiry may be.
*/
const maxTokenLifetime = 5 * 365 * 24 * time.Hour

/*
CreatePersonalAccessTokenInput is the user minting the token, their
membership of the workspace it is bound to, its name, abilities and
expiry, nil for never, and the client asking.
*/
type CreatePersonalAccessTokenInput struct {
	User       models.User
	Membership authz.Membership
	Name       string
	Abilities  []string
	ExpiresAt  *time.Time
	Client     appauth.Client
}

/*
NewToken is a token just minted, with its secret, which is never
available again.
*/
type NewToken struct {
	Token Token
	Plain string
}

/*
CreatePersonalAccessToken mints a token acting as the caller's member.

Rules:
  - At most twenty tokens a user may mint per hour.
  - Every ability is a known ability group, or * for every group.
  - Every group must hold at least one permission of the member's role.
  - The expiry is in the future and at most five years away.
  - A member holds at most the configured number of live tokens.
  - The owner is mailed and the creation recorded in their security log.

Errors: [exceptions.ErrAbilityNotAllowed],
[exceptions.ErrTokenExpiryInvalid], [exceptions.ErrTokenLimitReached],
[exceptions.ErrTooManyAttempts], or a wrapped error.
*/
type CreatePersonalAccessToken struct {
	DB          *database.DB
	Clock       clock.Clock
	Events      *events.Dispatcher
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	Tokens      *appauth.PersonalTokens
	Permissions *appauth.Catalog
	SecurityLog *appauth.SecurityLog
	Config      config.TokensConfig
}

/*
Handle mints the token.
*/
func (a *CreatePersonalAccessToken) Handle(ctx context.Context, in CreatePersonalAccessTokenInput) (NewToken, error) {
	abilities, err := a.abilities(in)
	if err != nil {
		return NewToken{}, err
	}

	now := a.Clock.Now()
	if in.ExpiresAt != nil && (!in.ExpiresAt.After(now) || in.ExpiresAt.Sub(now) > maxTokenLifetime) {
		return NewToken{}, exceptions.ErrTokenExpiryInvalid
	}

	err = appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "tokens:create:" + in.User.ID.String(), Limit: 20, Window: time.Hour,
	})
	if err != nil {
		return NewToken{}, err
	}

	in.Abilities, in.Name = abilities, str.Clean(in.Name)

	var made NewToken

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		made, err = a.mint(ctx, tx, in)

		return err
	})

	return made, err
}

/*
abilities returns the chosen abilities in catalog order, or * alone,
refusing any that is not a group or that the member's role holds no
permission of.
*/
func (a *CreatePersonalAccessToken) abilities(in CreatePersonalAccessTokenInput) ([]string, error) {
	if slices.Contains(in.Abilities, appauth.AllAbilities) {
		return []string{appauth.AllAbilities}, nil
	}

	allowed := authz.EffectiveAbilities(a.Permissions, in.Membership.Principal.Permissions)

	var refused []string

	for _, ability := range in.Abilities {
		if !slices.Contains(allowed, ability) && !slices.Contains(refused, ability) {
			refused = append(refused, ability)
		}
	}

	if len(refused) > 0 {
		return nil, exceptions.ErrAbilityNotAllowed.With("abilities", refused)
	}

	return slices.DeleteFunc(appauth.AbilityGroups(), func(group string) bool {
		return !slices.Contains(in.Abilities, group)
	}), nil
}

/*
mint checks the member's token count under a lock on the member, stores
the token, and records, announces and mails it.
*/
func (a *CreatePersonalAccessToken) mint(
	ctx context.Context, tx *database.Tx, in CreatePersonalAccessTokenInput,
) (NewToken, error) {
	q := models.New(tx)
	m := in.Membership

	if _, err := q.LockWorkspaceMember(ctx, models.LockWorkspaceMemberParams{
		WorkspaceID: m.Workspace.ID, ID: m.Member.ID,
	}); err != nil {
		return NewToken{}, fmt.Errorf("lock member to mint token: %w", err)
	}

	live, err := q.CountLiveMemberTokens(ctx, models.CountLiveMemberTokensParams{
		WorkspaceID: m.Workspace.ID, OwnerID: m.Member.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return NewToken{}, fmt.Errorf("count member tokens: %w", err)
	}

	if live >= int64(a.Config.MaxPerMember) {
		return NewToken{}, exceptions.ErrTokenLimitReached
	}

	tok, plain, err := a.issuer(m.Workspace).Create(ctx, token.Token{
		OwnerID: m.Member.ID, TenantID: m.Workspace.ID.String(), Name: in.Name, Abilities: in.Abilities,
		ExpiresAt: expiry(in.ExpiresAt),
	})
	if err != nil {
		return NewToken{}, fmt.Errorf("mint token: %w", err)
	}

	stored, err := findManaged(ctx, q, m, tok.ID)
	if err != nil {
		return NewToken{}, err
	}

	return NewToken{Token: stored, Plain: plain}, a.announce(ctx, tx, in, stored)
}

/*
announce records the new token in its owner's security log, dispatches
token.created and mails the owner.
*/
func (a *CreatePersonalAccessToken) announce(
	ctx context.Context, tx *database.Tx, in CreatePersonalAccessTokenInput, tok Token,
) error {
	m := in.Membership
	tokenID := models.PublicID(models.PrefixPersonalToken, tok.ID)

	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventTokenCreated, Client: in.Client,
		Metadata: map[string]any{
			"token_id": tokenID, "name": tok.Name, "abilities": tok.Abilities,
			"workspace_id": models.PublicID(models.PrefixWorkspace, m.Workspace.ID),
		},
	})
	if err != nil {
		return err
	}

	err = a.Events.Dispatch(ctx, tx, appevents.TokenCreated{
		WorkspaceID: m.Workspace.ID, TokenID: tok.ID, MemberID: m.Member.ID, UserID: in.User.ID,
		Name: tok.Name, Abilities: tok.Abilities,
	})
	if err != nil {
		return err
	}

	err = appmail.ToUser(a.Mail, in.User).QueueTx(ctx, tx, appmail.TokenCreatedMail{
		Name: in.User.Name, Token: tok.Name, Workspace: m.Workspace.Name, Abilities: tok.Abilities,
		IP: in.Client.IP, TokensURL: a.Links.Tokens(),
	})
	if err != nil {
		return fmt.Errorf("queue token created mail: %w", err)
	}

	return nil
}

/*
issuer returns the tokens of the workspace's kind: test tokens for a
test workspace, live ones otherwise.
*/
func (a *CreatePersonalAccessToken) issuer(w models.Workspace) *token.Tokens {
	if w.TestMode {
		return a.Tokens.Test
	}

	return a.Tokens.Live
}

/*
expiry returns the expiry as the token store keeps it, the zero time
for never.
*/
func expiry(at *time.Time) time.Time {
	if at == nil {
		return time.Time{}
	}

	return *at
}
