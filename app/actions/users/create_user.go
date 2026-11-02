/*
|--------------------------------------------------------------------------
| Create User
|--------------------------------------------------------------------------
|
| Every way an account comes to exist ends here: signing up, accepting an
| invitation, first-run setup and the command line. One email belongs to
| one live account, compared without regard to case, and the database
| decides when two requests race for the same address. A name is stored
| cleaned of invisible characters, so two names that look alike are
| alike.
|
*/

package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateUserInput is the account to create. PasswordHash is already
hashed, or nil for an account without a password. Verified marks the
email as proven, such as when an invitation link carried it. CreatedVia
is a registered way of creating an account.
*/
type CreateUserInput struct {
	Name         string
	Email        string
	PasswordHash *string
	Verified     bool
	CreatedVia   string
}

/*
CreateUser stores a new account.

Rules:
  - The email is trimmed and keeps its case as typed; it is unique
    without regard to case.
  - The name is cleaned of control and invisible characters.

Errors: [exceptions.ErrEmailTaken] when a live account has the address,
or a wrapped database error.
*/
type CreateUser struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle creates the account, inside the transaction in the context when
there is one.
*/
func (a *CreateUser) Handle(ctx context.Context, in CreateUserInput) (models.User, error) {
	now := a.Clock.Now()

	var verifiedAt *time.Time
	if in.Verified {
		verifiedAt = &now
	}

	var user models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		user, err = models.New(tx).CreateUser(ctx, models.CreateUserParams{
			Name:            str.Clean(in.Name),
			Email:           strings.TrimSpace(in.Email),
			PasswordHash:    in.PasswordHash,
			EmailVerifiedAt: verifiedAt,
			CreatedVia:      &in.CreatedVia,
			Now:             now,
		})

		return err
	})
	if database.IsUniqueViolation(err, "users_email_key") {
		return models.User{}, exceptions.ErrEmailTaken
	}

	if err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
