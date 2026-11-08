package attributes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/attributes"
)

func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("infers a new key's type from its first value", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "boolean", attributes.Infer("vip", true))
		assert.Equal(t, "number", attributes.Infer("seats", 3.0))
		assert.Equal(t, "date", attributes.Infer("signed_up_at", 1700000000.0))
		assert.Equal(t, "date", attributes.Infer("renewal", "2026-12-01"))
		assert.Equal(t, "string", attributes.Infer("plan", "pro"))
	})

	t.Run("keeps dates as unix seconds whatever they arrive as", func(t *testing.T) {
		t.Parallel()
		for _, in := range []any{"2026-12-01T00:00:00Z", "2026-12-01", 1796083200.0, 1796083200000.0} {
			got, problem := attributes.Coerce("date", nil, in)
			assert.Empty(t, problem)
			assert.InDelta(t, 1796083200.0, got, 0)
		}

		_, problem := attributes.Coerce("date", nil, "next week")
		assert.Equal(t, "type_mismatch", problem)
	})

	t.Run("drops values that do not fit and lets null through", func(t *testing.T) {
		t.Parallel()
		_, problem := attributes.Coerce("number", nil, "12")
		assert.Equal(t, "type_mismatch", problem)
		_, problem = attributes.Coerce("string", nil, 12.0)
		assert.Equal(t, "type_mismatch", problem)
		_, problem = attributes.Coerce("boolean", nil, "yes")
		assert.Equal(t, "type_mismatch", problem)
		_, problem = attributes.Coerce("list", []string{"pro", "team"}, "gold")
		assert.Equal(t, "invalid_option", problem)

		got, problem := attributes.Coerce("list", []string{"pro"}, "pro")
		assert.Equal(t, "pro", got)
		assert.Empty(t, problem)
		got, problem = attributes.Coerce("number", nil, nil)
		assert.Nil(t, got)
		assert.Empty(t, problem)
	})

	t.Run("describes a value without showing it", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "text of 3 characters", attributes.Shape("pro"))
		assert.Equal(t, "number", attributes.Shape(1.0))
	})
}
