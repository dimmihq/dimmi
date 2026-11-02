package i18n_test

import (
	"cmp"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"
)

func files() fstest.MapFS {
	return fstest.MapFS{
		"en/auth.json": {Data: []byte(`{
			"failed": "These credentials do not match.",
			"welcome": "Welcome, {{.name}}.",
			"throttle": {"seconds": "Try again in {{.count}} seconds."}
		}`)},
		"en/items.json":   {Data: []byte(`{"count": {"one": "{{.count}} item", "other": "{{.count}} items"}}`)},
		"fr/auth.json":    {Data: []byte(`{"failed": "Ces identifiants ne correspondent pas."}`)},
		"fr/items.json":   {Data: []byte(`{"count": {"one": "{{.count}} élément", "other": "{{.count}} éléments"}}`)},
		"pt-BR/auth.json": {Data: []byte(`{"failed": "Essas credenciais não correspondem."}`)},
		"README.md":       {Data: []byte("not a message file")},
	}
}

func newTranslator(t *testing.T) *i18n.Translator {
	t.Helper()

	translator, err := i18n.New(i18n.Config{Locale: "en"}, files())
	require.NoError(t, err)

	return translator
}

func TestTranslator(t *testing.T) {
	t.Parallel()

	t.Run("translates into the default locale when the context names none", func(t *testing.T) {
		t.Parallel()

		got := newTranslator(t).T(t.Context(), "auth.failed", nil)

		assert.Equal(t, "These credentials do not match.", got)
	})

	t.Run("translates into the locale of the context", func(t *testing.T) {
		t.Parallel()
		ctx := i18n.WithLocale(t.Context(), "fr")

		got := newTranslator(t).T(ctx, "auth.failed", nil)

		assert.Equal(t, "Ces identifiants ne correspondent pas.", got)
		assert.Equal(t, "fr", i18n.Locale(ctx))
	})

	t.Run("fills in named arguments", func(t *testing.T) {
		t.Parallel()

		got := newTranslator(t).T(t.Context(), "auth.welcome", i18n.Args{"name": "Ana"})

		assert.Equal(t, "Welcome, Ana.", got)
	})

	t.Run("picks the plural form from the count", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)
		fr := i18n.WithLocale(t.Context(), "fr")

		assert.Equal(t, "1 item", translator.T(t.Context(), "items.count", i18n.Args{"count": 1}))
		assert.Equal(t, "3 items", translator.T(t.Context(), "items.count", i18n.Args{"count": 3}))
		assert.Equal(t, "0 élément", translator.T(fr, "items.count", i18n.Args{"count": 0}))
		assert.Equal(t, "2 éléments", translator.T(fr, "items.count", i18n.Args{"count": 2}))
	})

	t.Run("picks the plural form of decimals and of counts written as text", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		assert.Equal(t, "1 item", translator.T(t.Context(), "items.count", i18n.Args{"count": "1"}))
		assert.Equal(t, "1.5 items", translator.T(t.Context(), "items.count", i18n.Args{"count": 1.5}))
		assert.Equal(t, "1.0 items", translator.T(t.Context(), "items.count", i18n.Args{"count": "1.0"}))
		assert.Equal(t, "-1 item", translator.T(t.Context(), "items.count", i18n.Args{"count": int64(-1)}))
		assert.Equal(t, "2 items", translator.T(t.Context(), "items.count", i18n.Args{"count": float32(2)}))
		assert.Equal(t, "items.count", translator.T(t.Context(), "items.count", i18n.Args{"count": "1.x"}))
	})

	t.Run("uses the other form in a locale without plural rules", func(t *testing.T) {
		t.Parallel()
		klingon := fstest.MapFS{"tlh/items.json": {Data: []byte(`{"count": {"one": "wa'", "other": "{{.count}}"}}`)}}

		translator, err := i18n.New(i18n.Config{Locale: "en"}, klingon)
		require.NoError(t, err)

		assert.Equal(t, "1", translator.T(i18n.WithLocale(t.Context(), "tlh"), "items.count", i18n.Args{"count": 1}))
	})

	t.Run("uses the other form when the message lacks the picked one", func(t *testing.T) {
		t.Parallel()
		partial := fstest.MapFS{"en/items.json": {Data: []byte(
			`{"count": {"other": "{{.count}} things"}, "few": {"one": "x"}}`,
		)}}

		translator, err := i18n.New(i18n.Config{Locale: "en"}, partial)
		require.NoError(t, err)

		assert.Equal(t, "1 things", translator.T(t.Context(), "items.count", i18n.Args{"count": 1}))
		assert.Equal(t, "items.few", translator.T(t.Context(), "items.few", i18n.Args{"count": 2}))
		assert.Equal(t, "items.few", translator.T(t.Context(), "items.few", nil))
	})

	t.Run("keeps placeholders it cannot fill as written", func(t *testing.T) {
		t.Parallel()
		odd := fstest.MapFS{"en/odd.json": {Data: []byte(`{
			"spaced": "Hi {{ .name }}, {{.name}}!",
			"missing": "Hi {{.name}}.",
			"logic": "{{if .name}}x{{end}}",
			"open": "Hi {{.name"
		}`)}}

		translator, err := i18n.New(i18n.Config{Locale: "en"}, odd)
		require.NoError(t, err)

		args := i18n.Args{"name": "Ana"}
		assert.Equal(t, "Hi Ana, Ana!", translator.T(t.Context(), "odd.spaced", args))
		assert.Equal(t, "Hi {{.name}}.", translator.T(t.Context(), "odd.missing", nil))
		assert.Equal(t, "{{if .name}}x{{end}}", translator.T(t.Context(), "odd.logic", args))
		assert.Equal(t, "Hi {{.name", translator.T(t.Context(), "odd.open", args))
	})

	t.Run("reads nested keys by their dotted path", func(t *testing.T) {
		t.Parallel()

		got := newTranslator(t).T(t.Context(), "auth.throttle.seconds", i18n.Args{"count": 30})

		assert.Equal(t, "Try again in 30 seconds.", got)
	})

	t.Run("falls back to the default locale for a missing message", func(t *testing.T) {
		t.Parallel()
		ctx := i18n.WithLocale(t.Context(), "fr")

		got := newTranslator(t).T(ctx, "auth.welcome", i18n.Args{"name": "Ana"})

		assert.Equal(t, "Welcome, Ana.", got)
	})

	t.Run("serves a regional locale from its closest match", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		assert.Equal(t, "Essas credenciais não correspondem.",
			translator.T(i18n.WithLocale(t.Context(), "pt-BR"), "auth.failed", nil))
		assert.Equal(t, "Ces identifiants ne correspondent pas.",
			translator.T(i18n.WithLocale(t.Context(), "fr-CA"), "auth.failed", nil))
	})

	t.Run("returns the key for a message no locale has", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		assert.Equal(t, "auth.unknown", translator.T(t.Context(), "auth.unknown", nil))
		assert.Equal(t, "items.count", translator.T(t.Context(), "items.count", i18n.Args{"count": "many"}))
	})

	t.Run("negotiates the best loaded locale for a request", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		assert.Equal(t, "fr", translator.Negotiate("fr-CH, fr;q=0.9, en;q=0.8"))
		assert.Equal(t, "pt-BR", translator.Negotiate("pt-BR"))
		assert.Equal(t, "en", translator.Negotiate("de-DE, de;q=0.9"))
		assert.Equal(t, "en", translator.Negotiate(""))
		assert.Equal(t, "en", translator.Negotiate("en;q=nope"))
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		locale string
		files  fstest.MapFS
		err    string
	}{
		"refuses an invalid default locale": {
			locale: "not a locale",
			err:    `default locale "not a locale"`,
		},
		"refuses a folder that is not a locale": {
			files: fstest.MapFS{"x_y!/a.json": {Data: []byte(`{}`)}},
			err:   "locale of x_y!/a.json",
		},
		"refuses a file that is not json": {
			files: fstest.MapFS{"en/a.json": {Data: []byte(`{`)}},
			err:   "parse en/a.json",
		},
		"refuses a message that is not text": {
			files: fstest.MapFS{"en/a.json": {Data: []byte(`{"b": {"c": 1}}`)}},
			err:   "parse en/a.json: a.b.c: i18n: message is not text",
		},
		"refuses plural forms mixed with other keys": {
			files: fstest.MapFS{"en/a.json": {Data: []byte(`{"b": {"one": "x", "label": "y"}}`)}},
			err:   "parse en/a.json: a.b: i18n: plural forms mixed with other keys",
		},
		"refuses an unreadable file": {
			files: fstest.MapFS{"en/a.json": {Mode: fs.ModeDir}},
			err:   "read en/a.json",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := i18n.New(i18n.Config{Locale: cmp.Or(tc.locale, "en")}, tc.files)

			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestTranslatorLocales(t *testing.T) {
	t.Parallel()

	t.Run("serves only the configured locales, the default first", func(t *testing.T) {
		t.Parallel()

		translator, err := i18n.New(i18n.Config{Locale: "en", Locales: []string{"fr", "en", "de"}}, files())
		require.NoError(t, err)

		assert.Equal(t, []string{"en", "fr", "de"}, translator.Locales())
		assert.Equal(t, "en", translator.Negotiate("pt-BR"))
		assert.Equal(t, "de", translator.Negotiate("de-AT"))
	})

	t.Run("matches a preferred locale to a served one", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		locale, ok := translator.Match("fr-CA")
		assert.True(t, ok)
		assert.Equal(t, "fr", locale)

		for _, preferred := range []string{"de", "", "not a locale"} {
			_, ok := translator.Match(preferred)
			assert.False(t, ok, preferred)
		}
	})

	t.Run("lets later files replace messages of earlier ones", func(t *testing.T) {
		t.Parallel()
		app := fstest.MapFS{"en/auth.json": {Data: []byte(`{"failed": "Wrong email or password."}`)}}

		translator, err := i18n.New(i18n.Config{Locale: "en"}, files(), app)
		require.NoError(t, err)

		assert.Equal(t, "Wrong email or password.", translator.T(t.Context(), "auth.failed", nil))
		assert.Equal(t, "Welcome, Ana.", translator.T(t.Context(), "auth.welcome", i18n.Args{"name": "Ana"}))
	})

	t.Run("falls back to english when the default locale lacks a message", func(t *testing.T) {
		t.Parallel()

		translator, err := i18n.New(i18n.Config{Locale: "fr"}, files())
		require.NoError(t, err)

		de := i18n.WithLocale(t.Context(), "de")
		assert.Equal(t, "Ces identifiants ne correspondent pas.", translator.T(de, "auth.failed", nil))
		assert.Equal(t, "Welcome, Ana.", translator.T(de, "auth.welcome", i18n.Args{"name": "Ana"}))
	})

	t.Run("tells whether a message exists", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)

		message, ok := translator.Lookup(t.Context(), "auth.failed", nil)
		assert.True(t, ok)
		assert.Equal(t, "These credentials do not match.", message)

		_, ok = translator.Lookup(t.Context(), "auth.unknown", nil)
		assert.False(t, ok)
	})

	t.Run("refuses a configured locale that is not one", func(t *testing.T) {
		t.Parallel()

		_, err := i18n.New(i18n.Config{Locale: "en", Locales: []string{"x_y!"}}, files())

		require.ErrorContains(t, err, `supported locale "x_y!"`)
	})
}

func TestContext(t *testing.T) {
	t.Parallel()

	t.Run("translates with the translator the context carries", func(t *testing.T) {
		t.Parallel()
		translator := newTranslator(t)
		ctx := i18n.WithLocale(i18n.WithTranslator(t.Context(), translator), "fr")

		assert.Same(t, translator, i18n.From(ctx))
		assert.Equal(t, "Ces identifiants ne correspondent pas.", i18n.T(ctx, "auth.failed", nil))
	})

	t.Run("returns the key when the context carries no translator", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, i18n.From(t.Context()))
		assert.Equal(t, "auth.failed", i18n.T(t.Context(), "auth.failed", nil))
		assert.Empty(t, i18n.Locale(t.Context()))
	})
}
