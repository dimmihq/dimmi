/*
|--------------------------------------------------------------------------
| Connection Errors
|--------------------------------------------------------------------------
|
| Every driver reports failures with the same sentinels, so the caller
| decides what a person sees without knowing which service answered:
| start again, the person said no, the service is down, or the stored
| credentials no longer work.
|
*/

package oauthclient

import "errors"

/*
Errors from connecting an account.

ErrInvalidState refuses a callback whose state is missing, unknown,
expired, already used, or started by another browser or for another
provider; no code is exchanged. ErrDenied means the person declined at
the provider. ErrExchange means the provider refused the code.
ErrInvalidGrant means the provider refused a refresh token, so the
connection must be made again. ErrProvider covers any other failure the
provider reports or causes, such as an unreachable API. ErrMisconfigured
comes from a provider whose configuration lacks a required option, and
ErrUnsupported from asking a provider for something it does not offer,
such as revoking a token.
*/
var (
	ErrInvalidState  = errors.New("oauthclient: invalid state")
	ErrDenied        = errors.New("oauthclient: connection denied at the provider")
	ErrExchange      = errors.New("oauthclient: code exchange refused")
	ErrInvalidGrant  = errors.New("oauthclient: refresh token refused")
	ErrProvider      = errors.New("oauthclient: provider error")
	ErrMisconfigured = errors.New("oauthclient: provider misconfigured")
	ErrUnsupported   = errors.New("oauthclient: not supported by the provider")
)

/*
ErrFakesNotAllowed is what Fake panics with on a manager whose
configuration does not allow fakes.
*/
var ErrFakesNotAllowed = errors.New("oauthclient: fakes are allowed only in tests")
