/*
|--------------------------------------------------------------------------
| List Personal Access Tokens
|--------------------------------------------------------------------------
|
| A member lists their own tokens, newest first. Revoked tokens stay on
| the list for ninety days, so their owner can see a token ended and
| when it was last used; after that they are gone. Admins may list one
| other member's tokens, or everyone's.
|
*/

package tokens

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokedKeptFor is how long a revoked token stays listed.
*/
const RevokedKeptFor = 90 * 24 * time.Hour

/*
ListPersonalAccessTokensInput is who asks, through their membership,
whose tokens they want and which page. Everyone lists every member's
tokens; otherwise Member names one member, ids.Nil() for the caller.
*/
type ListPersonalAccessTokensInput struct {
	Membership authz.Membership
	Member     ids.UUID
	Everyone   bool
	Offset     int
	Limit      int
}

/*
TokenPage is one page of tokens, and whether more follow.
*/
type TokenPage struct {
	Tokens []Token
	More   bool
}

/*
ListPersonalAccessTokens lists tokens of the caller's workspace.

Rules:
  - Anyone lists their own tokens.
  - Listing another member's tokens, or everyone's, needs
    tokens:manage_any.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListPersonalAccessTokens struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the page of tokens.
*/
func (a *ListPersonalAccessTokens) Handle(ctx context.Context, in ListPersonalAccessTokensInput) (TokenPage, error) {
	owner, err := ownerFilter(in)
	if err != nil {
		return TokenPage{}, err
	}

	var (
		rows   []Token
		filter *ids.UUID
	)

	if owner != ids.Nil() {
		filter = &owner
	}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err = models.New(tx).ListWorkspaceTokens(ctx, models.ListWorkspaceTokensParams{
			WorkspaceID: in.Membership.Workspace.ID, OwnerID: filter,
			RevokedAfter: a.Clock.Now().Add(-RevokedKeptFor),
			PageOffset:   int64(in.Offset), PageLimit: int64(in.Limit + 1),
		})

		return err
	})
	if err != nil {
		return TokenPage{}, fmt.Errorf("list tokens: %w", err)
	}

	if len(rows) > in.Limit {
		return TokenPage{Tokens: rows[:in.Limit], More: true}, nil
	}

	return TokenPage{Tokens: rows}, nil
}

/*
ownerFilter returns whose tokens to list, ids.Nil() for everyone's,
refusing other members' tokens to a caller who may not manage them.
*/
func ownerFilter(in ListPersonalAccessTokensInput) (ids.UUID, error) {
	m := in.Membership

	if in.Everyone && !m.Can(ManageAnyPermission) {
		return ids.Nil(), exceptions.ErrForbidden
	}

	if in.Everyone {
		return ids.Nil(), nil
	}

	owner := cmp.Or(in.Member, m.Member.ID)
	if !mayManage(m, owner) {
		return ids.Nil(), exceptions.ErrForbidden
	}

	return owner, nil
}
