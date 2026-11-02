package customdomain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	valid := map[string]string{
		"docs.example.com":         "docs.example.com",
		"  Docs.Example.COM.  ":    "docs.example.com",
		"café.example":             "xn--caf-dma.example",
		"hilfe.straße.de":          "hilfe.xn--strae-oqa.de",
		"example.com":              "example.com",
		"help.xn--caf-dma.example": "help.xn--caf-dma.example",
	}

	for input, want := range valid {
		t.Run("accepts "+input, func(t *testing.T) {
			t.Parallel()

			got, err := customdomain.Normalize(input)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}

	invalid := []string{
		"",
		"   ",
		"localhost",
		"docs.example.com:8443",
		"https://docs.example.com",
		"*.example.com",
		"user@example.com",
		"192.0.2.1",
		"2001:db8::1",
		"co.uk",
		"com",
		"docs_site.example.com",
		"-docs.example.com",
		strings.Repeat("a", 64) + ".example.com",
		strings.Repeat("abcdefghi.", 26) + "com",
	}

	for _, input := range invalid {
		t.Run("rejects "+input, func(t *testing.T) {
			t.Parallel()

			_, err := customdomain.Normalize(input)

			require.ErrorIs(t, err, customdomain.ErrInvalidHostname)
		})
	}
}

func TestIsApex(t *testing.T) {
	t.Parallel()

	t.Run("tells a registrable domain from a name under one", func(t *testing.T) {
		t.Parallel()

		assert.True(t, customdomain.IsApex("example.com"))
		assert.True(t, customdomain.IsApex("example.co.uk"))
		assert.False(t, customdomain.IsApex("docs.example.com"))
		assert.False(t, customdomain.IsApex("docs.example.co.uk"))
		assert.False(t, customdomain.IsApex("co.uk"))
	})
}
