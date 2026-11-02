/*
|--------------------------------------------------------------------------
| Password Hasher
|--------------------------------------------------------------------------
|
| Passwords are never stored, only slow salted hashes of them. A hash
| names its own algorithm and cost, so a stored hash keeps verifying after
| the configuration moves on, and NeedsRehash tells the login flow when to
| store a fresh hash made with the current settings.
|
*/

package hashing

import (
	"context"
	"errors"
)

/*
Errors a hasher returns.

ErrInvalidHash means the stored hash is not in a format the hasher reads.
ErrBusy means too many hashes are already being computed; the caller
should answer "try again" instead of queueing more work. ErrPasswordTooLong
comes from algorithms with an input limit, such as bcrypt's 72 bytes.
*/
var (
	ErrInvalidHash     = errors.New("hashing: invalid hash")
	ErrBusy            = errors.New("hashing: too many hashes in progress")
	ErrPasswordTooLong = errors.New("hashing: password too long")
)

/*
Hasher hashes passwords and checks them against stored hashes.

Make returns a self-describing string holding the algorithm, its
parameters, the salt and the hash. Check reports false for a wrong
password and an error only when the hash cannot be read. Both may wait
for a free slot, which is why they take a context.
*/
type Hasher interface {
	/*
		Make hashes the password with a fresh random salt.
	*/
	Make(ctx context.Context, password string) (string, error)

	/*
		Check reports whether the password matches the hash.
	*/
	Check(ctx context.Context, password, hash string) (bool, error)

	/*
		NeedsRehash reports whether the hash was made with other settings
		than the current ones, or cannot be read at all.
	*/
	NeedsRehash(hash string) bool
}
