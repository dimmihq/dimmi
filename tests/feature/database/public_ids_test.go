package database_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

func TestPublicIDs(t *testing.T) {
	t.Parallel()

	t.Run("registers every prefix once, reserved ones included", func(t *testing.T) {
		t.Parallel()
		registry := ids.NewRegistry()

		require.NoError(t, models.RegisterPublicIDs(registry))
		err := models.RegisterPublicIDs(registry)

		reserved := 0
		for _, entry := range models.PublicIDCatalog() {
			if strings.HasPrefix(entry.Table, "reserved: ") {
				reserved++
			}
		}
		require.ErrorIs(t, err, ids.ErrPrefixTaken)
		assert.Len(t, models.PublicIDCatalog(), 133)
		assert.Equal(t, 5, reserved)
	})

	t.Run("turns every id into its public form and back", func(t *testing.T) {
		t.Parallel()
		registry := ids.NewRegistry()
		require.NoError(t, models.RegisterPublicIDs(registry))
		id := ids.New()

		for _, entry := range models.PublicIDCatalog() {
			public := models.PublicID(entry.Prefix, id)
			decoded, err := registry.Decode(string(entry.Prefix), public)

			require.NoError(t, err, entry.Table)
			assert.Equal(t, id, decoded, entry.Table)
			assert.True(t, strings.HasPrefix(public, string(entry.Prefix)+"_"), entry.Table)
		}
	})

	t.Run("refuses an id of another kind", func(t *testing.T) {
		t.Parallel()
		registry := ids.NewRegistry()
		require.NoError(t, models.RegisterPublicIDs(registry))
		message := models.PublicID(models.PrefixMessage, ids.New())

		_, err := registry.Decode(string(models.PrefixConversation), message)

		require.ErrorIs(t, err, ids.ErrWrongPrefix)
	})
}
