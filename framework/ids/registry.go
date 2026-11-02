/*
|--------------------------------------------------------------------------
| Public Identifiers
|--------------------------------------------------------------------------
|
| Ids that leave the application carry a short type prefix, such as
| user_01h455vb4pex5vsknk084sn02q, so a person reading one knows what it
| names and an id pasted into the wrong place is refused instead of
| finding an unrelated row. Each prefix is registered once with the name
| of the thing it names; two types can never share a prefix, and an id
| whose prefix was never registered cannot be read back.
|
*/

package ids

import (
	"errors"
	"fmt"
	"sync"

	"go.jetify.com/typeid"
)

/*
Errors from the registry. A prefix to register must be lowercase ASCII
letters and underscores, at most 63 characters, not starting or ending
with an underscore, and owned by no other type. A public id to decode
must carry the expected prefix, which must have been registered.
*/
var (
	ErrInvalidPrefix = errors.New("ids: invalid prefix")
	ErrPrefixTaken   = errors.New("ids: prefix already registered")
	ErrWrongPrefix   = errors.New("ids: wrong prefix")
	ErrUnknownPrefix = errors.New("ids: unknown prefix")
)

/*
Registry holds the public id prefixes the application uses and the name
of the type each one names. It is safe for concurrent use.
*/
type Registry struct {
	mu    sync.RWMutex
	names map[string]string
}

/*
NewRegistry returns an empty registry.
*/
func NewRegistry() *Registry {
	return &Registry{names: map[string]string{}}
}

/*
Register records that the prefix names the type, such as "user" for
user accounts.
*/
func (r *Registry) Register(prefix, name string) error {
	if _, err := typeid.WithPrefix(prefix); err != nil || prefix == "" {
		return fmt.Errorf("register %s prefix %q: %w", name, prefix, ErrInvalidPrefix)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if owner, ok := r.names[prefix]; ok {
		return fmt.Errorf("register %s prefix %q, owned by %s: %w", name, prefix, owner, ErrPrefixTaken)
	}

	r.names[prefix] = name

	return nil
}

/*
Encode returns the public id for the UUID: the prefix, an underscore and
the UUID as 26 lowercase Crockford base32 characters. It never fails; an
id encoded with an unregistered prefix is refused by Decode.
*/
func (r *Registry) Encode(prefix string, id UUID) string {
	return Encode(prefix, id)
}

/*
Decode reads a public id that must carry the prefix. An id of another
type fails with ErrWrongPrefix and text that is no id at all with
ErrInvalid, so both can be answered as "not found".
*/
func (r *Registry) Decode(prefix, s string) (UUID, error) {
	r.mu.RLock()
	name, ok := r.names[prefix]
	r.mu.RUnlock()

	if !ok {
		return Nil(), fmt.Errorf("decode id with prefix %q: %w", prefix, ErrUnknownPrefix)
	}

	tid, err := typeid.FromString(s)
	if err != nil {
		return Nil(), fmt.Errorf("decode %s id: %w", name, ErrInvalid)
	}

	if tid.Prefix() != prefix {
		return Nil(), fmt.Errorf("decode %s id: %w", name, ErrWrongPrefix)
	}

	return UUID(tid.UUIDBytes()), nil
}
