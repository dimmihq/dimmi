package pagination_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http/pagination"
)

type position struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func TestCursors(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	cursors := pagination.NewCursors(key)

	t.Run("round trips a position", func(t *testing.T) {
		t.Parallel()
		want := position{CreatedAt: "2026-11-01T10:00:00Z", ID: "conv_1"}
		encoded, err := cursors.Encode(want)
		require.NoError(t, err)
		assert.NotContains(t, encoded, "=")
		assert.NotContains(t, encoded, "/")

		var got position
		require.NoError(t, cursors.Decode(encoded, &got))
		assert.Equal(t, want, got)
	})

	t.Run("does not depend on the caller keeping the key unchanged", func(t *testing.T) {
		t.Parallel()
		mutable := []byte("secret-key")
		c := pagination.NewCursors(mutable)
		encoded, err := c.Encode(position{ID: "a"})
		require.NoError(t, err)
		mutable[0] = 'X'

		var got position
		require.NoError(t, c.Decode(encoded, &got))
	})

	t.Run("refuses a tampered payload", func(t *testing.T) {
		t.Parallel()
		encoded, err := cursors.Encode(position{ID: "conv_1"})
		require.NoError(t, err)
		_, mac, _ := strings.Cut(encoded, ".")
		forged := base64.RawURLEncoding.EncodeToString([]byte(`{"id":"conv_2"}`)) + "." + mac

		var got position
		err = cursors.Decode(forged, &got)
		require.ErrorIs(t, err, pagination.ErrInvalidCursor)
		assert.Equal(t, fault.Invalid, fault.KindOf(err))
		assert.Equal(t, "invalid_cursor", fault.CodeOf(err))
		assert.Empty(t, got.ID)
	})

	t.Run("refuses a cursor signed with another key", func(t *testing.T) {
		t.Parallel()
		encoded, err := pagination.NewCursors([]byte("other")).Encode(position{ID: "conv_1"})
		require.NoError(t, err)

		require.ErrorIs(t, cursors.Decode(encoded, &position{}), pagination.ErrInvalidCursor)
	})

	t.Run("refuses garbage", func(t *testing.T) {
		t.Parallel()
		for _, s := range []string{"", "nodot", "!!!.abc", "abc.!!!", "abc.abc"} {
			require.ErrorIs(t, cursors.Decode(s, &position{}), pagination.ErrInvalidCursor, s)
		}
	})

	t.Run("refuses a signed cursor that does not fit the value", func(t *testing.T) {
		t.Parallel()
		encoded, err := cursors.Encode([]int{1, 2})
		require.NoError(t, err)

		require.ErrorIs(t, cursors.Decode(encoded, &position{}), pagination.ErrInvalidCursor)
	})

	t.Run("reports a value that cannot be encoded", func(t *testing.T) {
		t.Parallel()
		_, err := cursors.Encode(func() {})
		require.Error(t, err)
	})
}
