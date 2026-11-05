/*
|--------------------------------------------------------------------------
| Signing In Around Single Sign-On
|--------------------------------------------------------------------------
|
| When a company requires its identity provider and that provider is
| down, its owners still need a way in. An owner may ask to sign in with
| their password, and second factor when they have one, around it. The
| session lasts an hour and every owner of the company hears about it.
| Anyone else who asks gets exactly the answer of a wrong password, after
| the same hashing, so the request tells a stranger nothing.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
bypassing turns the login policy's answer into whether this sign-in goes
around the identity provider: only when the person asked to and the
policy requires the provider. Any other refusal stands.
*/
func (a *AttemptLogin) bypassing(policy error, in AttemptLoginInput) (bool, error) {
	if policy == nil {
		return false, nil
	}

	if in.SSOBypass && errors.Is(policy, exceptions.ErrSSORequired) {
		return true, nil
	}

	return false, policy
}

/*
ownersOnly keeps the account only when it may go around the identity
provider; anyone else's sign-in then fails like a wrong password.
*/
func (a *AttemptLogin) ownersOnly(ctx context.Context, try signInTry) (signInTry, error) {
	checker, ok := a.LoginPolicy.(appauth.SSOBypassChecker)
	if !ok || try.account == nil {
		try.account = nil

		return try, nil
	}

	organization, owner, err := checker.MayBypass(ctx, try.attempt.Email, try.account.ID)
	if err != nil {
		return try, err
	}

	if !owner {
		try.account = nil

		return try, nil
	}

	try.bypass, try.organization = true, organization

	return try, nil
}

/*
announceBypass mails every owner of the company that one of them signed
in around its identity provider.
*/
func (a *AttemptLogin) announceBypass(ctx context.Context, try signInTry) error {
	owners, err := models.New(a.DB).ListOrganizationOwners(ctx, try.organization)
	if err != nil {
		return fmt.Errorf("list owners to warn of bypass: %w", err)
	}

	seen := map[string]bool{}

	for _, owner := range owners {
		if seen[owner.Email] {
			continue
		}

		seen[owner.Email] = true

		pending := a.Mail.To(mail.Address{Name: owner.Name, Email: owner.Email})
		if owner.Locale != "" {
			pending.Locale(owner.Locale)
		}

		if err := pending.Queue(ctx, appmail.SSOBypassUsedMail{
			Name: owner.Name, Owner: try.account.Name, SettingsURL: a.Links.SingleSignOn(),
		}); err != nil {
			return fmt.Errorf("queue bypass warning: %w", err)
		}
	}

	return nil
}
