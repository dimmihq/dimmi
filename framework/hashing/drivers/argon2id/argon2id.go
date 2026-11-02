/*
|--------------------------------------------------------------------------
| Argon2id Hasher
|--------------------------------------------------------------------------
|
| Argon2id is the default password hash: it costs memory as well as CPU,
| which makes guessing on graphics cards expensive. Hashes are standard
| PHC strings ($argon2id$v=19$m=...,t=...,p=...$salt$hash), so any other
| Argon2 implementation can verify them. The defaults follow the OWASP
| recommendation of 19 MiB, two passes and one lane; tests configure a
| far lower cost.
|
*/

package argon2id

import (
	"context"
	"errors"
	"fmt"

	phc "github.com/alexedwards/argon2id"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
ErrInvalidOptions is returned for parameters Argon2id cannot run with.
*/
var ErrInvalidOptions = errors.New("argon2id: invalid options")

/*
Options are the Argon2id cost parameters.

Memory is in KiB and must be at least 8 KiB per lane. Iterations is the
number of passes and Parallelism the number of lanes, both at least one.
SaltLength and KeyLength are in bytes.
*/
type Options struct {
	Memory      uint32 `default:"19456" koanf:"memory"`
	Iterations  uint32 `default:"2"     koanf:"iterations"`
	Parallelism uint8  `default:"1"     koanf:"parallelism"`
	SaltLength  uint32 `default:"16"    koanf:"salt_length"`
	KeyLength   uint32 `default:"32"    koanf:"key_length"`
}

/*
Hasher hashes passwords with Argon2id.
*/
type Hasher struct {
	params phc.Params
}

var _ hashing.Hasher = (*Hasher)(nil)

/*
Factory builds Argon2id hashers for the hasher manager.
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
	if opts.Iterations < 1 || opts.Parallelism < 1 || opts.Memory < 8*uint32(opts.Parallelism) {
		return nil, ErrInvalidOptions
	}

	if opts.SaltLength < 8 || opts.KeyLength < 16 {
		return nil, ErrInvalidOptions
	}

	return &Hasher{params: phc.Params{
		Memory:      opts.Memory,
		Iterations:  opts.Iterations,
		Parallelism: opts.Parallelism,
		SaltLength:  opts.SaltLength,
		KeyLength:   opts.KeyLength,
	}}, nil
}

/*
Make hashes the password with a fresh random salt.
*/
func (h *Hasher) Make(_ context.Context, password string) (string, error) {
	hash, err := phc.CreateHash(password, &h.params)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return hash, nil
}

/*
Check reports whether the password matches the hash. The hash's own
parameters are used, so hashes made under older settings still verify.
*/
func (h *Hasher) Check(_ context.Context, password, hash string) (bool, error) {
	ok, err := phc.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("%w: %w", hashing.ErrInvalidHash, err)
	}

	return ok, nil
}

/*
NeedsRehash reports whether the hash is unreadable or was made with
other parameters than the current ones.
*/
func (h *Hasher) NeedsRehash(hash string) bool {
	params, salt, key, err := phc.DecodeHash(hash)
	if err != nil {
		return true
	}

	return params.Memory != h.params.Memory ||
		params.Iterations != h.params.Iterations ||
		params.Parallelism != h.params.Parallelism ||
		len(salt) != int(h.params.SaltLength) ||
		len(key) != int(h.params.KeyLength)
}
