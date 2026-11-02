package origin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http/origin"
)

func mustParse(t *testing.T, s string) origin.Origin {
	t.Helper()

	o, err := origin.Parse(s)
	require.NoError(t, err)

	return o
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("normalizes spellings of the same origin", func(t *testing.T) {
		t.Parallel()
		cases := map[string]string{
			"https://example.com":           "https://example.com",
			"HTTPS://Example.COM/":          "https://example.com",
			"https://example.com:443":       "https://example.com",
			"http://example.com:80":         "http://example.com",
			"http://example.com:8080":       "http://example.com:8080",
			"https://example.com:0443":      "https://example.com",
			"  https://example.com  ":       "https://example.com",
			"https://bücher.example":        "https://xn--bcher-kva.example",
			"http://127.0.0.1:3000":         "http://127.0.0.1:3000",
			"http://[::1]:8080":             "http://[::1]:8080",
			"http://[2001:DB8::1]":          "http://[2001:db8::1]",
			"https://xn--bcher-kva.example": "https://xn--bcher-kva.example",
		}
		for in, want := range cases {
			assert.Equal(t, want, mustParse(t, in).String(), in)
		}
	})

	t.Run("splits scheme host and port", func(t *testing.T) {
		t.Parallel()
		o := mustParse(t, "https://App.Example.com:8443")
		assert.Equal(t, origin.Origin{Scheme: "https", Host: "app.example.com", Port: "8443"}, o)
		assert.False(t, o.IsNull())
	})

	t.Run("accepts the opaque null origin", func(t *testing.T) {
		t.Parallel()
		o := mustParse(t, "null")
		assert.True(t, o.IsNull())
		assert.Equal(t, "null", o.String())
	})

	t.Run("refuses schemes other than http and https", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{"ftp://example.com", "file:///etc/passwd", "example.com", "javascript:alert(1)"} {
			_, err := origin.Parse(in)
			require.ErrorIs(t, err, origin.ErrUnsupportedScheme, in)
		}
	})

	t.Run("refuses urls that carry more than an origin", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"https://example.com/path",
			"https://example.com/?q=1",
			"https://example.com?",
			"https://example.com#top",
			"https://user:pass@example.com",
			"https:example.com",
		} {
			_, err := origin.Parse(in)
			require.ErrorIs(t, err, origin.ErrNotOrigin, in)
		}
	})

	t.Run("refuses broken hosts and ports", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"https://",
			"https://:443",
			"https://example.com:0",
			"https://example.com:65536",
			"https://example.com:http",
			"https://exa_mple.com",
			"https://exa mple.com",
			"http://[::1",
		} {
			_, err := origin.Parse(in)
			require.ErrorIs(t, err, origin.ErrInvalid, in)
		}
	})
}

func TestMatch(t *testing.T) {
	t.Parallel()

	t.Run("matches an exact origin in any spelling", func(t *testing.T) {
		t.Parallel()
		o := mustParse(t, "https://app.example.com")
		assert.True(t, origin.Match("https://app.example.com", o))
		assert.True(t, origin.Match("HTTPS://APP.example.com:443/", o))
		assert.False(t, origin.Match("http://app.example.com", o))
		assert.False(t, origin.Match("https://app.example.com:8443", o))
		assert.False(t, origin.Match("https://example.com", o))
	})

	t.Run("matches both schemes when the pattern has none", func(t *testing.T) {
		t.Parallel()
		assert.True(t, origin.Match("example.com", mustParse(t, "https://example.com")))
		assert.True(t, origin.Match("example.com", mustParse(t, "http://example.com")))
		assert.True(t, origin.Match("example.com:8080", mustParse(t, "http://example.com:8080")))
		assert.False(t, origin.Match("example.com", mustParse(t, "http://example.com:8080")))
	})

	t.Run("matches international names against punycode", func(t *testing.T) {
		t.Parallel()
		assert.True(t, origin.Match("https://bücher.example", mustParse(t, "https://xn--bcher-kva.example")))
	})

	t.Run("matches one label below a wildcard", func(t *testing.T) {
		t.Parallel()
		pattern := "https://*.example.com"
		assert.True(t, origin.Match(pattern, mustParse(t, "https://app.example.com")))
		assert.False(t, origin.Match(pattern, mustParse(t, "https://example.com")))
		assert.False(t, origin.Match(pattern, mustParse(t, "https://a.b.example.com")))
		assert.False(t, origin.Match(pattern, mustParse(t, "https://evilexample.com")))
		assert.False(t, origin.Match(pattern, mustParse(t, "http://app.example.com")))
		assert.True(t, origin.Match("*.example.com", mustParse(t, "http://app.example.com")))
		assert.True(t, origin.Match("http://*.localhost:5173", mustParse(t, "http://app.localhost:5173")))
	})

	t.Run("never lets a wildcard cover a public suffix", func(t *testing.T) {
		t.Parallel()
		assert.False(t, origin.Match("https://*.com", mustParse(t, "https://example.com")))
		assert.False(t, origin.Match("https://*.co.uk", mustParse(t, "https://example.co.uk")))
		assert.False(t, origin.Match("https://*.github.io", mustParse(t, "https://someone.github.io")))
		assert.True(t, origin.Match("https://*.example.co.uk", mustParse(t, "https://app.example.co.uk")))
	})

	t.Run("never lets a wildcard cover an ip address", func(t *testing.T) {
		t.Parallel()
		assert.False(t, origin.Match("http://*.0.0.1", mustParse(t, "http://127.0.0.1")))
		assert.False(t, origin.Match("http://*.[::1]", mustParse(t, "http://[::1]")))
	})

	t.Run("never matches the null origin", func(t *testing.T) {
		t.Parallel()
		assert.False(t, origin.Match("null", mustParse(t, "null")))
		assert.False(t, origin.Match("https://example.com", mustParse(t, "null")))
		assert.False(t, origin.Match("example.com", origin.Origin{}))
	})

	t.Run("matches nothing with an invalid pattern", func(t *testing.T) {
		t.Parallel()
		o := mustParse(t, "https://example.com")
		for _, pattern := range []string{"", "*", "https://*", "https://example.com/path", "ftp://example.com"} {
			assert.False(t, origin.Match(pattern, o), pattern)
		}
	})
}
