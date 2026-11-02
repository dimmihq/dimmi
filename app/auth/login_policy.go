/*
|--------------------------------------------------------------------------
| Login Policy
|--------------------------------------------------------------------------
|
| Before any secret is checked, every way of signing in asks the login
| policy whether the address may use that method at all. Today every
| method is allowed; the hook exists so that an organization requiring
| single sign-on can later refuse passwords for its domain without
| touching a single sign-in flow. The question is asked first, so a
| refusal costs no password hash and reveals nothing about the account.
|
| A policy that cannot be read fails closed: nothing is verified and the
| client is asked to retry.
|
*/

package auth

import (
	"context"
	"slices"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
LoginPolicy is what an address may sign in with. An empty
AllowedMethods allows every method; RedirectURL, when set, is where the
person must sign in instead.
*/
type LoginPolicy struct {
	AllowedMethods []string
	RedirectURL    string
}

/*
Allows reports whether the method may be used.
*/
func (p LoginPolicy) Allows(method string) bool {
	if p.RedirectURL != "" {
		return false
	}

	return len(p.AllowedMethods) == 0 || slices.Contains(p.AllowedMethods, method)
}

/*
LoginPolicyResolver finds the login policy for an email address.
*/
type LoginPolicyResolver interface {
	/*
		ForEmail returns the policy for the address.
	*/
	ForEmail(ctx context.Context, email string) (LoginPolicy, error)
}

/*
AllowAll is the login policy of an install without single sign-on:
every method, for every address.
*/
type AllowAll struct{}

/*
ForEmail allows every method.
*/
func (AllowAll) ForEmail(context.Context, string) (LoginPolicy, error) {
	return LoginPolicy{}, nil
}

/*
CheckLoginPolicy returns nil when the address may sign in with the
method, exceptions.ErrSSORequired when it may not, and
exceptions.ErrAuthBusy when the policy could not be read.
*/
func CheckLoginPolicy(ctx context.Context, resolver LoginPolicyResolver, email, method string) error {
	policy, err := resolver.ForEmail(ctx, email)
	if err != nil {
		return exceptions.ErrAuthBusy
	}

	if !policy.Allows(method) {
		return exceptions.ErrSSORequired.With("redirect_url", policy.RedirectURL)
	}

	return nil
}
