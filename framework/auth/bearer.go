/*
|--------------------------------------------------------------------------
| Bearer Guard
|--------------------------------------------------------------------------
|
| Tokens of different kinds arrive in the same Authorization header:
| personal access tokens, OAuth access tokens, whatever comes next. Each
| kind has a resolver that recognizes its own tokens, usually by prefix,
| and turns one into a principal. The guard hands the token to each
| resolver in turn, so adding a kind of token is registering a resolver,
| not writing a guard.
|
| A resolver also decides what kind of caller the token stands for. One
| OAuth resolver can answer "app" for a first-party grant and "token" for
| any other, so routes keep telling the two apart by the principal's
| Guard alone.
|
*/

package auth

import (
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
TokenResolver turns bearer tokens of one kind into principals.

Resolve returns ErrNoCredentials for a token that is not of its kind,
and a fault such as ErrInvalidToken for one of its kind that does not
check out: unknown, expired or revoked. It receives the request so it
can record where the token was used.
*/
type TokenResolver interface {
	/*
		Resolve returns the caller the token belongs to.
	*/
	Resolve(r *http.Request, token string) (Principal, error)
}

/*
TokenResolverFunc adapts a function to a TokenResolver.
*/
type TokenResolverFunc func(r *http.Request, token string) (Principal, error)

/*
Resolve calls the function.
*/
func (f TokenResolverFunc) Resolve(r *http.Request, token string) (Principal, error) {
	return f(r, token)
}

/*
BearerGuard returns the guard for "Authorization: Bearer" tokens, which
asks each resolver in order. A token no resolver recognizes is invalid.
*/
func BearerGuard(resolvers ...TokenResolver) Guard {
	return GuardFunc(func(r *http.Request) (Principal, error) {
		token, ok := bearerToken(r)
		if !ok {
			return nil, ErrNoCredentials
		}

		if token == "" {
			return nil, ErrInvalidToken
		}

		for _, resolver := range resolvers {
			p, err := resolver.Resolve(r, token)
			if errors.Is(err, ErrNoCredentials) {
				continue
			}

			return p, err
		}

		return nil, ErrInvalidToken
	})
}

/*
bearerToken returns the token of a bearer Authorization header, and
false when the request has no such header.
*/
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	return strings.TrimSpace(token), true
}
