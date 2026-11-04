/*
|--------------------------------------------------------------------------
| Guards
|--------------------------------------------------------------------------
|
| Two guards decide who is calling. The session guard reads the browser's
| session, which the session middleware loaded; the token guard reads a
| bearer token. Both load the account behind the credential on every
| request, so a deleted account stops working at once, and a token stops
| as soon as its member is deactivated.
|
| A token that is not a personal access token belongs to another guard,
| so the token guard steps aside and the bearer guard asks the next one.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SessionGuard authenticates the user of the browser's session.
*/
type SessionGuard struct {
	DB *database.DB
}

var _ fauth.Guard = (*SessionGuard)(nil)

/*
Authenticate returns the session's user, or fauth.ErrNoCredentials when
the request has no signed-in session or its account was deleted.
*/
func (g *SessionGuard) Authenticate(r *fhttp.Request) (fauth.Principal, error) {
	s, ok := session.From(r.Context())
	if !ok {
		return nil, fauth.ErrNoCredentials
	}

	userID, ok := s.UserID()
	if !ok {
		return nil, fauth.ErrNoCredentials
	}

	user, err := liveUser(r.Context(), g.DB, userID)
	if err != nil {
		return nil, err
	}

	return &Principal{
		User:            user,
		Kind:            KindSession,
		CredentialID:    s.ID(),
		Granted:         []string{AllAbilities},
		SignIn:          SignInOf(s),
		AuthenticatedAt: s.AuthenticatedAt(),
		Confirmed:       s.ConfirmedAt(),
		IP:              ClientAddr(r),
		LastActivityAt:  WorkedAt(s.Record()),
		SSOConnectionID: s.Record().ConnectionID,
		AuthDomain:      s.Record().AuthDomain,
	}, nil
}

/*
TokenGuard resolves personal access tokens.
*/
type TokenGuard struct {
	DB     *database.DB
	Tokens *PersonalTokens
}

var _ fauth.TokenResolver = (*TokenGuard)(nil)

/*
Resolve returns the member a personal access token acts as. A token
whose member is no longer active, or whose account was deleted, is
refused as if it were unknown.
*/
func (g *TokenGuard) Resolve(r *fhttp.Request, plain string) (fauth.Principal, error) {
	tokens, ok := g.tokensFor(plain)
	if !ok {
		return nil, fauth.ErrNoCredentials
	}

	tok, err := tokens.Find(r.Context(), plain)
	if err != nil {
		return nil, err
	}

	member, minted, err := tokenMember(r.Context(), g.DB, tok)
	if err != nil {
		return nil, err
	}

	user, err := liveUser(r.Context(), g.DB, member.UserID)
	if errors.Is(err, fauth.ErrNoCredentials) {
		return nil, fauth.ErrInvalidToken
	}

	if err != nil {
		return nil, err
	}

	if err := tokens.Use(r.Context(), tok, fhttp.ClientIP(r)); err != nil {
		return nil, err
	}

	return &Principal{
		User: user, Kind: KindToken, CredentialID: tok.ID, WorkspaceID: member.WorkspaceID,
		MemberID: member.ID, Granted: tok.Abilities, MintedAsRole: minted, IP: ClientAddr(r),
	}, nil
}

/*
tokensFor returns the tokens whose prefix the plain token carries.
*/
func (g *TokenGuard) tokensFor(plain string) (*token.Tokens, bool) {
	switch {
	case strings.HasPrefix(plain, LiveTokenPrefix):
		return g.Tokens.Live, true
	case strings.HasPrefix(plain, TestTokenPrefix):
		return g.Tokens.Test, true
	default:
		return nil, false
	}
}

/*
liveUser returns the account, or fauth.ErrNoCredentials when it is gone
or deleted.
*/
func liveUser(ctx context.Context, db *database.DB, id ids.UUID) (models.User, error) {
	user, err := models.New(db).FindUser(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return models.User{}, fauth.ErrNoCredentials
	}

	if err != nil {
		return models.User{}, fmt.Errorf("load signed-in user: %w", err)
	}

	if user.IsDeleted() {
		return models.User{}, fauth.ErrNoCredentials
	}

	return user, nil
}

/*
tokenMember returns the active member a token acts as and the role they
held when it was minted, read inside the token's workspace, or
fauth.ErrInvalidToken.
*/
func tokenMember(ctx context.Context, db *database.DB, tok token.Token) (models.WorkspaceMember, enums.Role, error) {
	workspaceID, err := ids.Parse(tok.TenantID)
	if err != nil {
		return models.WorkspaceMember{}, "", fauth.ErrInvalidToken
	}

	var (
		member models.WorkspaceMember
		minted *string
	)

	err = db.Transaction(tenancy.With(ctx, tok.TenantID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		member, err = q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: workspaceID, ID: tok.OwnerID,
		})
		if err != nil {
			return err
		}

		minted, err = q.FindTokenMintedRole(ctx, models.FindTokenMintedRoleParams{WorkspaceID: workspaceID, ID: tok.ID})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.WorkspaceMember{}, "", fauth.ErrInvalidToken
	}

	if err != nil {
		return models.WorkspaceMember{}, "", fmt.Errorf("load token member: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) {
		return models.WorkspaceMember{}, "", fauth.ErrInvalidToken
	}

	if minted == nil {
		return member, "", nil
	}

	return member, enums.Role(*minted), nil
}
