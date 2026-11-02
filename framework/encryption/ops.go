/*
|--------------------------------------------------------------------------
| Key Manager
|--------------------------------------------------------------------------
|
| The key manager is how the application encrypts with data keys. Values
| use AES-256-GCM; streams such as backups use AES-GCM-HKDF in segments,
| so a file of any size is encrypted without holding it in memory; MACs
| use HMAC-SHA256 and are deterministic, for lookup hashes and tombstones
| that must match a value without storing it.
|
| Rotation gives a scope new value and stream keys and rewrapping moves
| every stored keyset to the current KEK. MAC keys are never rotated,
| because a rotated MAC key would change every stored lookup hash.
|
*/

package encryption

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/tink-crypto/tink-go/v2/keyset"
)

/*
macSize is the length of a MAC in bytes.
*/
const macSize = 32

/*
KeyManager encrypts with per-scope data keys.
*/
type KeyManager interface {
	/*
		Encrypt seals the plaintext with the scope's keys, bound to the
		associated data, such as the table, column and row it is stored
		in.
	*/
	Encrypt(ctx context.Context, scope Scope, plaintext, associatedData []byte) ([]byte, error)

	/*
		Decrypt opens what Encrypt returned, given the same associated
		data. The scope is read from the ciphertext.
	*/
	Decrypt(ctx context.Context, ciphertext, associatedData []byte) ([]byte, error)

	/*
		MAC returns a deterministic 32-byte MAC of the data under the
		scope's key.
	*/
	MAC(ctx context.Context, scope Scope, data []byte) ([]byte, error)

	/*
		NewStreamWriter returns a writer that encrypts everything written
		to it into w; closing it writes the final segment.
	*/
	NewStreamWriter(ctx context.Context, scope Scope, w io.Writer) (io.WriteCloser, error)

	/*
		NewStreamReader returns a reader that decrypts what a stream
		writer for the same scope wrote.
	*/
	NewStreamReader(ctx context.Context, scope Scope, r io.Reader) (io.Reader, error)
}

var _ KeyManager = (*Keys)(nil)

/*
Encrypt seals the plaintext with the scope's value keys.
*/
func (k *Keys) Encrypt(ctx context.Context, scope Scope, plaintext, associatedData []byte) ([]byte, error) {
	e, err := k.current(ctx, KeysetRef{Scope: scope, Purpose: PurposeAEAD})
	if err != nil {
		return nil, err
	}

	header := encodeHeader(scope)
	ciphertext, err := e.aead.Encrypt(plaintext, slices.Concat(header, associatedData))

	return slices.Concat(header, ciphertext), err
}

/*
Decrypt opens a ciphertext. When the cached keyset cannot, the stored
keyset is read once more, in case another server rotated it.
*/
func (k *Keys) Decrypt(ctx context.Context, ciphertext, associatedData []byte) ([]byte, error) {
	h, err := decodeHeader(ciphertext)
	if err != nil {
		return nil, err
	}

	ref := KeysetRef{Scope: h.scope, Purpose: PurposeAEAD}
	aad, body := slices.Concat(h.raw, associatedData), h.body

	e, err := k.existing(ctx, ref)
	if err != nil {
		return nil, err
	}

	if plaintext, err := e.aead.Decrypt(body, aad); err == nil {
		return plaintext, nil
	}

	stored, err := k.load(ctx, ref)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(stored.wrapped, e.wrapped) {
		return nil, ErrDecrypt
	}

	plaintext, err := stored.aead.Decrypt(body, aad)
	if err != nil {
		return nil, ErrDecrypt
	}

	return plaintext, nil
}

/*
MAC returns the HMAC-SHA256 of the data under the scope's MAC key.
*/
func (k *Keys) MAC(ctx context.Context, scope Scope, data []byte) ([]byte, error) {
	e, err := k.current(ctx, KeysetRef{Scope: scope, Purpose: PurposeMAC})
	if err != nil {
		return nil, err
	}

	mac, err := e.prf.ComputePrimaryPRF(data, macSize)
	if err != nil {
		return nil, fmt.Errorf("compute mac: %w", err)
	}

	return mac, nil
}

/*
NewStreamWriter returns a writer encrypting into w with the scope's
stream keys.
*/
func (k *Keys) NewStreamWriter(ctx context.Context, scope Scope, w io.Writer) (io.WriteCloser, error) {
	e, err := k.current(ctx, KeysetRef{Scope: scope, Purpose: PurposeStream})
	if err != nil {
		return nil, err
	}

	writer, err := e.stream.NewEncryptingWriter(w, encodeHeader(scope))
	if err != nil {
		return nil, fmt.Errorf("start encrypted stream: %w", err)
	}

	return writer, nil
}

/*
NewStreamReader returns a reader decrypting r with the scope's stream
keys, read from the store so a recent rotation is always seen.
*/
func (k *Keys) NewStreamReader(ctx context.Context, scope Scope, r io.Reader) (io.Reader, error) {
	e, err := k.load(ctx, KeysetRef{Scope: scope, Purpose: PurposeStream})
	if err != nil {
		return nil, err
	}

	reader, err := e.stream.NewDecryptingReader(r, encodeHeader(scope))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecrypt, err)
	}

	return reader, nil
}

/*
Rotate gives the scope new primary keys for values and streams. Older
keys stay in the keysets, so existing ciphertext keeps decrypting. A
keyset never created has nothing to rotate and is left alone.
*/
func (k *Keys) Rotate(ctx context.Context, scope Scope) error {
	for _, purpose := range []Purpose{PurposeAEAD, PurposeStream} {
		if err := k.rotateKeyset(ctx, KeysetRef{Scope: scope, Purpose: purpose}); err != nil {
			return err
		}
	}

	return nil
}

func (k *Keys) rotateKeyset(ctx context.Context, ref KeysetRef) error {
	e, err := k.load(ctx, ref)
	if errors.Is(err, ErrKeysetNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	keys := keyset.NewManagerFromHandle(e.handle)

	id, err := keys.Add(template(ref.Purpose))
	if err != nil {
		return fmt.Errorf("add key: %w", err)
	}

	if err := keys.SetPrimary(id); err != nil {
		return fmt.Errorf("promote key: %w", err)
	}

	handle, err := keys.Handle()
	if err != nil {
		return fmt.Errorf("build keyset: %w", err)
	}

	return k.replace(ctx, ref, e.wrapped, handle)
}

/*
RewrapAll wraps every stored keyset with the current KEK. Run it after
changing the KEK, while the old one is still configured as previous.
*/
func (k *Keys) RewrapAll(ctx context.Context) error {
	for ref, err := range k.opts.Store.All(ctx) {
		if err != nil {
			return fmt.Errorf("list keysets: %w", err)
		}

		e, err := k.load(ctx, ref)
		if err != nil {
			return err
		}

		if err := k.replace(ctx, ref, e.wrapped, e.handle); err != nil {
			return err
		}
	}

	return nil
}

func (k *Keys) replace(ctx context.Context, ref KeysetRef, previous []byte, handle *keyset.Handle) error {
	wrapped, err := k.wrap(ctx, ref, handle)
	if err != nil {
		return err
	}

	if err := k.opts.Store.Replace(ctx, ref, previous, wrapped); err != nil {
		return fmt.Errorf("store keyset %s: %w", describe(ref), err)
	}

	_, err = k.remember(ref, handle, wrapped)

	return err
}
