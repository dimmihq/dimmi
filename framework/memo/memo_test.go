package memo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/memo"
)

func TestCache(t *testing.T) {
	t.Parallel()

	t.Run("returns what was set until it is deleted", func(t *testing.T) {
		t.Parallel()
		c := memo.New[string, int](memo.Options{Size: 10, TTL: time.Minute})

		c.Set("a", 1)
		got, ok := c.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, got)

		c.Delete("a")
		_, ok = c.Get("a")
		assert.False(t, ok)
	})

	t.Run("forgets a value once its time is over", func(t *testing.T) {
		t.Parallel()
		c := memo.New[string, int](memo.Options{Size: 10, TTL: 20 * time.Millisecond})

		c.Set("a", 1)

		assert.Eventually(t, func() bool {
			_, ok := c.Get("a")

			return !ok
		}, time.Second, 5*time.Millisecond)
	})
}
