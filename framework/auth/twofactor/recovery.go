/*
|--------------------------------------------------------------------------
| Recovery Codes
|--------------------------------------------------------------------------
|
| Recovery codes get a user in when their authenticator is lost. They are
| shown once, when they are made, and each works once. Only SHA-256
| digests are kept, and the list of digests is sealed as well, so neither
| a copy of the table nor the digests alone let anyone log in.
|
| Codes use Crockford's base32 alphabet in lowercase, without the letters
| people confuse with digits, in two groups of five. Spaces, dashes and
| capitals are ignored when a code is typed back.
|
*/

package twofactor

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
RecoveryCodeType is the type recovery codes register under.
*/
const RecoveryCodeType = "recovery_code"

/*
crockford is the lowercase Crockford base32 alphabet. It has 32
characters, so a random byte masked to five bits picks each one with
the same chance.
*/
const crockford = "0123456789abcdefghjkmnpqrstvwxyz"

/*
groupLength is the number of characters on each side of the dash.
*/
const groupLength = 5

/*
RecoveryCodes is the recovery code factor.
*/
type RecoveryCodes struct {
	encrypter encryption.Encrypter
}

var _ Factor = (*RecoveryCodes)(nil)

/*
NewRecoveryCodes returns the factor, sealing code lists with the
encrypter.
*/
func NewRecoveryCodes(enc encryption.Encrypter) *RecoveryCodes {
	return &RecoveryCodes{encrypter: enc}
}

/*
Type returns RecoveryCodeType.
*/
func (rc *RecoveryCodes) Type() string {
	return RecoveryCodeType
}

/*
Generate returns count new codes to show once, and their sealed digests
to store as the credential's secret.
*/
func (rc *RecoveryCodes) Generate(count int) (codes []string, sealed []byte, err error) {
	digests := make([]string, 0, count)
	codes = make([]string, 0, count)

	for range count {
		code := newRecoveryCode()
		codes = append(codes, code)
		digests = append(digests, hashing.SHA256Hex([]byte(normalize(code))))
	}

	sealed, err = rc.seal(digests)
	if err != nil {
		return nil, nil, err
	}

	return codes, sealed, nil
}

/*
Verify accepts a code still in the credential's list and removes it, so
it never works again.
*/
func (rc *RecoveryCodes) Verify(_ context.Context, cred *Credential, input string) error {
	digests, err := rc.open(cred.Secret)
	if err != nil {
		return err
	}

	given := []byte(hashing.SHA256Hex([]byte(normalize(input))))
	match := -1

	for i, digest := range digests {
		if hashing.Equal([]byte(digest), given) {
			match = i
		}
	}

	if match < 0 {
		return ErrInvalidCode
	}

	sealed, err := rc.seal(slices.Delete(digests, match, match+1))
	if err != nil {
		return err
	}

	cred.Secret = sealed

	return nil
}

/*
Remaining returns how many codes of the credential are still unused.
*/
func (rc *RecoveryCodes) Remaining(cred Credential) (int, error) {
	digests, err := rc.open(cred.Secret)
	if err != nil {
		return 0, err
	}

	return len(digests), nil
}

func (rc *RecoveryCodes) seal(digests []string) ([]byte, error) {
	plain, err := json.Marshal(digests)
	if err != nil {
		return nil, fmt.Errorf("encode recovery codes: %w", err)
	}

	sealed, err := rc.encrypter.Encrypt(plain)
	if err != nil {
		return nil, fmt.Errorf("seal recovery codes: %w", err)
	}

	return sealed, nil
}

func (rc *RecoveryCodes) open(sealed []byte) ([]string, error) {
	plain, err := rc.encrypter.Decrypt(sealed)
	if err != nil {
		return nil, fmt.Errorf("open recovery codes: %w", err)
	}

	var digests []string
	if err := json.Unmarshal(plain, &digests); err != nil {
		return nil, fmt.Errorf("decode recovery codes: %w", err)
	}

	return digests, nil
}

/*
newRecoveryCode returns a random code such as "7k2mq-x9df4". The system
random source never fails, so there is no error to return.
*/
func newRecoveryCode() string {
	raw := make([]byte, 2*groupLength)
	_, _ = rand.Read(raw)

	code := make([]byte, 0, len(raw)+1)
	for i, b := range raw {
		if i == groupLength {
			code = append(code, '-')
		}

		code = append(code, crockford[b&31])
	}

	return string(code)
}

func normalize(code string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.ToLower(code))
}
