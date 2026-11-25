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
	"net/netip"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateUserInput is the account to create. PasswordHash is already
hashed, or nil for an account without a password. Verified marks the
email as proven, such as when an invitation link carried it. CreatedVia
is a registered way of creating an account. Terms is the person's
acceptance of the legal terms, nil when they were not asked.
*/
type CreateUserInput struct {
	Name         string
	Email        string
	PasswordHash *string
	Verified     bool
	CreatedVia   string
	Terms        *TermsAcceptance
}

/*
TermsAcceptance is where a person was when they ticked the box that
accepts the legal terms: the address and the browser of the request.
*/
type TermsAcceptance struct {
	IP        netip.Addr
	UserAgent string
}

/*
CreateUser stores a new account.

Rules:
  - The email is trimmed and keeps its case as typed; it is unique
    without regard to case.
  - The name is cleaned of control and invisible characters.
  - An acceptance of the terms is stored with the version in force, the
    time, and the address and browser it came from.

Errors: [exceptions.ErrEmailTaken] when a live account has the address,
or a wrapped database error.
*/
type CreateUser struct {
	DB           *database.DB
	Clock        clock.Clock
	TermsVersion string
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

		user, err = models.New(tx).CreateUser(ctx, a.accepted(in.Terms, models.CreateUserParams{
			Name:            str.Clean(in.Name),
			Email:           mail.CanonicalAddress(in.Email),
			PasswordHash:    in.PasswordHash,
			EmailVerifiedAt: verifiedAt,
			CreatedVia:      &in.CreatedVia,
			Now:             now,
		}))

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

/*
accepted returns the account with the acceptance of the terms written
on it, unchanged without one. An address that does not parse is left
out.
*/
func (a *CreateUser) accepted(terms *TermsAcceptance, account models.CreateUserParams) models.CreateUserParams {
	if terms == nil {
		return account
	}

	account.TermsVersion = &a.TermsVersion
	account.TermsAcceptedAt = &account.Now
	account.TermsAcceptedUserAgent = &terms.UserAgent

	if terms.IP.IsValid() {
		account.TermsAcceptedIp = &terms.IP
	}

	return account
}
