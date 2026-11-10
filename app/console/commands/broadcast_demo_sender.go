//go:build dev

/*
|--------------------------------------------------------------------------
| Broadcast Demo Sender
|--------------------------------------------------------------------------
|
| Email broadcasts leave only from a connected address on a verified
| domain. A development machine cannot publish DNS records, so the demo
| connects hello@acme-demo.test, adds its domain the normal way, with a
| real signing key, and then marks it verified. Mail goes to the local
| mail catcher like any other.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The demo's sending address and its domain.
*/
const (
	demoAddress = "hello@acme-demo.test"
	demoDomain  = "acme-demo.test"
)

/*
demoSender connects the demo address and verifies its domain, keeping
both when they exist already.
*/
func demoSender(ctx context.Context, a *app.App, ws, member ids.UUID) error {
	var m authz.Membership

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, ok, err := slackteam.Membership(ctx, models.New(tx), ws, member)
		if err == nil && !ok {
			err = errors.New("the teammate is not active")
		}

		m = found

		return err
	})
	if err != nil {
		return fmt.Errorf("load membership: %w", err)
	}

	_, err = a.Actions.CreateEmailAddress.Handle(ctx, email.CreateEmailAddressInput{
		Membership: m, Address: demoAddress, DisplayName: "Acme",
	})
	if err != nil && !errors.Is(err, exceptions.ErrAddressInUse) {
		return fmt.Errorf("connect %s: %w", demoAddress, err)
	}

	_, err = (&email.CreateEmailDomain{Domains: a.EmailDomains}).Handle(ctx, email.CreateEmailDomainInput{
		Membership: m, Domain: demoDomain,
	})
	if err != nil && !errors.Is(err, exceptions.ErrDomainExists) {
		return fmt.Errorf("add %s: %w", demoDomain, err)
	}

	_, err = a.DB.Exec(ctx, `UPDATE email_domains SET status = 'verified', updated_at = now()
		WHERE workspace_id = $1 AND domain = $2`, ws, demoDomain)
	if err != nil {
		return fmt.Errorf("verify %s: %w", demoDomain, err)
	}

	return nil
}
