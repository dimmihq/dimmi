/*
|--------------------------------------------------------------------------
| Sealed Values
|--------------------------------------------------------------------------
|
| A sealed value is how a secret is stored in a bytea column: credentials,
| tokens, provider keys. It is encoded as JSON and encrypted with its
| scope's data keys, bound to the row it lives in, so a sealed value
| copied into another row or column fails to open. A nil Sealed is a
| NULL column.
|
*/

package encryption

import (
	"context"
	"encoding/json"
	"fmt"
)

/*
Sealed is an encrypted value as stored in a bytea column.
*/
type Sealed []byte

/*
Slot is where a sealed value belongs: the scope whose keys seal it, and
associated data naming its table, column and row.
*/
type Slot struct {
	Scope Scope
	AAD   []byte
}

/*
Seal encodes the value as JSON and encrypts it for the slot.
*/
func Seal[T any](ctx context.Context, keys KeyManager, slot Slot, value T) (Sealed, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode sealed value: %w", err)
	}

	ciphertext, err := keys.Encrypt(ctx, slot.Scope, plaintext, slot.AAD)
	if err != nil {
		return nil, err
	}

	return ciphertext, nil
}

/*
Open decrypts a sealed value given the associated data it was sealed
with, and decodes it.
*/
func Open[T any](ctx context.Context, keys KeyManager, sealed Sealed, aad []byte) (T, error) {
	var value T

	plaintext, err := keys.Decrypt(ctx, sealed, aad)
	if err != nil {
		return value, err
	}

	if err := json.Unmarshal(plaintext, &value); err != nil {
		return value, fmt.Errorf("decode sealed value: %w", err)
	}

	return value, nil
}
