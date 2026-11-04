/*
|--------------------------------------------------------------------------
| Disable Two-Factor Command
|--------------------------------------------------------------------------
|
| A user who lost their authenticator and their recovery codes cannot
| sign in, and nothing in the app can let them. The operator of the
| server turns two-factor authentication off for them, the same way the
| user would: every factor goes, the owner is mailed, and their app
| sessions end. They then sign in with their first method alone and set
| up a new authenticator. The operator overrides workspaces that require
| a second factor; the user stays out of those until they add one.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/factors"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrUserNotFound means no account has the email the operator typed.
*/
var ErrUserNotFound = errors.New("no account has this email")

/*
UserDisableTwoFactorCommand turns two-factor authentication off for the
account with an email.
*/
type UserDisableTwoFactorCommand struct {
	App *app.App
}

/*
Name returns user:disable-2fa.
*/
func (c *UserDisableTwoFactorCommand) Name() string {
	return "user:disable-2fa"
}

/*
Description says what the command does.
*/
func (c *UserDisableTwoFactorCommand) Description() string {
	return "Turn two-factor authentication off for a user locked out of it"
}

/*
Flags declares the email of the account.
*/
func (c *UserDisableTwoFactorCommand) Flags(flags *console.Flags) {
	flags.Argument("email", "the email of the account")
}

/*
Handle removes every factor of the account and says what changed.
*/
func (c *UserDisableTwoFactorCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	email := strings.ToLower(strings.TrimSpace(in.Argument("email")))

	user, err := models.New(c.App.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrUserNotFound, email)
	}

	if err != nil {
		return fmt.Errorf("find user to disable two-factor: %w", err)
	}

	wasOn, err := c.App.Actions.RemoveAllAuthFactors.Handle(ctx, factors.RemoveAllAuthFactorsInput{
		UserID: user.ID, Client: appauth.Client{UserAgent: "cli"}, KeepGrant: ids.Nil(), Force: true,
	})
	if err != nil {
		return err
	}

	if !wasOn {
		out.Info("Two-factor authentication was already off for %s.", email)

		return nil
	}

	out.Info("Two-factor authentication is off for %s. Their factors are removed and their apps signed out.", email)

	return nil
}
