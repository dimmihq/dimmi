/*
|--------------------------------------------------------------------------
| Personal Access Token Formats
|--------------------------------------------------------------------------
|
| A personal access token starts with dmi_live_, or dmi_test_ when its
| workspace is a test workspace, so a person and a secret scanner can
| tell at a glance what leaked and how much it matters. The framework
| issues and checks the tokens; the two prefixes are the product's. Both
| share one table, where row-level security hides each token outside its
| workspace. A token names its workspace only once it is found, so the
| lookup finds the workspace first through a security definer function
| and then reads the token inside it.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The token prefixes.
*/
const (
	LiveTokenPrefix = "dmi_live_"
	TestTokenPrefix = "dmi_test_"
)

/*
PersonalTokens issues and checks personal access tokens of both kinds.
*/
type PersonalTokens struct {
	Live *token.Tokens
	Test *token.Tokens
}

/*
NewPersonalTokens returns the tokens of both prefixes on the shared
table, and an error when a prefix is not one tokens can carry.
*/
func NewPersonalTokens(db *database.DB, clk clock.Clock) (*PersonalTokens, error) {
	store := workspaceTokenStore{PostgresStore: token.NewPostgresStore(db, token.DefaultTable), db: db}

	live, liveErr := token.New(store, clk, token.Options{Prefix: LiveTokenPrefix})
	test, testErr := token.New(store, clk, token.Options{Prefix: TestTokenPrefix})

	return &PersonalTokens{Live: live, Test: test}, errors.Join(liveErr, testErr)
}

/*
workspaceTokenStore is the token table seen through its workspaces.
Every other method of the store runs in the caller's workspace.
*/
type workspaceTokenStore struct {
	*token.PostgresStore

	db *database.DB
}

/*
FindByHash finds the token's workspace and reads the token inside it.
*/
func (s workspaceTokenStore) FindByHash(ctx context.Context, hash []byte) (token.Token, error) {
	workspaceID, err := models.New(s.db).FindTokenWorkspace(ctx, hash)
	if errors.Is(err, database.ErrNotFound) {
		return token.Token{}, token.ErrNotFound
	}

	if err != nil {
		return token.Token{}, fmt.Errorf("find token workspace: %w", err)
	}

	var found token.Token

	err = s.db.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, _ *database.Tx) error {
		found, err = s.PostgresStore.FindByHash(ctx, hash)

		return err
	})

	return found, err
}
