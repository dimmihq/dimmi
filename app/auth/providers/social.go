/*
|--------------------------------------------------------------------------
| Social Providers
|--------------------------------------------------------------------------
|
| Social accounts such as GitHub and Google sign in through the
| framework's social login, which keeps each attempt's PKCE verifier and
| nonce on the server and checks the state the browser hands back. A
| social driver only adds what the product keeps about the person and
| maps the framework's errors to the provider errors the flow knows.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/dimmihq/dimmi/framework/socialite"
)

/*
social is a provider signed in through the framework's social login
under its key. Identity reads what the product keeps from the
signed-in user.
*/
type social struct {
	key      string
	protocol string
	display  Display
	manager  *socialite.Manager
	identity func(socialite.User) ExternalIdentity
}

/*
Key names the provider.
*/
func (s social) Key() string {
	return s.key
}

/*
Protocol is the protocol the provider speaks.
*/
func (s social) Protocol() string {
	return s.protocol
}

/*
Display is how apps show the provider.
*/
func (s social) Display() Display {
	return s.display
}

/*
AuthorizeURL starts an attempt at the provider's configured callback.
*/
func (s social) AuthorizeURL(ctx context.Context) (Authorization, error) {
	authURL, state, err := s.manager.Redirect(ctx, s.key, "")
	if err != nil {
		return Authorization{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	return Authorization{URL: authURL, State: state}, nil
}

/*
Callback finishes the attempt and reads the person.
*/
func (s social) Callback(ctx context.Context, state string, query url.Values) (ExternalIdentity, error) {
	user, err := s.manager.Callback(ctx, s.key, state, query)

	switch {
	case errors.Is(err, socialite.ErrInvalidState):
		return ExternalIdentity{}, fmt.Errorf("%w: %w", ErrState, err)
	case errors.Is(err, socialite.ErrDenied):
		return ExternalIdentity{}, fmt.Errorf("%w: %w", ErrDenied, err)
	case err != nil:
		return ExternalIdentity{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	identity := s.identity(user)
	identity.Provider = s.key

	return identity, nil
}

/*
baseIdentity reads what every social provider reports.
*/
func baseIdentity(user socialite.User) ExternalIdentity {
	return ExternalIdentity{
		Subject:       user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified && user.Email != "",
		Name:          user.Name,
		AvatarURL:     user.AvatarURL,
	}
}

/*
rawString returns the named string the provider returned about the
person, or "".
*/
func rawString(user socialite.User, key string) string {
	value, _ := user.Raw[key].(string)

	return value
}
