package arch_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/http/controllers/web"
	"github.com/dimmihq/dimmi/tests"
)

/*
viteInputs returns the entries the real build produces, read from the
input list in vite.config.ts.
*/
func viteInputs(t *testing.T) []string {
	t.Helper()

	config, err := os.ReadFile("../../vite.config.ts")
	require.NoError(t, err)

	list := regexp.MustCompile(`(?m)^\s*input:\s*\[([^\]]*)\]`).FindSubmatch(config)
	require.Len(t, list, 2, "vite.config.ts has no input list")

	var inputs []string
	for _, match := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(list[1], -1) {
		inputs = append(inputs, string(match[1]))
	}
	slices.Sort(inputs)

	return inputs
}

func TestViteEntries(t *testing.T) {
	t.Parallel()

	t.Run("the server only renders entries the real build produces", func(t *testing.T) {
		t.Parallel()
		inputs := viteInputs(t)

		for _, entry := range []string{web.Entry, web.PortalEntry, web.PortalStyles} {
			assert.Contains(t, inputs, entry, "%s is rendered but vite.config.ts does not build it", entry)
		}
	})

	t.Run("the tests' build names exactly the entries the real build produces", func(t *testing.T) {
		t.Parallel()

		raw, err := fs.ReadFile(tests.Frontend(), ".vite/manifest.json")
		require.NoError(t, err)

		var manifest map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &manifest))

		assert.Equal(t, viteInputs(t), slices.Sorted(maps.Keys(manifest)))
	})
}
