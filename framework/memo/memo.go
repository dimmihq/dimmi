/*
|--------------------------------------------------------------------------
| Process Memory Cache
|--------------------------------------------------------------------------
|
| Some reads happen on every request and must answer in microseconds, so a
| process keeps them in memory. What it keeps is only ever a copy: every
| value expires on its own a while after it was written, the cache never
| grows past its size, and dropping a key is always safe.
|
*/

package memo

import (
	"time"

	"github.com/maypok86/otter/v2"
)

/*
Options sets how a cache keeps values. Size is the most values it holds,
evicting the least useful first; TTL is how long a value lives after it was
written.
*/
type Options struct {
	Size int
	TTL  time.Duration
}

/*
Cache keeps values by key in this process's memory.
*/
type Cache[K comparable, V any] struct {
	store *otter.Cache[K, V]
}

/*
New returns an empty cache.
*/
func New[K comparable, V any](o Options) *Cache[K, V] {
	return &Cache[K, V]{store: otter.Must(&otter.Options[K, V]{
		MaximumSize: o.Size, ExpiryCalculator: otter.ExpiryWriting[K, V](o.TTL),
	})}
}

/*
Get returns the value kept under the key, and false when there is none or
it expired.
*/
func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.store.GetIfPresent(key)
}

/*
Set keeps the value under the key, replacing what was there.
*/
func (c *Cache[K, V]) Set(key K, value V) {
	c.store.Set(key, value)
}

/*
Delete drops the value kept under the key.
*/
func (c *Cache[K, V]) Delete(key K) {
	c.store.Invalidate(key)
}
