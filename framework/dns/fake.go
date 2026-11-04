/*
|--------------------------------------------------------------------------
| Fake Zone
|--------------------------------------------------------------------------
|
| Tests describe the records a name should have and lookups answer from
| them, exactly as a real server holding that zone would: names match
| without regard to case or a trailing dot, aliases are followed, and a
| missing name or record type is ErrNotFound.
|
*/

package dns

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
)

/*
maxAliases bounds how many aliases a lookup follows, so a loop in a
fake zone ends instead of spinning.
*/
const maxAliases = 8

/*
Records are everything a fake zone holds for one name. A name with a
CNAME is an alias and holds nothing else.
*/
type Records struct {
	TXT   []string
	MX    []MX
	CNAME string
	Addrs []netip.Addr
	NS    []string
}

/*
Fake is a resolver answering from records set by the test.
*/
type Fake struct {
	mu    sync.RWMutex
	names map[string]Records
}

/*
NewFake returns an empty fake zone.
*/
func NewFake() *Fake {
	return &Fake{names: map[string]Records{}}
}

/*
Set replaces every record of the name.
*/
func (f *Fake) Set(name string, records Records) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.names[normalize(name)] = records

	return f
}

/*
TXT returns the TXT records of the name.
*/
func (f *Fake) TXT(ctx context.Context, name string) ([]string, error) {
	records, err := f.lookup(ctx, "txt", name)
	if err != nil {
		return nil, err
	}

	return present("txt", name, records.TXT)
}

/*
MX returns the mail exchangers of the name, lowest preference first.
*/
func (f *Fake) MX(ctx context.Context, name string) ([]MX, error) {
	records, err := f.lookup(ctx, "mx", name)
	if err != nil {
		return nil, err
	}

	exchangers, err := present("mx", name, records.MX)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(exchangers, func(a, b MX) int { return cmp.Compare(a.Pref, b.Pref) })

	return exchangers, nil
}

/*
CNAME returns the canonical name of the name.
*/
func (f *Fake) CNAME(ctx context.Context, name string) (string, error) {
	if _, err := f.lookup(ctx, "cname", name); err != nil {
		return "", err
	}

	canonical, _, _ := f.resolve(name)

	return canonical, nil
}

/*
Addrs returns the addresses of the name.
*/
func (f *Fake) Addrs(ctx context.Context, name string) ([]netip.Addr, error) {
	records, err := f.lookup(ctx, "addrs", name)
	if err != nil {
		return nil, err
	}

	return present("addrs", name, records.Addrs)
}

/*
NS returns the name servers of the name.
*/
func (f *Fake) NS(ctx context.Context, name string) ([]string, error) {
	records, err := f.lookup(ctx, "ns", name)
	if err != nil {
		return nil, err
	}

	return present("ns", name, records.NS)
}

/*
lookup returns the records at the end of the name's aliases, or
ErrNotFound when the name does not exist.
*/
func (f *Fake) lookup(ctx context.Context, kind, name string) (Records, error) {
	if err := ctx.Err(); err != nil {
		return Records{}, fmt.Errorf("lookup %s %s: %w", kind, name, err)
	}

	_, records, ok := f.resolve(name)
	if !ok {
		return Records{}, fmt.Errorf("lookup %s %s: %w", kind, name, ErrNotFound)
	}

	return records, nil
}

/*
resolve follows aliases from the name and returns the canonical name with
its records.
*/
func (f *Fake) resolve(name string) (string, Records, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	name = normalize(name)
	for range maxAliases {
		records, ok := f.names[name]
		if !ok {
			return "", Records{}, false
		}

		if records.CNAME == "" {
			return name, records, true
		}

		name = normalize(records.CNAME)
	}

	return "", Records{}, false
}

/*
present returns a copy of the values, or ErrNotFound when the name holds
none of that type.
*/
func present[T any](kind, name string, values []T) ([]T, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("lookup %s %s: %w", kind, name, ErrNotFound)
	}

	return slices.Clone(values), nil
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
}
