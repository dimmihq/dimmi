/*
|--------------------------------------------------------------------------
| Bcrypt Hasher
|--------------------------------------------------------------------------
|
| Bcrypt is here for passwords that arrive already hashed, from another
| system being imported. Configured beside the default hasher, it lets
| those users sign in, after which the login flow stores a new hash made
| by the default. It reads every $2a$, $2b$ and $2y$ hash.
|
| Bcrypt only looks at the first 72 bytes of a password, so a longer
| password is refused instead of being silently cut short.
|
*/

package bcrypt

import (
	"context"
	"errors"
	"fmt"

	xbcrypt "golang.org/x/crypto/bcrypt"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
ErrInvalidCost is returned for a cost outside bcrypt's range of 4 to 31.
*/
var ErrInvalidCost = errors.New("bcrypt: cost must be between 4 and 31")

/*
maxPasswordBytes is the longest input bcrypt reads; checking a longer
password would compare only its prefix.
*/
const maxPasswordBytes = 72

/*
Options are the bcrypt parameters. Cost is the base-2 logarithm of the
number of rounds.
*/
type Options struct {
	Cost int `default:"12" koanf:"cost"`
}

/*
Hasher hashes passwords with bcrypt.
*/
type Hasher struct {
	cost int
}

var _ hashing.Hasher = (*Hasher)(nil)

/*
Factory builds bcrypt hashers for the hasher manager.
*/
func Factory(_ string, section config.Section) (hashing.Hasher, error) {
	var opts Options
	if err := section.Decode(&opts); err != nil {
		return nil, err
	}

	return New(opts)
}

/*
New returns a hasher with the given cost.
*/
func New(opts Options) (*Hasher, error) {
	if opts.Cost < xbcrypt.MinCost || opts.Cost > xbcrypt.MaxCost {
		return nil, ErrInvalidCost
	}

	return &Hasher{cost: opts.Cost}, nil
}

/*
Make hashes the password with a fresh random salt.
*/
func (h *Hasher) Make(_ context.Context, password string) (string, error) {
	hash, err := xbcrypt.GenerateFromPassword([]byte(password), h.cost)
	if errors.Is(err, xbcrypt.ErrPasswordTooLong) {
		return "", hashing.ErrPasswordTooLong
	}

	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

/*
Check reports whether the password matches the hash.
*/
func (h *Hasher) Check(_ context.Context, password, hash string) (bool, error) {
	if len(password) > maxPasswordBytes {
		return false, hashing.ErrPasswordTooLong
	}

	err := xbcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, xbcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("%w: %w", hashing.ErrInvalidHash, err)
	}

	return true, nil
}

/*
NeedsRehash reports whether the hash is unreadable or was made at
another cost than the current one.
*/
func (h *Hasher) NeedsRehash(hash string) bool {
	cost, err := xbcrypt.Cost([]byte(hash))

	return err != nil || cost != h.cost
}
