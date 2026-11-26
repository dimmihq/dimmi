package helpcenter

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/lang"
)

func translator(t *testing.T) *i18n.Translator {
	t.Helper()
	tr, err := i18n.New(i18n.Config{Locale: i18n.Source}, lang.Files())
	require.NoError(t, err)
	return tr
}

func TestEveryLanguageReadsInItself(t *testing.T) {
	tr := translator(t)
	for _, l := range helpcenters.Languages() {
		got := Center{Locale: l.Code, Lang: tr}.T("helpful_question")
		if l.Code == "en" || l.Code == "en-GB" {
			assert.Equal(t, "Was this article helpful?", got)
			continue
		}
		assert.NotEqual(t, "Was this article helpful?", got, l.Code)
		assert.NotEqual(t, "helpcenter.helpful_question", got, l.Code)
	}
	assert.Equal(t, "Dieser Artikel ist noch nicht auf Deutsch verfügbar. Du liest das Original.",
		Center{Locale: "de", Lang: tr}.T("untranslated", "language", "Deutsch"))
	assert.Equal(t, "3 articles", Center{Lang: tr}.T("article_count", "count", 3))
	assert.Equal(t, "1 article", Center{Lang: tr}.T("article_count", "count", 1))
	assert.Equal(t, "Articles : 3", Center{Locale: "fr", Lang: tr}.T("article_count", "count", 3))
}

func TestEveryLanguageHasEveryWordWithItsPlaceholders(t *testing.T) {
	source := readWords(t, "en")
	dirs, err := fs.ReadDir(lang.Files(), ".")
	require.NoError(t, err)
	for _, d := range dirs {
		if _, err := fs.Stat(lang.Files(), d.Name()+"/helpcenter.json"); err != nil || d.Name() == "en" {
			continue
		}
		got := readWords(t, d.Name())
		for key, text := range source {
			assert.Contains(t, got, key, d.Name())
			if key != "date_format" {
				samePlaceholders(t, d.Name()+" "+key, [2]string{text, got[key]})
			}
		}
	}
}

/*
samePlaceholders checks a translation fills the same values as the
English text, the count of an English plural aside when allowed.
*/
func samePlaceholders(t *testing.T, name string, texts [2]string) {
	t.Helper()
	english, translated := texts[0], texts[1]
	countOptional := strings.HasSuffix(name, " by_many")
	placeholder := regexp.MustCompile(`\{\{\.\w+\}\}`)
	want := slices.Compact(slices.Sorted(slices.Values(placeholder.FindAllString(english, -1))))
	have := slices.Compact(slices.Sorted(slices.Values(placeholder.FindAllString(translated, -1))))
	if countOptional {
		want = slices.DeleteFunc(want, func(p string) bool { return p == "{{.count}}" })
		have = slices.DeleteFunc(have, func(p string) bool { return p == "{{.count}}" })
	}
	assert.Equal(t, want, have, name)
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
			other, _ := v["other"].(string)
			out[k] = other
		}
	}
	return out
}

func TestAgoCountsBackInEachLanguage(t *testing.T) {
	tr := translator(t)
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	back := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	for _, c := range []struct {
		locale string
		days   int
		want   string
	}{
		{"de", 3, "vor 3 Tagen"},
		{"de", 9, "vor über einer Woche"},
		{"fr", 0, "aujourd'hui"},
		{"fr", 70, "il y a plus de 2 mois"},
		{"ru", 2, "2 дня назад"},
		{"ru", 5, "5 дней назад"},
		{"ru", 800, "более 2 лет назад"},
		{"pl", 3, "3 dni temu"},
		{"pl", 45, "ponad miesiąc temu"},
		{"ar", 2, "منذ يومين"},
		{"ar", 4, "منذ 4 أيام"},
		{"ja", 16, "2週間以上前"},
	} {
		assert.Equal(t, c.want, Center{Locale: c.locale, Lang: tr}.Ago(back(c.days), now), c.locale, c.days)
	}
}

func TestPortalWordsReadInEachLanguage(t *testing.T) {
	tr := translator(t)
	ru := Center{Locale: "ru", Lang: tr}
	assert.Equal(t, "Голосовать за «Тёмная тема», 5 голосов", ru.T("vote_for", "title", "Тёмная тема", "count", 5))
	assert.Equal(t, "май", ru.Month(time.May))
	assert.Equal(t, "Mai", Center{Locale: "de", Lang: tr}.Month(time.May))
	assert.Equal(t, "Jan", Center{Lang: tr}.Month(time.January))
	assert.Empty(t, Center{Lang: tr}.Month(0))
	assert.Equal(t, "Changelog", Center{Lang: tr}.ChangelogTitle(""))
	assert.Equal(t, "Journal des modifications", Center{Locale: "fr", Lang: tr}.ChangelogTitle("Changelog"))
	assert.Equal(t, "What's new", Center{Locale: "fr", Lang: tr}.ChangelogTitle("What's new"))
	for _, l := range helpcenters.Languages() {
		assert.Len(t, strings.Fields(Center{Locale: l.Code, Lang: tr}.T("months_short")), 12, l.Code)
	}
}
