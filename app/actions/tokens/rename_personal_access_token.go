/*
|--------------------------------------------------------------------------
| Rename Personal Access Token
|--------------------------------------------------------------------------
|
| A token's name is only a label for the people who manage it, such as
| "CI deploy", so it may change at any time without touching what the
| token can do or whether it works.
|
*/

package tokens

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RenamePersonalAccessTokenInput is who asks, through their membership,
the token and its new name.
*/
type RenamePersonalAccessTokenInput struct {
	Membership authz.Membership
	TokenID    ids.UUID
	Name       string
}

/*
RenamePersonalAccessToken gives a token a new name.

Rules:
  - Only a token the caller may manage is found.

Errors: [exceptions.ErrNotFound], or a wrapped database error.
*/
type RenamePersonalAccessToken struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle renames the token and returns it as it now is.
*/
func (a *RenamePersonalAccessToken) Handle(ctx context.Context, in RenamePersonalAccessTokenInput) (Token, error) {
	var tok Token

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := findManaged(ctx, q, in.Membership, in.TokenID)
		if err != nil {
			return err
		}

		_, err = q.RenameWorkspaceToken(ctx, models.RenameWorkspaceTokenParams{
			WorkspaceID: in.Membership.Workspace.ID, ID: found.ID, Name: str.Clean(in.Name), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("rename token: %w", err)
		}

		tok, err = findManaged(ctx, q, in.Membership, found.ID)

		return err
	})

	return tok, err
}
