/*
|--------------------------------------------------------------------------
| Factors
|--------------------------------------------------------------------------
|
| A second factor is anything a user holds besides their password: an
| authenticator app today, a security key or a code by email later. Each
| kind is a Factor registered under its type, so a new kind plugs in
| without the login flow learning its name.
|
| A factor's secret is sealed before it is stored, and checking a code may
| change what is stored: the last time step a one-time password used, the
| recovery codes still left. The caller locks the stored row, verifies,
| and writes the credential back in the same transaction, so two requests
| racing with the same code cannot both pass.
|
*/

package twofactor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Errors verification returns.

ErrInvalidCode means the code is wrong. ErrCodeReused means it was right
but already used, which callers usually answer the same way.
ErrUnknownFactor means no factor is registered for a credential's type.
*/
var (
	ErrInvalidCode   = errors.New("twofactor: invalid code")
	ErrCodeReused    = errors.New("twofactor: code already used")
	ErrUnknownFactor = errors.New("twofactor: unknown factor type")
)

/*
Credential is a user's stored factor as verification sees it.

ID is the stored factor's id and Type the kind of factor. Secret is the
sealed secret, and Counter the last time step a one-time password was
accepted at, so the same code is never accepted twice.
*/
type Credential struct {
	ID      ids.UUID
	Type    string
	Secret  []byte
	Counter int64
}

/*
Factor is one kind of second factor.
*/
type Factor interface {
	/*
		Type returns the kind's name, such as "totp".
	*/
	Type() string

	/*
		Verify checks the input against the credential and updates the
		credential when it passes. It returns ErrInvalidCode or
		ErrCodeReused when it does not.
	*/
	Verify(ctx context.Context, cred *Credential, input string) error
}

/*
Registry holds the factors by type.
*/
type Registry map[string]Factor

/*
NewRegistry returns a registry holding the factors.
*/
func NewRegistry(factors ...Factor) Registry {
	r := Registry{}
	for _, f := range factors {
		r.Register(f)
	}

	return r
}

/*
Register adds the factor, replacing one of the same type.
*/
func (r Registry) Register(f Factor) {
	r[f.Type()] = f
}

/*
Find returns the factor of the type.
*/
func (r Registry) Find(kind string) (Factor, bool) {
	f, ok := r[kind]

	return f, ok
}

/*
Types returns the registered types in alphabetical order.
*/
func (r Registry) Types() []string {
	return slices.Sorted(maps.Keys(r))
}

/*
Verify checks the input with the factor of the credential's type.
*/
func (r Registry) Verify(ctx context.Context, cred *Credential, input string) error {
	f, ok := r.Find(cred.Type)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownFactor, cred.Type)
	}

	return f.Verify(ctx, cred, input)
}
