package i18n_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"
)

func source() fstest.MapFS {
	return fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/app\n")},
		"app/auth.go": {Data: []byte(`package app

import "context"

func messages(ctx context.Context, lang interface {
	T(ctx context.Context, key string, args map[string]any) string
	Lookup(ctx context.Context, key string, args map[string]any) (string, bool)
}, key string) {
	_ = lang.T(ctx, "auth.failed", nil)
	_, _ = lang.Lookup(ctx, "items.count", nil)
	_ = T(ctx, ` + "`auth.welcome`" + `, nil)
	_ = lang.T(ctx, key, nil)
	_ = lang.T(ctx, 42, nil)
	_ = Other(ctx, "auth.ignored", nil)
	_ = func() string { return "x" }()
	_ = T(ctx)
}
`)},
		"app/auth_test.go":         {Data: []byte(`package app; var _ = T(nil, "auth.test_only", nil)`)},
		"vendor/lib/lib.go":        {Data: []byte(`package lib; var _ = T(nil, "vendor.key", nil)`)},
		"node_modules/x/x.go":      {Data: []byte(`package x; var _ = T(nil, "node.key", nil)`)},
		".git/hooks/hook.go":       {Data: []byte(`package hooks; var _ = T(nil, "hidden.key", nil)`)},
		"app/testdata/fixture.go":  {Data: []byte(`package fixture; var _ = T(nil, "fixture.key", nil)`)},
		"app/README.md":            {Data: []byte(`T(ctx, "readme.key", nil)`)},
		"app/nested/deep/thing.go": {Data: []byte(`package deep; var _ = x.T(nil, "deep.key", nil)`)},
	}
}

/*
unreadable is a directory in which one entry cannot be opened.
*/
type unreadable struct {
	files fstest.MapFS
	name  string
}

func (u unreadable) Open(name string) (fs.File, error) {
	if name == u.name {
		return nil, fs.ErrPermission
	}

	return u.files.Open(name)
}

func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("finds literal keys passed to T and Lookup", func(t *testing.T) {
		t.Parallel()

		keys, err := i18n.ScanKeys(source())

		require.NoError(t, err)
		assert.Equal(t, []string{"auth.failed", "auth.welcome", "deep.key", "items.count"}, keys)
	})

	t.Run("refuses source that does not parse", func(t *testing.T) {
		t.Parallel()

		_, err := i18n.ScanKeys(fstest.MapFS{"bad.go": {Data: []byte("package")}})

		require.ErrorContains(t, err, "parse bad.go")
	})

	t.Run("refuses source it cannot read", func(t *testing.T) {
		t.Parallel()
		files := fstest.MapFS{"bad.go": {Data: []byte("package bad")}, "locked/a.go": {Data: []byte("package a")}}

		_, err := i18n.ScanKeys(unreadable{files: files, name: "bad.go"})
		require.ErrorContains(t, err, "read bad.go")

		_, err = i18n.ScanKeys(unreadable{files: files, name: "locked"})
		require.ErrorContains(t, err, "scan message keys")
	})

	t.Run("lists the keys of every locale", func(t *testing.T) {
		t.Parallel()

		keys, err := i18n.Keys(files())

		require.NoError(t, err)
		assert.Equal(t, map[string][]string{
			"en":    {"auth.failed", "auth.throttle.seconds", "auth.welcome", "items.count"},
			"fr":    {"auth.failed", "items.count"},
			"pt-BR": {"auth.failed"},
		}, keys)
	})

	t.Run("reports keys a locale lacks and keys no code uses", func(t *testing.T) {
		t.Parallel()
		used := []string{"auth.failed", "auth.welcome", "items.count"}

		report, err := i18n.Check(used, files())

		require.NoError(t, err)
		assert.False(t, report.Clean())
		assert.Equal(t, map[string][]string{
			"fr":    {"auth.throttle.seconds", "auth.welcome"},
			"pt-BR": {"auth.throttle.seconds", "auth.welcome", "items.count"},
		}, report.Missing)
		assert.Equal(t, []string{"auth.throttle.seconds"}, report.Unused)
	})

	t.Run("passes complete and used messages", func(t *testing.T) {
		t.Parallel()
		complete := fstest.MapFS{
			"en/auth.json": {Data: []byte(`{"failed": "Wrong."}`)},
			"fr/auth.json": {Data: []byte(`{"failed": "Faux."}`)},
		}

		report, err := i18n.Check([]string{"auth.failed"}, complete)

		require.NoError(t, err)
		assert.True(t, report.Clean())
	})

	t.Run("refuses message files that do not parse", func(t *testing.T) {
		t.Parallel()
		broken := fstest.MapFS{"en/a.json": {Data: []byte(`{`)}}

		_, err := i18n.Keys(broken)
		require.ErrorContains(t, err, "parse en/a.json")

		_, err = i18n.Check(nil, broken)
		require.ErrorContains(t, err, "parse en/a.json")
	})
}
