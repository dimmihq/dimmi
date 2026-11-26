package dom_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dom"
	"github.com/dimmihq/dimmi/framework/markdown"
)

func TestArticleMarkdown(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ html, want string }{
		"button": {
			`<p>Ready?</p><a href="https://example.com/start?a=1&amp;b=2" class="intercom-h2b-button">Start [now]</a>`,
			"Ready?\n\n::button[Start \\[now\\]]{href=\"https://example.com/start?a=1&b=2\"}",
		},
		"centered button": {
			`<div class="intercom-align-center"><a href="https://example.com" class="intercom-h2b-button">Go</a></div>`,
			`::button[Go]{href="https://example.com" align="center"}`,
		},
		"video": {
			`<iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" title="Demo"></iframe>`,
			`::embed[Demo]{src="https://www.youtube.com/embed/dQw4w9WgXcQ"}`,
		},
		"protocol-relative video": {
			`<iframe src="//player.vimeo.com/video/76979871"></iframe>`,
			`::embed[]{src="https://player.vimeo.com/video/76979871"}`,
		},
		"centered picture": {
			`<div class="intercom-align-center"><img src="https://cdn.example.com/a.png" alt="A"></div>`,
			"![A](https://cdn.example.com/a.png#align=center)",
		},
		"yellow callout": {
			`<div class="intercom-interblocks-callout" style="background-color: #feedaf80; border-color: #fbc91633;">` +
				`<p>Careful <b>now</b></p><p>Two</p></div>`,
			"> [!WARNING]\n> Careful **now**\n>\n> Two",
		},
		"red callout": {
			`<div class="intercom-interblocks-callout" style="background-color:#fde2e1"><p>x</p></div>`,
			"> [!CAUTION]\n> x",
		},
		"green callout": {
			`<div class="intercom-interblocks-callout" style="background-color:#d7efdc"><p>x</p></div>`,
			"> [!TIP]\n> x",
		},
		"blue callout": {
			`<div class="intercom-interblocks-callout" style="background-color:#e3e7fa"><p>x</p></div>`,
			"> [!NOTE]\n> x",
		},
		"purple callout": {
			`<div class="intercom-interblocks-callout" style="background-color:#ece2fa"><p>x</p></div>`,
			"> [!IMPORTANT]\n> x",
		},
		"grey callout": {
			`<div class="intercom-interblocks-callout" style="background-color:#f1f1f1"><p>x</p></div>`,
			"> [!NOTE]\n> x",
		},
		"nested sections": {
			`<details><summary>Q one</summary><p>Answer</p>` +
				`<details><summary>Inner</summary><p>Deep</p></details></details>`,
			"::::details[Q one]\nAnswer\n\n:::details[Inner]\nDeep\n:::\n::::",
		},
		"caption": {
			`<figure><img src="/b.png" alt="B"><figcaption>The <b>caption</b></figcaption></figure>`,
			`![B](/b.png "The caption")`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := dom.ToArticleMarkdown(tc.html)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestArticleRoundTrip(t *testing.T) {
	t.Parallel()

	r := markdown.New(markdown.Options{
		GFM: true, Footnotes: true, Alerts: true, HeadingIDs: true, ImageLayout: true, Blocks: true,
	})
	src := "## Getting started\n\n" +
		"::embed[Setup tour]{src=\"https://www.youtube.com/watch?v=dQw4w9WgXcQ\"}\n\n" +
		"::button[Start a trial]{href=\"https://example.com/start\" align=\"center\"}\n\n" +
		"> [!TIP]\n> Keep it short.\n\n" +
		"::::details[Refunds]\nFive days.\n\n:::details[Partial]\nTwo.\n:::\n::::\n\n" +
		":::center\n## Welcome\n\nCentered words.\n:::\n\n" +
		"![Plans](/plans.png#width=60&align=center \"The plans page\")"

	first, err := r.HTML([]byte(src))
	require.NoError(t, err)

	back, err := dom.ToArticleMarkdown(first.HTML)
	require.NoError(t, err)

	second, err := r.HTML([]byte(back))
	require.NoError(t, err)
	assert.Equal(t, first.HTML, second.HTML, "markdown after the trip:\n%s", back)
}

func TestArticleMarkdownHostile(t *testing.T) {
	t.Parallel()

	r := markdown.New(markdown.Options{GFM: true, Alerts: true, Blocks: true})
	for _, html := range []string{
		`<iframe src="javascript:alert(1)"></iframe>`,
		`<iframe src="https://evil.example/x" onload="alert(1)"></iframe>`,
		`<a class="intercom-h2b-button" href="javascript:alert(1)">x</a>`,
		`<a class="intercom-h2b-button" href="https://x.com&quot; onclick=&quot;alert(1)">x</a>`,
		`<details><summary>]{src="javascript:alert(1)"}</summary><script>alert(1)</script></details>`,
		`<div class="intercom-interblocks-callout" style="background-color:url(javascript:alert(1))"><p>x</p></div>`,
	} {
		md, err := dom.ToArticleMarkdown(html)
		require.NoError(t, err)

		res, err := r.HTML([]byte(md))
		require.NoError(t, err)
		assert.NotContains(t, res.HTML, `href="javascript`, html)
		assert.NotContains(t, res.HTML, `src="javascript`, html)
		assert.NotContains(t, res.HTML, "onclick", html)
		assert.NotContains(t, res.HTML, "<script", html)
		assert.NotContains(t, res.HTML, "evil.example\"", html)
	}
}

func TestArticleCenteredText(t *testing.T) {
	t.Parallel()

	got, err := dom.ToArticleMarkdown(`<h2 class="intercom-align-center">Welcome</h2>` +
		`<p class="intercom-align-center">Two lines</p><p>Left</p>` +
		`<details><summary>Q</summary><p class="intercom-align-center">Mid</p></details>`)
	require.NoError(t, err)

	assert.Equal(t, ":::center\n## Welcome\n\nTwo lines\n:::\n\nLeft\n\n"+
		"::::details[Q]\n:::center\nMid\n:::\n::::", got)
}
