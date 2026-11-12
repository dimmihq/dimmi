package widgetstrings_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/widgetstrings"
	widgetlang "github.com/dimmihq/dimmi/resources/widget/lang"
)

func TestShippedLanguages(t *testing.T) {
	t.Parallel()

	words, err := widgetstrings.Load()
	require.NoError(t, err)

	t.Run("every language has every built-in word, with the same placeholders", func(t *testing.T) {
		t.Parallel()

		english := read(t, "en.json")
		for _, locale := range words.Locales() {
			other := read(t, locale+".json")
			assert.Equal(t, slices.Sorted(maps.Keys(english)), slices.Sorted(maps.Keys(other)), locale)

			for key, word := range english {
				if word == "" {
					continue
				}

				assert.NotEmpty(t, other[key], "%s %s", locale, key)
				assert.Equal(t, strings.Contains(word, "{email}"), strings.Contains(other[key], "{email}"), locale)
			}
		}
	})

	t.Run("ships the messenger's languages, two of them right to left", func(t *testing.T) {
		t.Parallel()

		for _, locale := range []string{"es", "fr", "de", "pt-BR", "it", "nl", "ar", "he", "ja", "zh-CN"} {
			assert.Contains(t, words.Locales(), locale)
		}

		assert.Equal(t, "rtl", widgetstrings.Direction("ar"))
		assert.Equal(t, "rtl", widgetstrings.Direction("he"))
		assert.Equal(t, "ltr", widgetstrings.Direction("zh-CN"))
	})

	t.Run("a visitor gets their language, its region's sibling, or English", func(t *testing.T) {
		t.Parallel()

		for asked, want := range map[string]string{
			"fr-CA": "fr", "de": "de", "pt-PT": "pt-BR", "pt": "pt-BR", "zh-CN": "zh-CN", "zh": "zh-CN",
			"he-IL": "he", "sw": "en", "": "en",
		} {
			assert.Equal(t, want, words.Negotiate(widgetstrings.Request{Requested: asked, Default: "en"}), asked)
		}

		resolved, err := words.Resolve("ar", "en", []byte(`{"greeting":"Hi from Acme"}`))
		require.NoError(t, err)
		assert.Equal(t, "Hi from Acme", resolved["greeting"])
		assert.Equal(t, "شوهدت", resolved["seen"])
	})
}

/*
read returns one shipped language file.
*/
func read(t *testing.T, name string) map[string]string {
	t.Helper()

	data, err := fs.ReadFile(widgetlang.Files(), name)
	require.NoError(t, err)

	out := map[string]string{}
	require.NoError(t, json.Unmarshal(data, &out))

	return out
}
