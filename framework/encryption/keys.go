/*
|--------------------------------------------------------------------------
| Data Keys
|--------------------------------------------------------------------------
|
| Every scope, such as the system or one tenant, has its own data keys,
| so deleting a scope's keys makes everything encrypted for it unreadable
| at once, with no data left to hunt down. Keys are Tink keysets: one for
| values, one for streams and one for MACs, each created on first use and
| stored only wrapped by the KEK. Rotating adds a new primary key to a
| keyset and keeps the old ones, so existing ciphertext stays readable.
|
| Where keysets are stored is the application's choice: it implements
| KeysetStore over its own table. Unwrapped keysets are cached in memory
| for a short time; a ciphertext that does not decrypt with the cached
| keyset is retried once with the stored one, so a rotation made by
| another server is picked up at once.
|
*/

package encryption

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/prf"
	tinkpb "github.com/tink-crypto/tink-go/v2/proto/tink_go_proto"
	"github.com/tink-crypto/tink-go/v2/streamingaead"
	"github.com/tink-crypto/tink-go/v2/tink"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Errors from data keys and their store.

ErrKeysetNotFound means a scope has no keyset for the purpose, either
never created or destroyed. ErrKeysetExists and ErrKeysetChanged are how
a store reports losing a race to another server. ErrMalformed refuses a
ciphertext without a valid header. ErrInvalidScope refuses a scope whose
kind or ID is longer than 255 bytes. ErrInvalidOptions means a key
manager was built without a KEK or a store.
*/
var (
	ErrKeysetNotFound = errors.New("encryption: keyset not found")
	ErrKeysetExists   = errors.New("encryption: keyset already exists")
	ErrKeysetChanged  = errors.New("encryption: keyset changed concurrently")
	ErrMalformed      = errors.New("encryption: malformed ciphertext")
	ErrInvalidScope   = errors.New("encryption: scope kind and id must be at most 255 bytes")
	ErrInvalidOptions = errors.New("encryption: kek and store are required")
)

/*
Scope owns a set of data keys. Kind says what owns them, such as
"system" or "tenant", and ID which one.
*/
type Scope struct {
	Kind string
	ID   string
}

/*
Purpose is what a keyset is for.
*/
type Purpose string

/*
The keyset purposes: encrypting values, encrypting streams, and
computing deterministic MACs.
*/
const (
	PurposeAEAD   Purpose = "aead"
	PurposeStream Purpose = "stream"
	PurposeMAC    Purpose = "mac"
)

/*
KeysetRef names one stored keyset.
*/
type KeysetRef struct {
	Scope   Scope
	Purpose Purpose
}

/*
KeysetStore keeps wrapped keysets. Implementations must make Create and
Replace atomic, because two servers may race to create or rotate the
same keyset and the loser must not overwrite the winner.
*/
type KeysetStore interface {
	/*
		Find returns the wrapped keyset, or ErrKeysetNotFound.
	*/
	Find(ctx context.Context, ref KeysetRef) ([]byte, error)

	/*
		Create stores a new keyset, or returns ErrKeysetExists.
	*/
	Create(ctx context.Context, ref KeysetRef, wrapped []byte) error

	/*
		Replace swaps the keyset only while it still equals previous, and
		returns ErrKeysetChanged otherwise.
	*/
	Replace(ctx context.Context, ref KeysetRef, previous, next []byte) error

	/*
		All yields every stored keyset's reference.
	*/
	All(ctx context.Context) iter.Seq2[KeysetRef, error]
}

/*
KeysOptions configure a key manager.

KEK wraps every keyset written; it and Previous unwrap. CacheTTL is how
long an unwrapped keyset is reused, five minutes by default. Clock
defaults to the system clock.
*/
type KeysOptions struct {
	KEK      KEK
	Previous []KEK
	Store    KeysetStore
	CacheTTL time.Duration
	Clock    clock.Clock
}

/*
Keys is the KeyManager on Tink keysets wrapped by a KEK.
*/
type Keys struct {
	opts KeysOptions

	mu    sync.Mutex
	cache map[KeysetRef]entry
}

/*
entry is an unwrapped keyset with the primitive for its purpose, and
the wrapped bytes it was read from, which Replace needs to detect a
concurrent change.
*/
type entry struct {
	handle   *keyset.Handle
	aead     tink.AEAD
	stream   tink.StreamingAEAD
	prf      *prf.Set
	wrapped  []byte
	loadedAt time.Time
}

/*
NewKeys returns a key manager.
*/
func NewKeys(opts KeysOptions) (*Keys, error) {
	if opts.KEK == nil || opts.Store == nil {
		return nil, ErrInvalidOptions
	}

	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 5 * time.Minute
	}

	if opts.Clock == nil {
		opts.Clock = clock.System()
	}

	return &Keys{opts: opts, cache: map[KeysetRef]entry{}}, nil
}

/*
current returns the keyset for encrypting, creating it on first use.
*/
func (k *Keys) current(ctx context.Context, ref KeysetRef) (entry, error) {
	if len(ref.Scope.Kind) > maxField || len(ref.Scope.ID) > maxField {
		return entry{}, ErrInvalidScope
	}

	if e, ok := k.cached(ref); ok {
		return e, nil
	}

	e, err := k.load(ctx, ref)
	if !errors.Is(err, ErrKeysetNotFound) {
		return e, err
	}

	return k.create(ctx, ref)
}

/*
existing returns the keyset for decrypting; it is never created.
*/
func (k *Keys) existing(ctx context.Context, ref KeysetRef) (entry, error) {
	if e, ok := k.cached(ref); ok {
		return e, nil
	}

	return k.load(ctx, ref)
}

func (k *Keys) cached(ref KeysetRef) (entry, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	e, ok := k.cache[ref]
	if !ok || k.opts.Clock.Since(e.loadedAt) >= k.opts.CacheTTL {
		return entry{}, false
	}

	return e, true
}

/*
load reads the keyset from the store, bypassing the cache.
*/
func (k *Keys) load(ctx context.Context, ref KeysetRef) (entry, error) {
	wrapped, err := k.opts.Store.Find(ctx, ref)
	if err != nil {
		return entry{}, fmt.Errorf("find keyset %s: %w", describe(ref), err)
	}

	handle, err := k.unwrap(ctx, ref, wrapped)
	if err != nil {
		return entry{}, err
	}

	return k.remember(ref, handle, wrapped)
}

/*
create stores a new keyset. When another server created one first, its
keyset is used instead.
*/
func (k *Keys) create(ctx context.Context, ref KeysetRef) (entry, error) {
	handle, err := keyset.NewHandle(template(ref.Purpose))
	if err != nil {
		return entry{}, fmt.Errorf("create keyset: %w", err)
	}

	wrapped, err := k.wrap(ctx, ref, handle)
	if err != nil {
		return entry{}, err
	}

	err = k.opts.Store.Create(ctx, ref, wrapped)
	if errors.Is(err, ErrKeysetExists) {
		return k.load(ctx, ref)
	}

	if err != nil {
		return entry{}, fmt.Errorf("store keyset %s: %w", describe(ref), err)
	}

	return k.remember(ref, handle, wrapped)
}

func (k *Keys) remember(ref KeysetRef, handle *keyset.Handle, wrapped []byte) (entry, error) {
	e := entry{handle: handle, wrapped: wrapped, loadedAt: k.opts.Clock.Now()}
	if err := e.build(ref.Purpose); err != nil {
		return entry{}, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	k.cache[ref] = e

	return e, nil
}

func (k *Keys) wrap(ctx context.Context, ref KeysetRef, handle *keyset.Handle) ([]byte, error) {
	var buf bytes.Buffer

	err := handle.WriteWithContext(ctx, keyset.NewBinaryWriter(&buf), tinkKEK{kek: k.opts.KEK}, refAAD(ref))
	if err != nil {
		return nil, fmt.Errorf("wrap keyset %s: %w", describe(ref), err)
	}

	return buf.Bytes(), nil
}

/*
unwrap tries the current KEK, then each previous one.
*/
func (k *Keys) unwrap(ctx context.Context, ref KeysetRef, wrapped []byte) (*keyset.Handle, error) {
	var err error

	for _, kek := range append([]KEK{k.opts.KEK}, k.opts.Previous...) {
		reader := keyset.NewBinaryReader(bytes.NewReader(wrapped))

		var handle *keyset.Handle
		if handle, err = keyset.ReadWithContext(ctx, reader, tinkKEK{kek: kek}, refAAD(ref)); err == nil {
			return handle, nil
		}
	}

	return nil, fmt.Errorf("%w: unwrap keyset %s: %w", ErrDecrypt, describe(ref), err)
}

func template(purpose Purpose) *tinkpb.KeyTemplate {
	switch purpose {
	case PurposeStream:
		return streamingaead.AES256GCMHKDF1MBKeyTemplate()
	case PurposeMAC:
		return prf.HMACSHA256PRFKeyTemplate()
	case PurposeAEAD:
	}

	return aead.AES256GCMKeyTemplate()
}

/*
build creates the primitive for the keyset's purpose.
*/
func (e *entry) build(purpose Purpose) error {
	var err error

	switch purpose {
	case PurposeStream:
		e.stream, err = streamingaead.New(e.handle)
	case PurposeMAC:
		e.prf, err = prf.NewPRFSet(e.handle)
	case PurposeAEAD:
		e.aead, err = aead.New(e.handle)
	}

	if err != nil {
		return fmt.Errorf("load %s keyset: %w", purpose, err)
	}

	return nil
}

/*
refAAD binds a wrapped keyset to its scope and purpose.
*/
func refAAD(ref KeysetRef) []byte {
	return appendString(append(encodeScope(ref.Scope), "keyset"...), string(ref.Purpose))
}

func describe(ref KeysetRef) string {
	return fmt.Sprintf("%s/%s/%s", ref.Scope.Kind, ref.Scope.ID, ref.Purpose)
}
