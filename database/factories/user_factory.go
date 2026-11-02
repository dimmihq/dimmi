/*
|--------------------------------------------------------------------------
| User Factory
|--------------------------------------------------------------------------
|
| Accounts for tests: verified, with the shared test password, and an
| email no other test uses, so tests running side by side never collide.
|
*/

package factories

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/testkit/factory"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Password is the password of every account the factory makes.
*/
const Password = "ada-lovelace-1815"

/*
User returns the account factory. Its accounts are verified and have
Password; states change that.
*/
func User(ctx context.Context, a *app.App) (*factory.Factory[models.User], error) {
	hash, err := a.Hash.Make(ctx, Password)
	if err != nil {
		return nil, fmt.Errorf("hash factory password: %w", err)
	}

	return factory.Define(func(seq int) models.User {
		return models.User{
			Name:            fmt.Sprintf("Ada Lovelace %d", seq),
			Email:           fmt.Sprintf("ada.%d.%s@example.com", seq, strings.ToLower(str.Random(8))),
			PasswordHash:    &hash,
			EmailVerifiedAt: new(a.Clock.Now()),
		}
	}).Persist(func(ctx context.Context, u models.User) (models.User, error) {
		return a.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: u.Name, Email: u.Email, PasswordHash: u.PasswordHash, Verified: u.EmailVerifiedAt != nil,
			CreatedVia: appauth.CreatedViaRegister,
		})
	}), nil
}

/*
WithoutPassword makes an account that signs in only through a provider
or a magic link.
*/
func WithoutPassword(u models.User) models.User {
	u.PasswordHash = nil

	return u
}

/*
Unverified makes an account whose email is not proven.
*/
func Unverified(u models.User) models.User {
	u.EmailVerifiedAt = nil

	return u
}

/*
EnableTwoFactor marks the account as protected by a second factor, the
way activating its first factor does.
*/
func EnableTwoFactor(ctx context.Context, a *app.App, u models.User) (models.User, error) {
	now := a.Clock.Now()

	err := models.New(a.DB).UpdateUserTwoFactorConfirmedAt(ctx, models.UpdateUserTwoFactorConfirmedAtParams{
		ID: u.ID, TwoFactorConfirmedAt: &now, Now: now,
	})
	if err != nil {
		return models.User{}, fmt.Errorf("enable two factor: %w", err)
	}

	u.TwoFactorConfirmedAt = &now

	return u, nil
}

/*
DeleteAccount marks the account deleted, the way deleting it does.
*/
func DeleteAccount(ctx context.Context, a *app.App, u models.User) error {
	err := models.New(a.DB).UpdateUserDeletedAt(ctx, models.UpdateUserDeletedAtParams{ID: u.ID, Now: a.Clock.Now()})
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	return nil
}
