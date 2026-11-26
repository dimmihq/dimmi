package markdown_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
articleFeatures is the dialect help center articles render with.
*/
func articleFeatures() markdown.Options {
	opts := allFeatures()
	opts.ImageLayout = true
	opts.Blocks = true

	return opts
}

/*
playerAddress matches the start of every allowed player address.
*/
var playerAddress = regexp.MustCompile(`^https://(www\.youtube-nocookie\.com/embed/|player\.vimeo\.com/video/|` +
	`www\.loom\.com/embed/|fast\.wistia\.net/embed/iframe/|play\.vidyard\.com/|demo\.arcade\.software/|` +
	`app\.supademo\.com/embed/|app\.storylane\.io/demo/|www\.tella\.tv/video/|capture\.navattic\.com/)`)

/*
assertSafeBlocks fails when HTML holds anything that could run script,
or a frame that is not an allowed player with its fixed attributes.
*/
func assertSafeBlocks(t *testing.T, out, source string) {
	t.Helper()

	banned := regexp.MustCompile(`^(script|object|embed|svg|math|style|link|meta|form|base)$`)
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
			if token.Data == "iframe" {
				assertFrameAttr(t, attr, source)
			}
		}
	}
}

/*
assertFrameAttr fails on a frame attribute the renderer never writes,
a frame from anywhere but a player, or a loosened sandbox.
*/
func assertFrameAttr(t *testing.T, attr html.Attribute, source string) {
	t.Helper()

	allowed := map[string]bool{
		"src": true, "title": true, "loading": true, "allow": true, "allowfullscreen": true,
		"sandbox": true, "referrerpolicy": true,
	}
	assert.True(t, allowed[attr.Key], "frame attribute %s from %q", attr.Key, source)

	switch attr.Key {
	case "src":
		assert.Regexp(t, playerAddress, attr.Val, "frame from %q", source)
	case "sandbox":
		assert.NotContains(t, attr.Val, "allow-top-navigation", "sandbox from %q", source)
	}
}

func TestBlocks(t *testing.T) {
	t.Parallel()

	t.Run("embeds a video lazily from its share address", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(), `::embed[Setting up the inbox]{src="https://youtu.be/dQw4w9WgXcQ?t=42"}`)

		assert.Equal(t, `<figure class="embed embed-youtube"><iframe `+
			`src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?start=42" title="Setting up the inbox" `+
			`loading="lazy" allow="autoplay; fullscreen; picture-in-picture; encrypted-media; clipboard-write" `+
			`allowfullscreen="" sandbox="allow-scripts allow-same-origin allow-popups allow-presentation allow-forms" `+
			`referrerpolicy="strict-origin-when-cross-origin"></iframe></figure>`+"\n", res.HTML)
		assert.Empty(t, res.Warnings)
	})

	t.Run("links an address no provider allows and warns", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(), `::embed[Our tour]{src="https://videos.example.com/tour"}`)

		assert.Equal(t,
			`<p><a href="https://videos.example.com/tour" rel="nofollow noopener ugc">Our tour</a></p>`+"\n", res.HTML)
		assert.Equal(t, []markdown.Warning{{Code: markdown.WarnEmbed, Detail: "https://videos.example.com/tour"}},
			res.Warnings)
	})

	t.Run("draws a button as a link in a row", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(),
			"::button[Start a trial]{href=\"https://example.com/start\" align=\"center\"}\n\n"+
				"::button[Billing]{href=\"/billing\"}")

		assert.Equal(t, `<p class="button-row button-align-center"><a href="https://example.com/start" `+
			`class="button" rel="nofollow noopener ugc">Start a trial</a></p>`+"\n"+
			`<p class="button-row button-align-start"><a href="/billing" class="button">Billing</a></p>`+"\n",
			res.HTML)
	})

	t.Run("passes a button's link through the rewrite hook", func(t *testing.T) {
		t.Parallel()

		opts := articleFeatures()
		opts.RewriteLink = func(l markdown.Link) markdown.Link {
			l.URL = "/hc/acme" + l.URL
			return l
		}
		res := render(t, opts, `::button[Billing]{href="/billing"}`)

		assert.Contains(t, res.HTML, `<a href="/hc/acme/billing" class="button">Billing</a>`)
	})

	t.Run("shows a button with a bad link as its label and warns", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(), `::button[Run]{href="javascript:alert(1)"}`)

		assert.Equal(t, "<p>Run</p>\n", res.HTML)
		assert.Equal(t, []markdown.Warning{{Code: markdown.WarnButtonLink, Detail: "javascript:alert(1)"}},
			res.Warnings)
	})

	t.Run("nests details and keeps fenced colons as code", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(),
			"::::details[How do refunds work?]\nFive *working* days.\n\n:::details[And partial ones?]\n"+
				"```\n:::\n```\n:::\n\nAfter.\n::::\n\nOutside.")

		assert.Equal(t, `<details class="details"><summary>How do refunds work?</summary>`+"\n"+
			"<p>Five <em>working</em> days.</p>\n"+
			`<details class="details"><summary>And partial ones?</summary>`+"\n"+
			"<pre><code>:::\n</code></pre>\n</details>\n<p>After.</p>\n</details>\n<p>Outside.</p>\n", res.HTML)
	})

	t.Run("closes a details block left open at the end", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(), ":::details[Open]\nStill inside")

		assert.Equal(t, `<details class="details"><summary>Open</summary>`+"\n<p>Still inside</p>\n</details>\n",
			res.HTML)
	})

	t.Run("captions a titled picture and keeps its link and place", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(),
			"![Plans](/plans.png#align=center \"The plans page\")\n\n"+
				"[![Plans](/plans.png \"Click to open\")](https://example.com)\n\n"+
				"![No caption](/a.png)")

		assert.Equal(t, `<figure class="image-figure image-figure-center"><img src="/plans.png" alt="Plans" `+
			`class="image-align-center" loading="lazy" decoding="async">`+
			`<figcaption>The plans page</figcaption></figure>`+"\n"+
			`<figure class="image-figure"><a href="https://example.com" rel="nofollow noopener ugc"><img src="/plans.png" `+
			`alt="Plans" loading="lazy" decoding="async"></a><figcaption>Click to open</figcaption></figure>`+"\n"+
			`<p><img src="/a.png" alt="No caption" loading="lazy" decoding="async"></p>`+"\n", res.HTML)
	})

	t.Run("leaves unknown and malformed directives as text", func(t *testing.T) {
		t.Parallel()

		res := render(t, articleFeatures(), "::video[x]{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n\n"+
			"::embed[unclosed{src=x}\n\n:::embed[leaf as container]\n\n    ::embed{src=\"https://youtu.be/dQw4w9WgXcQ\"}")

		assert.NotContains(t, res.HTML, "<iframe")
		assert.Contains(t, res.HTML, "<p>::video[x]")
		assert.Contains(t, res.HTML, "<pre><code>::embed")
	})

	t.Run("renders directives as text when blocks are off", func(t *testing.T) {
		t.Parallel()

		res := render(t, allFeatures(), `::embed{src="https://youtu.be/dQw4w9WgXcQ"}`)

		assert.NotContains(t, res.HTML, "<iframe")
		assert.Contains(t, res.HTML, "::embed")
	})

	t.Run("neutralizes every payload in the block corpus", func(t *testing.T) {
		t.Parallel()

		corpus := string(fixture(t, "xss-blocks.md"))
		r := markdown.New(articleFeatures())

		for _, payload := range append(strings.Split(corpus, "\n\n"), corpus) {
			res, err := r.HTML([]byte(payload))
			require.NoError(t, err)

			assertSafeBlocks(t, res.HTML, payload)
		}
	})

	t.Run("strips frames the renderer did not write", func(t *testing.T) {
		t.Parallel()

		out := markdown.Policy().Sanitize(`<iframe src="https://evil.example/x"></iframe>` +
			`<iframe src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ" onload="alert(1)" ` +
			`sandbox="allow-top-navigation allow-scripts" srcdoc="<script>alert(1)</script>"></iframe>`)

		assert.NotContains(t, out, "evil.example")
		assert.NotContains(t, out, "onload")
		assert.NotContains(t, out, "srcdoc")
		assert.NotContains(t, out, "allow-top-navigation")
	})

	t.Run("survives deeply nested details", func(t *testing.T) {
		t.Parallel()

		r := markdown.New(articleFeatures())
		var src strings.Builder
		for i := 2000; i >= 3; i-- {
			src.WriteString(strings.Repeat(":", i) + "details[x]\n")
		}
		for _, s := range []string{src.String(), strings.Repeat(":::details[x]\n", 2000)} {
			res, err := r.HTML([]byte(s))
			require.NoError(t, err)

			assertSafeBlocks(t, res.HTML, s[:12])
		}
	})
}

func TestResolveEmbed(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":                 "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ":                      "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":                   "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=90s":                          "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?start=90",
		"https://vimeo.com/76979871":                                  "https://player.vimeo.com/video/76979871",
		"https://vimeo.com/76979871/a1b2c3d4e5":                       "https://player.vimeo.com/video/76979871?h=a1b2c3d4e5",
		"https://player.vimeo.com/video/76979871":                     "https://player.vimeo.com/video/76979871",
		"https://www.loom.com/share/0281766fa2d04bb788eaf19e65135184": "https://www.loom.com/embed/0281766fa2d04bb788eaf19e65135184",
		"https://acme.wistia.com/medias/e4a27b971d":                   "https://fast.wistia.net/embed/iframe/e4a27b971d",
		"https://fast.wistia.net/embed/iframe/e4a27b971d":             "https://fast.wistia.net/embed/iframe/e4a27b971d",
		"https://share.vidyard.com/watch/v3x8bz9Lg8H1rA2pHP7nYA":      "https://play.vidyard.com/v3x8bz9Lg8H1rA2pHP7nYA",
		"https://app.arcade.software/share/TqmPcv0fEXrYr5LhLwTh":      "https://demo.arcade.software/TqmPcv0fEXrYr5LhLwTh?embed",
		"https://app.supademo.com/demo/clx1y2z3a0001":                 "https://app.supademo.com/embed/clx1y2z3a0001",
		"https://app.storylane.io/share/abcd1234efgh":                 "https://app.storylane.io/demo/abcd1234efgh",
		"https://www.tella.tv/video/my-walkthrough-abcd":              "https://www.tella.tv/video/my-walkthrough-abcd/embed",
		"https://capture.navattic.com/clabc123def":                    "https://capture.navattic.com/clabc123def",
	} {
		embed, ok := markdown.ResolveEmbed(raw)
		if assert.True(t, ok, raw) {
			assert.Equal(t, want, embed.Player, raw)
		}
	}

	for _, raw := range []string{
		"",
		"javascript:alert(1)",
		"ftp://youtu.be/dQw4w9WgXcQ",
		"https://youtu.be.evil.example/dQw4w9WgXcQ",
		"https://evil.example/?u=https://youtu.be/dQw4w9WgXcQ",
		"https://user:pass@youtu.be/dQw4w9WgXcQ",
		"https://youtu.be:8443/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ%22",
		"https://www.youtube.com/embed/../../evil",
		"https://vimeo.com/channels/staff",
		"https://a.b.wistia.com/medias/e4a27b971d",
		"https://wistia.com/medias/e4a27b971d",
		"https://www.loom.com/share/not-a-loom-id",
		"https://demo.arcade.software/x/y",
	} {
		_, ok := markdown.ResolveEmbed(raw)
		assert.False(t, ok, raw)
	}
}

func TestEmbedOrigins(t *testing.T) {
	t.Parallel()

	origins := markdown.EmbedOrigins()

	assert.Contains(t, origins, "https://www.youtube-nocookie.com")
	assert.Contains(t, origins, "https://player.vimeo.com")
	for _, o := range origins {
		assert.Regexp(t, `^https://[a-z0-9.-]+$`, o)
	}
}

func TestParseArticle(t *testing.T) {
	t.Parallel()

	doc := markdown.ParseArticle([]byte("::embed[Tour]{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n\n" +
		"::button[Start]{href=\"/start\"}\n\n:::details[Why?]\nBecause.\n:::\n\n![Plans](/p.png \"Caption\")"))

	require.Len(t, doc.Children, 4)
	assert.Equal(t, &markdown.Node{
		Kind: markdown.KindEmbed, Text: "Tour", URL: "https://youtu.be/dQw4w9WgXcQ",
		Children: []*markdown.Node{},
	}, doc.Children[0])
	assert.Equal(t, &markdown.Node{
		Kind: markdown.KindButton, Text: "Start", URL: "/start",
		Children: []*markdown.Node{},
	}, doc.Children[1])
	assert.Equal(t, markdown.KindDetails, doc.Children[2].Kind)
	assert.Equal(t, "Why?", doc.Children[2].Text)
	assert.Equal(t, markdown.KindParagraph, doc.Children[2].Children[0].Kind)
	assert.Equal(t, "Caption", doc.Children[3].Children[0].Title)

	plain := markdown.Parse([]byte(`::embed{src="https://youtu.be/dQw4w9WgXcQ"}`))
	assert.Equal(t, markdown.KindParagraph, plain.Children[0].Kind)
}

func TestFlattenBlocks(t *testing.T) {
	t.Parallel()

	src := "Intro\n\n::embed[Set up [fast]]{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n\n" +
		"::embed{src=\"https://vimeo.com/76979871\"}\n\n" +
		"::button[Start a trial]{href=\"https://example.com/start\"}\n\n" +
		"::::details[Refunds]\nFive days.\n\n:::details[Partial]\nTwo.\n:::\n::::\n\n" +
		"```\n::embed{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n```\n"

	assert.Equal(t, "Intro\n\n[Set up \\[fast\\]](<https://youtu.be/dQw4w9WgXcQ>)\n\n"+
		"[Video](<https://vimeo.com/76979871>)\n\n"+
		"[Start a trial](<https://example.com/start>)\n\n"+
		"**Refunds**\nFive days.\n\n**Partial**\nTwo.\n\n\n\n"+
		"```\n::embed{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n```\n", markdown.FlattenBlocks([]byte(src)))
}
