package helpcenter

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/lang"
)

func TestEveryLanguageReadsInItself(t *testing.T) {
	english := map[string]string{"en": "", "en-GB": ""}
	for _, l := range helpcenters.Languages() {
		c := Center{Locale: l.Code}
		got := c.T("helpful_question")
		if _, ok := english[l.Code]; ok {
			assert.Equal(t, "Was this article helpful?", got)
			continue
		}
		assert.NotEqual(t, "Was this article helpful?", got, l.Code)
		assert.NotEqual(t, "helpcenter.helpful_question", got, l.Code)
	}
	assert.Equal(t, "Dieser Artikel ist noch nicht auf Deutsch verfügbar. Du liest das Original.",
		Center{Locale: "de"}.T("untranslated", "language", "Deutsch"))
	assert.Equal(t, "3 articles", Center{}.T("article_count", "count", 3))
	assert.Equal(t, "1 article", Center{}.T("article_count", "count", 1))
	assert.Equal(t, "Articles : 3", Center{Locale: "fr"}.T("article_count", "count", 3))
}

func TestEveryLanguageHasEveryWordWithItsPlaceholders(t *testing.T) {
	source := readWords(t, "en")
	placeholder := regexp.MustCompile(`\{\{\.\w+\}\}`)
	dirs, err := fs.ReadDir(lang.Files(), ".")
	require.NoError(t, err)
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "en" {
			continue
		}
		if _, err := fs.Stat(lang.Files(), d.Name()+"/helpcenter.json"); err != nil {
			continue
		}
		got := readWords(t, d.Name())
		for key, text := range source {
			assert.Contains(t, got, key, d.Name())
			want := placeholder.FindAllString(text, -1)
			have := placeholder.FindAllString(got[key], -1)
			slices.Sort(want)
			slices.Sort(have)
			if key != "date_format" {
				for _, p := range slices.Compact(have) {
					assert.Contains(t, want, p, d.Name()+" "+key)
				}
				for _, p := range slices.Compact(want) {
					if p != "{{.count}}" || key != "by_many" {
						assert.Contains(t, have, p, d.Name()+" "+key)
					}
				}
			}
		}
	}
}

func readWords(t *testing.T, locale string) map[string]string {
	t.Helper()
	raw, err := fs.ReadFile(lang.Files(), locale+"/helpcenter.json")
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	out := map[string]string{}
	for k, v := range parsed {
		switch v := v.(type) {
		case string:
			out[k] = v
		case map[string]any:
			out[k] = strings.TrimSpace(v["other"].(string))
		}
	}
	return out
}
