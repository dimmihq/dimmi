package ids_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
)

func TestIDs(t *testing.T) {
	t.Parallel()

	t.Run("makes increasing version 7 uuids", func(t *testing.T) {
		t.Parallel()
		first, second := ids.New(), ids.New()

		assert.Equal(t, uuid.Version(7), first.Version())
		assert.Equal(t, uuid.RFC4122, first.Variant())
		assert.Less(t, first.String(), second.String())
	})

	t.Run("carries a given time to the millisecond", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2019, 3, 14, 15, 9, 26, 535_897_000, time.UTC)

		id := ids.NewV7At(at)

		assert.Equal(t, uuid.Version(7), id.Version())
		assert.Equal(t, uuid.RFC4122, id.Variant())
		sec, nsec := id.Time().UnixTime()
		assert.Equal(t, at.Truncate(time.Millisecond), time.Unix(sec, nsec).UTC())
		assert.NotEqual(t, id, ids.NewV7At(at))
		assert.Less(t, id.String(), ids.NewV7At(at.Add(time.Millisecond)).String())
	})

	t.Run("stores times before 1970 as 1970", func(t *testing.T) {
		t.Parallel()

		id := ids.NewV7At(time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC))

		assert.Equal(t, make([]byte, 6), id[:6])
	})

	t.Run("parses the standard form", func(t *testing.T) {
		t.Parallel()
		id := ids.New()

		parsed, err := ids.Parse(id.String())
		require.NoError(t, err)
		assert.Equal(t, id, parsed)

		_, err = ids.Parse("not-a-uuid")
		require.ErrorIs(t, err, ids.ErrInvalid)
	})

	t.Run("has a nil value", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, ids.UUID{}, ids.Nil())
	})

	t.Run("encodes a public id the registry reads back", func(t *testing.T) {
		t.Parallel()
		r := ids.NewRegistry()
		require.NoError(t, r.Register("usr", "user"))
		id := ids.New()

		public := ids.Encode("usr", id)

		assert.Equal(t, r.Encode("usr", id), public)
		decoded, err := r.Decode("usr", public)
		require.NoError(t, err)
		assert.Equal(t, id, decoded)
	})
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("round trips a public id", func(t *testing.T) {
		t.Parallel()
		reg := ids.NewRegistry()
		require.NoError(t, reg.Register("user", "user"))
		id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")

		public := reg.Encode("user", id)
		assert.Equal(t, "user_01h455vb4pex5vsknk084sn02q", public)

		decoded, err := reg.Decode("user", public)
		require.NoError(t, err)
		assert.Equal(t, id, decoded)
	})

	t.Run("refuses an id of another type", func(t *testing.T) {
		t.Parallel()
		reg := ids.NewRegistry()
		require.NoError(t, reg.Register("user", "user"))
		require.NoError(t, reg.Register("team", "team"))

		_, err := reg.Decode("user", reg.Encode("team", ids.New()))
		require.ErrorIs(t, err, ids.ErrWrongPrefix)
		require.ErrorContains(t, err, "decode user id")

		_, err = reg.Decode("user", "01h455vb4pex5vsknk084sn02q")
		require.ErrorIs(t, err, ids.ErrWrongPrefix)
	})

	t.Run("refuses text that is not an id", func(t *testing.T) {
		t.Parallel()
		reg := ids.NewRegistry()
		require.NoError(t, reg.Register("user", "user"))

		invalid := []string{
			"", "user_", "user_zzz", "user_81h455vb4pex5vsknk084sn02q", "USER_01h455vb4pex5vsknk084sn02q",
		}
		for _, s := range invalid {
			_, err := reg.Decode("user", s)
			require.ErrorIs(t, err, ids.ErrInvalid, s)
		}
	})

	t.Run("refuses a prefix that was never registered", func(t *testing.T) {
		t.Parallel()
		reg := ids.NewRegistry()

		_, err := reg.Decode("ghost", reg.Encode("ghost", ids.New()))

		require.ErrorIs(t, err, ids.ErrUnknownPrefix)
	})

	t.Run("refuses invalid and taken prefixes", func(t *testing.T) {
		t.Parallel()
		reg := ids.NewRegistry()
		require.NoError(t, reg.Register("api_key", "api key"))

		for _, prefix := range []string{"", "User", "_user", "user_", "user1"} {
			require.ErrorIs(t, reg.Register(prefix, "user"), ids.ErrInvalidPrefix, prefix)
		}

		err := reg.Register("api_key", "access key")
		require.ErrorIs(t, err, ids.ErrPrefixTaken)
		require.ErrorContains(t, err, "owned by api key")
	})
}
