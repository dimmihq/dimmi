/*
|--------------------------------------------------------------------------
| Token Access
|--------------------------------------------------------------------------
|
| A member sees and manages their own tokens. Seeing and revoking every
| member's tokens takes tokens:manage_any, which admins hold so they can
| cut off a leaked token of someone who is away. Anyone else asking for a
| token that is not theirs is told it does not exist, so ids reveal
| nothing about other members' tokens.
|
*/

package tokens

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ManageAnyPermission lets a member see and revoke every member's tokens.
*/
const ManageAnyPermission = "tokens:manage_any"

/*
Token is a personal access token as listed, with the member it acts as
and never its secret.
*/
type Token = models.ListWorkspaceTokensRow

/*
mayManage reports whether the caller may see and change a token that
acts as the owner.
*/
func mayManage(m authz.Membership, owner ids.UUID) bool {
	return owner == m.Member.ID || m.Can(ManageAnyPermission)
}

/*
findManaged returns the workspace's token with the id when the caller
may manage it, or exceptions.ErrNotFound.
*/
func findManaged(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID) (Token, error) {
	row, err := q.FindWorkspaceToken(ctx, models.FindWorkspaceTokenParams{WorkspaceID: m.Workspace.ID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return Token{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Token{}, fmt.Errorf("find token: %w", err)
	}

	if !mayManage(m, row.OwnerID) {
		return Token{}, exceptions.ErrNotFound
	}

	return Token(row), nil
}

/*
ShowPersonalAccessToken returns one token the caller may manage.

Errors: [exceptions.ErrNotFound], or a wrapped database error.
*/
type ShowPersonalAccessToken struct {
	DB *database.DB
}

/*
Handle returns the token with the id.
*/
func (a *ShowPersonalAccessToken) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (Token, error) {
	var tok Token

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		tok, err = findManaged(ctx, models.New(tx), m, id)

		return err
	})

	return tok, err
}
