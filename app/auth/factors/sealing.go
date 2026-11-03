/*
|--------------------------------------------------------------------------
| Factor Sealing
|--------------------------------------------------------------------------
|
| A factor's secret is sealed together with the factor's id, and opens
| only when that id comes back with it. Someone able to write the table
| but not to read the key cannot move one user's authenticator secret
| onto another user's row: the copy fails to open instead of letting
| them in.
|
*/

package factors

import (
	"bytes"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
boundSealer seals values together with the id it is bound to.
*/
type boundSealer struct {
	inner twofactor.Sealer
	bound []byte
}

var _ twofactor.Sealer = boundSealer{}

/*
bind returns a sealer whose values open only for the factor id.
*/
func bind(sealer twofactor.Sealer, id ids.UUID) boundSealer {
	return boundSealer{inner: sealer, bound: []byte(id.String() + ":")}
}

/*
Encrypt seals the plaintext with the bound id in front.
*/
func (b boundSealer) Encrypt(plaintext []byte) ([]byte, error) {
	return b.inner.Encrypt(append(bytes.Clone(b.bound), plaintext...))
}

/*
Decrypt opens the ciphertext, or returns encryption.ErrDecrypt when it
was sealed for another id.
*/
func (b boundSealer) Decrypt(ciphertext []byte) ([]byte, error) {
	plain, err := b.inner.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}

	rest, ok := bytes.CutPrefix(plain, b.bound)
	if !ok {
		return nil, encryption.ErrDecrypt
	}

	return rest, nil
}

/*
KeyPurpose names the key derived from the app key that seals factor
secrets.
*/
const KeyPurpose = "auth-factors"
