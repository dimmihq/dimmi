/*
|--------------------------------------------------------------------------
| User Account Commands
|--------------------------------------------------------------------------
|
| Rescues the operator of a server performs for people who cannot help
| themselves through the app, usually because the server sends no mail:
| a password reset link printed instead of mailed, or a password set
| directly; an email vouched for; everyone signed out after a lost
| laptop. Each one changes the account the way the app would.
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

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
findUser returns the account with the email the operator typed.
*/
func findUser(ctx context.Context, a *app.App, typed string) (models.User, error) {
	email := strings.ToLower(strings.TrimSpace(typed))

	user, err := models.New(a.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return user, fmt.Errorf("%w: %s", ErrUserNotFound, email)
	}

	if err != nil {
		return user, fmt.Errorf("find user: %w", err)
	}

	return user, nil
}

/*
UserResetPasswordCommand prints a password reset link, or sets the
password directly.
*/
type UserResetPasswordCommand struct {
	App *app.App
}

/*
Name returns user:reset-password.
*/
func (c *UserResetPasswordCommand) Name() string {
	return "user:reset-password"
}

/*
Description says what the command does.
*/
func (c *UserResetPasswordCommand) Description() string {
	return "Print a password reset link, or set a password and sign the user out"
}

/*
Flags declares the email and the optional password to set.
*/
func (c *UserResetPasswordCommand) Flags(flags *console.Flags) {
	flags.Argument("email", "the email of the account")
	flags.String("password", "", "set this password now instead of printing a link")
}

/*
Handle prints the link, or sets the password.
*/
func (c *UserResetPasswordCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	user, err := findUser(ctx, c.App, in.Argument("email"))
	if err != nil {
		return err
	}

	if password := in.String("password"); password != "" {
		if err := c.App.Actions.OperatorAccounts.SetPassword(ctx, user, password); err != nil {
			return err
		}

		out.Info("The password of %s is set, and they are signed out everywhere.", user.Email)

		return nil
	}

	link, err := c.App.Actions.OperatorAccounts.ResetLink(ctx, user)
	if err != nil {
		return err
	}

	out.Line("%s", link)
	out.Info("The link works for 60 minutes. Only the newest link works.")

	return nil
}

/*
UserVerifyEmailCommand marks an account's email as proven.
*/
type UserVerifyEmailCommand struct {
	App *app.App
}

/*
Name returns user:verify-email.
*/
func (c *UserVerifyEmailCommand) Name() string {
	return "user:verify-email"
}

/*
Description says what the command does.
*/
func (c *UserVerifyEmailCommand) Description() string {
	return "Mark a user's email as verified"
}

/*
Flags declares the email.
*/
func (c *UserVerifyEmailCommand) Flags(flags *console.Flags) {
	flags.Argument("email", "the email of the account")
}

/*
Handle verifies the email.
*/
func (c *UserVerifyEmailCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	user, err := findUser(ctx, c.App, in.Argument("email"))
	if err != nil {
		return err
	}

	if err := c.App.Actions.OperatorAccounts.Verify(ctx, user); err != nil {
		return err
	}

	out.Info("%s is verified.", user.Email)

	return nil
}

/*
UserLogoutCommand signs an account out of every browser and app.
*/
type UserLogoutCommand struct {
	App *app.App
}

/*
Name returns user:logout.
*/
func (c *UserLogoutCommand) Name() string {
	return "user:logout"
}

/*
Description says what the command does.
*/
func (c *UserLogoutCommand) Description() string {
	return "Sign a user out of every browser and app"
}

/*
Flags declares the email.
*/
func (c *UserLogoutCommand) Flags(flags *console.Flags) {
	flags.Argument("email", "the email of the account")
}

/*
Handle signs the user out.
*/
func (c *UserLogoutCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	user, err := findUser(ctx, c.App, in.Argument("email"))
	if err != nil {
		return err
	}

	if err := c.App.Actions.OperatorAccounts.SignOut(ctx, user); err != nil {
		return err
	}

	out.Info("%s is signed out everywhere. Their access tokens still work.", user.Email)

	return nil
}
