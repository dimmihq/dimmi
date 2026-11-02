/*
|--------------------------------------------------------------------------
| Time-Based One-Time Passwords
|--------------------------------------------------------------------------
|
| An authenticator app and the server share a random seed and derive the
| same six digits from it every thirty seconds. The settings are the ones
| every authenticator app understands: SHA-1, six digits, thirty seconds.
| A code from the step before or after the current one is accepted too,
| so a phone clock that is slightly off still works.
|
| A code must never be accepted twice, or anyone who watched it being
| typed could use it again within the minute. Each accepted step must
| come after the last one stored for the credential, and the step is
| also claimed in the shared cache, so two servers seeing the same code
| at the same moment cannot both accept it before the database catches
| up.
|
| The seed is sealed with the application's encrypter before it is
| stored, and opened only to check a code or show it once at setup.
|
*/

package twofactor

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
)

/*
The one-time password settings: the step length in seconds, the steps
accepted on either side of the current one, the number of digits and
the seed length in bytes.
*/
const (
	period     = 30
	skew       = 1
	digits     = otp.DigitsSix
	seedLength = 20
)

/*
TOTPType is the type time-based one-time passwords register under.
*/
const TOTPType = "totp"

/*
TOTP is the time-based one-time password factor.
*/
type TOTP struct {
	issuer    string
	encrypter encryption.Encrypter
	cache     *cache.Store
	clock     clock.Clock
}

var _ Factor = (*TOTP)(nil)

/*
NewTOTP returns the factor. The issuer names the application in
authenticator apps; the encrypter seals seeds and the cache holds the
steps already used.
*/
func NewTOTP(issuer string, enc encryption.Encrypter, store *cache.Store, clk clock.Clock) *TOTP {
	return &TOTP{issuer: issuer, encrypter: enc, cache: store, clock: clk}
}

/*
Type returns TOTPType.
*/
func (t *TOTP) Type() string {
	return TOTPType
}

/*
NewSecret returns a new random seed, sealed for storage.
*/
func (t *TOTP) NewSecret() ([]byte, error) {
	seed := make([]byte, seedLength)
	_, _ = rand.Read(seed)

	sealed, err := t.encrypter.Encrypt(seed)
	if err != nil {
		return nil, fmt.Errorf("seal totp seed: %w", err)
	}

	return sealed, nil
}

/*
Setup returns what a person needs to add the sealed seed to an
authenticator app: the seed in base32 to type in, and the otpauth URL
for the account to show as a QR code.
*/
func (t *TOTP) Setup(sealed []byte, account string) (secret, url string, err error) {
	seed, err := t.encrypter.Decrypt(sealed)
	if err != nil {
		return "", "", fmt.Errorf("open totp seed: %w", err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      t.issuer,
		AccountName: account,
		Period:      period,
		Secret:      seed,
		Digits:      digits,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("build totp key: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

/*
Verify accepts a code for a step after the credential's counter, claims
the step so no other server accepts it, and moves the counter to it.
*/
func (t *TOTP) Verify(ctx context.Context, cred *Credential, input string) error {
	seed, err := t.encrypter.Decrypt(cred.Secret)
	if err != nil {
		return fmt.Errorf("open totp seed: %w", err)
	}

	step, err := matchingStep(seed, strings.ReplaceAll(input, " ", ""), t.clock.Now(), cred.Counter)
	if err != nil {
		return err
	}

	key := "twofactor:totp:" + cred.ID.String() + ":" + strconv.FormatInt(step, 10)

	claimed, err := t.cache.Add(ctx, key, []byte{1}, (2*skew+2)*period*time.Second)
	if err != nil {
		return fmt.Errorf("claim totp step: %w", err)
	}

	if !claimed {
		return ErrCodeReused
	}

	cred.Counter = step

	return nil
}

/*
matchingStep returns the step within the window whose code is the
input. Every step in the window is computed, so the time taken does not
reveal which one matched. A match at or before the last used step is
ErrCodeReused.
*/
func matchingStep(seed []byte, code string, now time.Time, last int64) (int64, error) {
	if len(code) != digits.Length() {
		return 0, ErrInvalidCode
	}

	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(seed)
	current := now.Unix() / period
	matched := int64(-1)

	for step := current - skew; step <= current+skew; step++ {
		if step < 0 {
			continue
		}

		ok, err := hotp.ValidateCustom(code, uint64(step), secret, hotp.ValidateOpts{
			Digits:    digits,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			return 0, fmt.Errorf("check totp code: %w", err)
		}

		if ok {
			matched = step
		}
	}

	switch {
	case matched < 0:
		return 0, ErrInvalidCode
	case matched <= last:
		return 0, ErrCodeReused
	default:
		return matched, nil
	}
}
