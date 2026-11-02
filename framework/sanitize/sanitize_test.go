package sanitize_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/sanitize"
)

func TestSanitize(t *testing.T) {
	t.Parallel()

	t.Run("neutralizes every payload in the xss corpus", func(t *testing.T) {
		t.Parallel()

		corpus, err := os.ReadFile("testdata/xss.txt")
		require.NoError(t, err)

		dangerous := regexp.MustCompile(
			`(?i)<script|<iframe|<object|<embed|<svg|<math|<style|<link|<meta|<form|<base|<input|javascript:|` +
				`vbscript:|data:text|\son[a-z]+=|style=|formaction|srcset`,
		)

		policies := map[string]*sanitize.Policy{
			"ugc":    sanitize.UGC(),
			"strict": sanitize.Strict(),
			"empty":  sanitize.NewPolicy(),
		}

		for name, policy := range policies {
			for payload := range strings.Lines(string(corpus)) {
				got := policy.Sanitize(payload)

				assert.NotRegexp(t, dangerous, got, "%s policy on %q", name, payload)
			}
		}
	})

	t.Run("keeps the rich text people write", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name, in, want string
		}{
			{"formatting", `<p><strong>bold</strong> <em>it</em></p>`, `<p><strong>bold</strong> <em>it</em></p>`},
			{"relative link stays followed", `<a href="/docs">docs</a>`, `<a href="/docs">docs</a>`},
			{
				"external link is not vouched for",
				`<a href="https://other.example/x">x</a>`,
				`<a href="https://other.example/x" rel="nofollow">x</a>`,
			},
			{
				"restrictive rel is kept",
				`<a href="https://other.example" rel="nofollow noopener ugc">x</a>`,
				`<a href="https://other.example" rel="nofollow noopener ugc">x</a>`,
			},
			{
				"permissive rel is dropped",
				`<a href="/a" rel="opener">x</a>`,
				`<a href="/a">x</a>`,
			},
			{"phone link", `<a href="tel:+15550100">call</a>`, `<a href="tel:+15550100">call</a>`},
			{"mail link", `<a href="mailto:a@example.com">mail</a>`, `<a href="mailto:a@example.com">mail</a>`},
			{"unknown scheme", `<a href="ftp://example.com/f">f</a>`, `f`},
			{"class is stripped", `<p class="x">t</p>`, `<p>t</p>`},
			{"unclosed markup stays unclosed", `<p><em>open`, `<p><em>open`},
		}

		policy := sanitize.UGC()

		for _, tc := range cases {
			assert.Equal(t, tc.want, policy.Sanitize(tc.in), tc.name)
		}
	})

	t.Run("keeps only escaped text under the strict policy", func(t *testing.T) {
		t.Parallel()

		got := sanitize.Strict().Sanitize(`<b>Tom</b> & <i>Jerry</i>`)

		assert.Equal(t, `Tom &amp; Jerry`, got)
	})

	t.Run("builds a policy from an empty allowlist", func(t *testing.T) {
		t.Parallel()

		policy := sanitize.NewPolicy().
			AllowElements("table", "tr", "td", "p").
			AllowAttrs("align").Matching(regexp.MustCompile(`^(left|right|center)$`)).OnElements("td").
			AllowAttrs("title").Globally().
			AllowAttrs("src", "alt").OnElements("img").
			AllowAttrs("href").OnElements("a").
			AllowStyles("color").Matching(regexp.MustCompile(`^#[0-9a-f]{6}$`)).OnElements("p").
			AllowStyles("text-align").Globally().
			AllowURLSchemes("https", "cid").
			AllowDataURIImages()

		cases := []struct {
			name, in, want string
		}{
			{"allowed elements", `<table><tr><td>1</td></tr></table>`, `<table><tr><td>1</td></tr></table>`},
			{"matching attribute", `<td align="left">x</td>`, `<td align="left">x</td>`},
			{"attribute outside the pattern", `<td align="javascript">x</td>`, `<td>x</td>`},
			{"global attribute", `<p title="t">x</p>`, `<p title="t">x</p>`},
			{"matching style", `<p style="color: #ff0000">x</p>`, `<p style="color: #ff0000">x</p>`},
			{"style outside the pattern", `<p style="color: red">x</p>`, `<p>x</p>`},
			{"global style", `<td style="text-align: center">x</td>`, `<td style="text-align: center">x</td>`},
			{"unlisted style", `<p style="position: fixed">x</p>`, `<p>x</p>`},
			{"content id image", `<img src="cid:logo" alt="l">`, `<img src="cid:logo" alt="l">`},
			{
				"data image",
				`<img src="data:image/png;base64,iVBORw0KGgo=">`,
				`<img src="data:image/png;base64,iVBORw0KGgo=">`,
			},
			{"plain http is not listed", `<a href="http://example.com">x</a>`, `x`},
			{"unlisted element", `<div>x</div>`, `x`},
		}

		for _, tc := range cases {
			assert.Equal(t, tc.want, policy.Sanitize(tc.in), tc.name)
		}
	})

	t.Run("matches attributes and styles without patterns", func(t *testing.T) {
		t.Parallel()

		policy := sanitize.NewPolicy().
			AllowAttrs("lang").OnElements("p").
			AllowStyles("color").OnElements("p")

		got := policy.Sanitize(`<p lang="en" style="color: red">x</p>`)

		assert.Equal(t, `<p lang="en" style="color: red">x</p>`, got)
	})
}
