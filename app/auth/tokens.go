/*
|--------------------------------------------------------------------------
| Personal Access Token Formats
|--------------------------------------------------------------------------
|
| A personal access token starts with dmi_live_, or dmi_test_ when its
| workspace is a test workspace, so a person and a secret scanner can
| tell at a glance what leaked and how much it matters. The framework
| issues and checks the tokens; the two prefixes are the product's. Both
| share one table.
|
*/

package auth

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
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
	store := token.NewPostgresStore(db, token.DefaultTable)

	live, liveErr := token.New(store, clk, token.Options{Prefix: LiveTokenPrefix})
	test, testErr := token.New(store, clk, token.Options{Prefix: TestTokenPrefix})

	return &PersonalTokens{Live: live, Test: test}, errors.Join(liveErr, testErr)
}
