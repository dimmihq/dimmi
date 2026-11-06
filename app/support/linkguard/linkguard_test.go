package linkguard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/linkguard"
)

func TestFilter(t *testing.T) {
	t.Parallel()

	g := linkguard.New()
	allowed := []string{"help.lovelace.test", "*.example.com"}

	cases := []struct {
		name, in, want string
	}{
		{
			"keeps an allowed markdown link",
			"See [pricing](https://help.lovelace.test/plans).", "See [pricing](https://help.lovelace.test/plans).",
		},
		{"unwraps a foreign markdown link to its text", "Go [here](https://evil.test/login) now", "Go here now"},
		{"drops a foreign bare url", "Visit https://evil.test/x today", "Visit today"},
		{"drops a bare url with an uppercase scheme", "Visit HTTPS://evil.test/x today", "Visit today"},
		{"keeps an allowed bare url and its punctuation", "Open https://help.lovelace.test/a.", "Open https://help.lovelace.test/a."},
		{"allows a wildcard subdomain", "See https://docs.example.com/x", "See https://docs.example.com/x"},
		{"allows the wildcard's bare domain", "See https://example.com/x", "See https://example.com/x"},
		{"rejects a lookalike domain", "See https://example.com.evil.test/x", "See"},
		{"strips images even from allowed hosts", "Look ![shot](https://help.lovelace.test/a.png) here", "Look here"},
		{"strips raw html", "Hi <script>alert(1)</script><b>there</b>", "Hi alert(1)there"},
		{"unwraps a javascript link", "Click [me](javascript:alert) ok", "Click me ok"},
		{"drops a foreign autolink", "Mail <https://evil.test/x> ok", "Mail ok"},
		{"keeps text without links", "Plain text.", "Plain text."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, g.Filter(c.in, allowed))
		})
	}

	t.Run("strips every link when nothing is allowed", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "See pricing", g.Filter("See [pricing](https://help.lovelace.test/plans)", nil))
	})

	t.Run("reads the hosts of source urls", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"help.lovelace.test"}, linkguard.Hosts([]string{"https://HELP.lovelace.test/a", "::"}))
	})
}
