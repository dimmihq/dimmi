package markdown_test

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
fixture reads a file from testdata.
*/
func fixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return b
}

/*
render renders source with the options and fails the test on error.
*/
func render(t *testing.T, opts markdown.Options, src string) markdown.Result {
	t.Helper()

	res, err := markdown.New(opts).HTML([]byte(src))
	require.NoError(t, err)

	return res
}

/*
allFeatures turns on every dialect feature.
*/
func allFeatures() markdown.Options {
	return markdown.Options{GFM: true, Footnotes: true, Alerts: true, HeadingIDs: true, Highlight: true}
}

/*
assertInert fails when HTML holds an element or attribute that could run
script or load a frame, judged on the parsed markup rather than text.
*/
func assertInert(t *testing.T, out, source string) {
	t.Helper()

	banned := regexp.MustCompile(`^(script|iframe|object|embed|svg|math|style|link|meta|form|base|details)$`)
	badURL := regexp.MustCompile(`(?i)^\s*(javascript|vbscript|data):`)

	tokens := html.NewTokenizer(strings.NewReader(out))
	for tt := tokens.Next(); tt != html.ErrorToken; tt = tokens.Next() {
		token := tokens.Token()
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}

		assert.NotRegexp(t, banned, token.Data, "element from %q", source)

		for _, attr := range token.Attr {
			assert.False(t, strings.HasPrefix(attr.Key, "on"), "handler %s from %q", attr.Key, source)
			assert.NotEqual(t, "style", attr.Key, "style from %q", source)

			if attr.Key == "href" || attr.Key == "src" {
				assert.NotRegexp(t, badURL, attr.Val, "url from %q", source)
			}
		}
	}
}

func TestHTML(t *testing.T) {
	t.Parallel()

	t.Run("keeps each typed line break when asked", func(t *testing.T) {
		t.Parallel()

		wrapped, err := markdown.New(markdown.Options{HardWraps: true}).HTML([]byte("Hi,\nThanks,\nAna"))
		require.NoError(t, err)
		flowing, err := markdown.New(markdown.Options{}).HTML([]byte("Hi,\nThanks,\nAna"))
		require.NoError(t, err)

		assert.Equal(t, 2, strings.Count(wrapped.HTML, "<br"))
		assert.NotContains(t, flowing.HTML, "<br")
	})

	t.Run("renders the full dialect", func(t *testing.T) {
		t.Parallel()

		_, body, err := markdown.FrontMatter(fixture(t, "guide.md"), 0)
		require.NoError(t, err)

		res, err := markdown.New(allFeatures()).HTML(body)
		require.NoError(t, err)

		for _, want := range []string{
			`<h1 id="resetting-your-password">Resetting your password</h1>`,
			`<a href="/guides/billing" title="Billing">billing</a>`,
			`<a href="https://blog.example.com/post" rel="nofollow noopener ugc">our blog</a>`,
			`<a href="mailto:support@example.com">support@example.com</a>`,
			`<a href="https://status.example.com" rel="nofollow noopener ugc">https://status.example.com</a>`,
			"<aside class=\"alert alert-warning\" role=\"note\">\n" +
				"<p>The link expires after <em>one hour</em>.</p>\n</aside>",
			"<blockquote>\n<p>A plain quote.</p>\n</blockquote>",
			`<li><input checked="" disabled="" type="checkbox"> Done</li>`,
			`<th>Plan</th>`,
			`<pre class="chroma-chroma"><code><span class="chroma-line"><span class="chroma-cl">` +
				`<span class="chroma-kd">func</span>`,
			"<pre><code>plain &lt;code&gt;\n</code></pre>",
			`<img src="/assets/reset.png" alt="Reset screen" loading="lazy" decoding="async">`,
			`<del>old</del>`,
			`<a href="#fn:1" class="footnote-ref" role="doc-noteref">1</a>`,
			`<div class="footnotes" role="doc-endnotes">`,
		} {
			assert.Contains(t, res.HTML, want)
		}

		assert.NotContains(t, res.HTML, "[!WARNING]")
		assert.NotContains(t, res.HTML, `title=""`)
	})

	t.Run("builds the table of contents", func(t *testing.T) {
		t.Parallel()

		res := render(t, allFeatures(), "# A\n\n## B c\n\n#### D\n\n## B c\n")

		assert.Equal(t, []markdown.TOCItem{{
			Title: "A",
			ID:    "a",
			Items: []markdown.TOCItem{
				{Title: "B c", ID: "b-c", Items: []markdown.TOCItem{{Title: "D", ID: "d"}}},
				{Title: "B c", ID: "b-c-1"},
			},
		}}, res.TOC)
	})

	t.Run("gives headings unique ids in any script", func(t *testing.T) {
		t.Parallel()

		res := render(t, markdown.Options{HeadingIDs: true},
			"# 日本語 見出し\n# Intro\n# Intro\n# Intro\n# !!!\n# Crème_brûlée-2\n")

		for _, want := range []string{
			`<h1 id="日本語-見出し">`,
			`<h1 id="intro">`,
			`<h1 id="intro-1">`,
			`<h1 id="intro-2">`,
			`<h1 id="heading">!!!</h1>`,
			`<h1 id="crème-brûlée-2">`,
		} {
			assert.Contains(t, res.HTML, want)
		}
	})

	t.Run("leaves headings without ids when off", func(t *testing.T) {
		t.Parallel()

		res := render(t, markdown.Options{}, "# Title\n")

		assert.Equal(t, "<h1>Title</h1>\n", res.HTML)
		assert.Equal(t, []markdown.TOCItem{{Title: "Title"}}, res.TOC)
	})

	t.Run("keeps the dialect to what is enabled", func(t *testing.T) {
		t.Parallel()

		res := render(t, markdown.Options{}, "> [!NOTE]\n> x\n\n~~s~~ https://a.example\n\n```go\nx := 1\n```\n")

		assert.Contains(t, res.HTML, "<blockquote>\n<p>[!NOTE]\nx</p>\n</blockquote>")
		assert.Contains(t, res.HTML, "~~s~~ https://a.example")
		assert.Contains(t, res.HTML, "<pre><code>x := 1\n</code></pre>")
		assert.Empty(t, res.TOC)
	})

	t.Run("recognizes every alert kind and nothing else", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []string{"NOTE", "tip", "Important", "WARNING", "caution"} {
			res := render(t, allFeatures(), "> [!"+kind+"]\n> body\n")

			assert.Contains(t, res.HTML, `<aside class="alert alert-`+strings.ToLower(kind)+`" role="note">`)
			assert.Contains(t, res.HTML, "<p>body</p>")
		}

		others := []string{"> [!DANGER]\n> x\n", "> [!NOTE] inline\n", "> **[!NOTE]**\n", "> # [!NOTE]\n", ">\n"}
		for _, src := range others {
			assert.NotContains(t, render(t, allFeatures(), src).HTML, "<aside", src)
		}

		alone := render(t, allFeatures(), "> [!TIP]\n")
		assert.Equal(t, "<aside class=\"alert alert-tip\" role=\"note\">\n</aside>\n", alone.HTML)

		nested := render(t, allFeatures(), "> outer\n>\n> > [!CAUTION]\n> > inner\n")
		assert.Contains(t, nested.HTML, "<blockquote>\n<p>outer</p>\n<aside class=\"alert alert-caution\"")
	})

	t.Run("highlights only known languages", func(t *testing.T) {
		t.Parallel()

		res := render(t, allFeatures(),
			"```no-such-language\n<b>x</b>\n```\n\n```\nplain\n```\n\n```python\ndef f(): pass\n```\n")

		assert.Contains(t, res.HTML, "<pre><code>&lt;b&gt;x&lt;/b&gt;\n</code></pre>")
		assert.Contains(t, res.HTML, "<pre><code>plain\n</code></pre>")
		assert.Contains(t, res.HTML, `<span class="chroma-k">def</span>`)
	})

	t.Run("shows raw html as text", func(t *testing.T) {
		t.Parallel()

		res := render(t, allFeatures(), "Hello <b>bold</b>\n\n<div>\n<p>block</p>\n</div>\n\n<!--\nnote\n-->\n")

		assert.Equal(t, "<p>Hello &lt;b&gt;bold&lt;/b&gt;</p>\n"+
			"<p>&lt;div&gt;\n&lt;p&gt;block&lt;/p&gt;\n&lt;/div&gt;</p>\n"+
			"<p>&lt;!--\nnote\n--&gt;</p>\n", res.HTML)
	})

	t.Run("neutralizes every payload in the xss corpus", func(t *testing.T) {
		t.Parallel()

		corpus := string(fixture(t, "xss.md"))
		r := markdown.New(allFeatures())

		for _, payload := range append(strings.Split(corpus, "\n\n"), corpus) {
			res, err := r.HTML([]byte(payload))
			require.NoError(t, err)

			assertInert(t, res.HTML, payload)
		}
	})

	t.Run("survives deeply nested and oversized input", func(t *testing.T) {
		t.Parallel()

		r := markdown.New(allFeatures())

		for _, src := range []string{
			strings.Repeat("> ", 2000) + "deep",
			strings.Repeat("- ", 2000) + "deep",
			strings.Repeat("*", 50000) + "x" + strings.Repeat("*", 50000),
			strings.Repeat("[", 50000) + "x" + strings.Repeat("](/u)", 50000),
		} {
			res, err := r.HTML([]byte(src))
			require.NoError(t, err)

			assertInert(t, res.HTML, src[:10])
		}
	})

	t.Run("sanitizes with the given policy", func(t *testing.T) {
		t.Parallel()

		res := render(t, markdown.Options{Sanitize: sanitize.Strict()}, "# Title with *style*\n")

		assert.Equal(t, "Title with style\n", res.HTML)
	})
}

func TestRewriteLink(t *testing.T) {
	t.Parallel()

	t.Run("rewrites links and images, with the policy still marking other hosts", func(t *testing.T) {
		t.Parallel()

		var seen []markdown.Link

		opts := allFeatures()
		opts.RewriteLink = func(l markdown.Link) markdown.Link {
			seen = append(seen, l)

			switch l.URL {
			case "art_1":
				l.URL = "/guides/one"
			case "https://own.example/x":
				l.Rel = ""
			case "/assets/a.png":
				l.URL = "https://cdn.example/a.png"
				l.Width, l.Height = 640, 480
			}

			return l
		}

		res := render(t, opts, "[One](art_1 \"First\") [own](https://own.example/x) ![A](/assets/a.png)")

		assert.Equal(t, `<p><a href="/guides/one" title="First">One</a> `+
			`<a href="https://own.example/x" rel="nofollow">own</a> `+
			`<img src="https://cdn.example/a.png" alt="A" loading="lazy" decoding="async" width="640" height="480">`+
			"</p>\n", res.HTML)
		assert.Equal(t, []markdown.Link{
			{URL: "art_1", Title: "First"},
			{URL: "https://own.example/x", Rel: "nofollow noopener ugc"},
			{URL: "/assets/a.png", Image: true},
		}, seen)
		assert.Empty(t, res.Warnings)
	})

	t.Run("drops links and keeps their text", func(t *testing.T) {
		t.Parallel()

		opts := markdown.Options{GFM: true}
		opts.RewriteLink = func(markdown.Link) markdown.Link {
			return markdown.Link{Warning: "broken_link"}
		}

		res := render(t, opts, "See [the *guide*](/missing), ![logo](/x.png) and https://gone.example.")

		assert.Equal(t, "<p>See the <em>guide</em>, logo and https://gone.example.</p>\n", res.HTML)
		assert.Equal(t, []markdown.Warning{
			{Code: "broken_link", Detail: "/missing"},
			{Code: "broken_link", Detail: "/x.png"},
			{Code: "broken_link", Detail: "https://gone.example"},
		}, res.Warnings)
	})

	t.Run("turns images into links", func(t *testing.T) {
		t.Parallel()

		opts := markdown.Options{}
		opts.RewriteLink = func(l markdown.Link) markdown.Link {
			l.Image = false

			return l
		}

		res := render(t, opts, "![a chart](https://other.example/c.png) ![](https://other.example/d.png)")

		assert.Equal(t, `<p><a href="https://other.example/c.png" rel="nofollow noopener ugc">a chart</a> `+
			`<a href="https://other.example/d.png" rel="nofollow noopener ugc">https://other.example/d.png</a></p>`+"\n",
			res.HTML)
	})
}

func TestWarnings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, src string
		want      []markdown.Warning
	}{
		{"clean document", "# A\n\n## B\n\n![alt](/a.png)\n", nil},
		{"empty body", "", []markdown.Warning{{Code: markdown.WarnEmptyBody}}},
		{"missing alt", "![](/a.png)", []markdown.Warning{{Code: markdown.WarnMissingAlt, Detail: "/a.png"}}},
		{
			"skipped heading",
			"## A\n\n#### B\n\n### C\n",
			[]markdown.Warning{{Code: markdown.WarnHeadingSkip, Detail: "B"}},
		},
		{"first heading may start deep", "### A\n\n#### B\n", nil},
	}

	for _, tc := range cases {
		res := render(t, markdown.Options{}, tc.src)

		assert.Equal(t, tc.want, res.Warnings, tc.name)
	}
}

func TestStyleSheet(t *testing.T) {
	t.Parallel()

	t.Run("writes css for the highlighting classes", func(t *testing.T) {
		t.Parallel()

		css, err := markdown.StyleSheet("github")
		require.NoError(t, err)

		assert.Contains(t, css, ".chroma-chroma {")
		assert.Contains(t, css, ".chroma-kd {")
	})

	t.Run("falls back to the default style", func(t *testing.T) {
		t.Parallel()

		css, err := markdown.StyleSheet("no-such-style")
		require.NoError(t, err)

		assert.Contains(t, css, ".chroma-chroma")
	})
}

func TestTokens(t *testing.T) {
	t.Parallel()

	t.Run("splits code of a known language into classed runs that rebuild it", func(t *testing.T) {
		t.Parallel()
		code := "func main() {\n\treturn 42\n}\n"

		tokens := markdown.Tokens("go", code)

		var rebuilt string
		classes := map[string]bool{}
		for _, tok := range tokens {
			rebuilt += tok.Text
			classes[tok.Class] = true
		}
		assert.Equal(t, code, rebuilt)
		assert.True(t, classes["kd"] || classes["k"])
		assert.True(t, classes["mi"])
	})

	t.Run("gives nothing for an unknown or missing language", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, markdown.Tokens("no-such-language", "x"))
		assert.Nil(t, markdown.Tokens("", "x"))
	})
}
