package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/integrations/email"
)

func TestStripText(t *testing.T) {
	t.Parallel()

	t.Run("cuts the quote at headers in many languages", func(t *testing.T) {
		t.Parallel()

		for _, header := range []string{
			"On Mon, Nov 2, 2026 at 10:04 AM Acme <a@acme.test> wrote:",
			"Le lun. 2 nov. 2026 à 10:04, Acme <a@acme.test> a écrit :",
			"Am Mo., 2. Nov. 2026 um 10:04 Uhr schrieb Acme <a@acme.test>:",
			"El lun, 2 nov 2026 a las 10:04, Acme escribió:",
			"Il giorno lun 2 nov 2026 Acme ha scritto:",
			"Op ma 2 nov 2026 om 10:04 schreef Acme <a@acme.test>:",
			"Em seg., 2 de nov. de 2026 às 10:04, Acme escreveu:",
			"W dniu 2.11.2026 o 10:04 Acme pisze:",
			"2 нояб. 2026 г., в 10:04, Acme написал:",
			"2026年11月2日 10:04，Acme 写道：",
			"2026年11月2日(月) 10:04 Acme <a@acme.test>のメッセージ:",
			"-----Original Message-----",
			"________________________________",
		} {
			got := email.StripText("Thanks!\n\n" + header + "\n> earlier")
			assert.Equal(t, "Thanks!", got.Visible, header)
			assert.True(t, got.Quote, header)
		}
	})

	t.Run("joins a header a client wrapped over two lines", func(t *testing.T) {
		t.Parallel()

		got := email.StripText("Sure.\n\nOn Mon, Nov 2, 2026 at 10:04 AM Bob Smith <\nbob@x.test> wrote:\n\n> hi")
		assert.Equal(t, "Sure.", got.Visible)
	})

	t.Run("cuts Outlook's header block", func(t *testing.T) {
		t.Parallel()

		got := email.StripText("OK\n\n**From:** Acme\n**Sent:** Monday\n**To:** Bob\n\nold text")
		assert.Equal(t, "OK", got.Visible)
		assert.False(t, email.StripText("From: the team\nwith love").Quote)
	})

	t.Run("cuts signatures after the delimiter or from a phone", func(t *testing.T) {
		t.Parallel()

		got := email.StripText("Hello\n-- \nBob\nCEO")
		assert.Equal(t, "Hello", got.Visible)
		assert.True(t, got.Signature)

		assert.Equal(t, "Hello", email.StripText("Hello\n\nGet Outlook for iOS").Visible)
		long := "Hello\n--\n" + strings.Repeat("line\n", 15)
		assert.Contains(t, email.StripText(long).Visible, "line")
		early := "Hi\n\nSent from my iPhone\n\nmore\ntext\nhere"
		assert.Equal(t, early, email.StripText(early).Visible)
	})

	t.Run("keeps everything rather than nothing", func(t *testing.T) {
		t.Parallel()

		got := email.StripText("> only a quote\n> nothing else")
		assert.Equal(t, "> only a quote\n> nothing else", got.Visible)
		assert.True(t, got.Fallback)
		assert.Equal(t, "", email.StripText("").Visible)
	})
}

func TestBuildBody(t *testing.T) {
	t.Parallel()

	t.Run("marks a client's quote and signature and keeps them out of the Markdown", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(`<div>New text</div><div class="gmail_signature">Bob</div>`+
			`<div class="gmail_quote">On Mon wrote:<blockquote>old</blockquote></div>`, "")

		assert.Equal(t, "New text", body.Markdown)
		assert.True(t, body.Quote)
		assert.True(t, body.Signature)
		assert.Contains(t, body.HTML, email.QuotedAttr)
		assert.Contains(t, body.HTML, email.SignatureAttr)
	})

	t.Run("keeps an answer written below a cited quote", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(`<blockquote type="cite">Question?</blockquote><div>My answer</div>`, "")

		assert.Contains(t, body.Markdown, "My answer")
		assert.Contains(t, body.Markdown, "Question?")
	})

	t.Run("drops hidden text and tracking pixels", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(`<p>Visible</p><p style="font-size:0">secret</p><div hidden>x</div>`+
			`<img src="https://t.test/p.gif" width="0"><span style="opacity:0;">y</span>`, "")

		assert.Equal(t, "Visible", body.Markdown)
		assert.NotContains(t, body.HTML, "t.test")
	})

	t.Run("falls back to the whole message when stripping leaves nothing", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(`<div class="gmail_quote">All quoted</div>`, "")

		assert.Equal(t, "All quoted", body.Markdown)
		assert.True(t, body.Fallback)
	})

	t.Run("caps very long bodies at a paragraph", func(t *testing.T) {
		t.Parallel()

		text := strings.Repeat(strings.Repeat("word ", 20)+"\n\n", 1000)
		body := email.BuildBody("", text)

		assert.True(t, body.Truncated)
		assert.LessOrEqual(t, len(body.Markdown), 64<<10)
		assert.True(t, strings.HasSuffix(body.Markdown, "[Message truncated. View original.]"))

		big := email.BuildBody("<p>"+strings.Repeat("x", 600<<10)+"</p>", "")
		assert.True(t, big.HTMLTooLarge)
		assert.Empty(t, big.HTML)
	})

	deep := func(inner string) string {
		return strings.Repeat("<div>", 600) + inner + strings.Repeat("</div>", 600)
	}

	t.Run("reads the text part when the html is nested too deep to parse", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(deep("<p>Deep words</p>"), "The order never came.")

		assert.Equal(t, "The order never came.", body.Markdown)
		assert.Empty(t, body.HTML)
	})

	t.Run("strips the tags of html nested too deep to parse when there is no text part", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(deep("<p>The order</p><p>never &amp; came.</p>"), "")

		assert.Equal(t, "The order\n\nnever & came.", strings.ReplaceAll(body.Markdown, "\n\n\n", "\n\n"))
	})

	t.Run("does not reveal hidden text by nesting html too deep to parse", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(deep(`<p>Visible</p><div style="display:none">secret <div>nested</div> more</div>`+
			`<span hidden>other</span><style>p{}</style>`), "")

		assert.Equal(t, "Visible", body.Markdown)
	})

	t.Run("reads the text part when the html is an empty shell", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody("<html><head><title>x</title></head><body>\n</body></html>", "Real text")

		assert.Equal(t, "Real text", body.Markdown)
		assert.Empty(t, body.HTML)
	})

	t.Run("reads the text part when all of the html is hidden", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody(`<body><div style="display:none">hidden</div></body>`, "Real text")

		assert.Equal(t, "Real text", body.Markdown)
	})

	t.Run("stays empty when neither the html nor the text says anything", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, email.BuildBody("<body> </body>", " \n").Markdown)
	})

	t.Run("keeps a second body part below the first, whatever precedes it", func(t *testing.T) {
		t.Parallel()

		body := email.BuildBody("", "First para\n\nSecond para")

		assert.Equal(t, "First para\n\nSecond para", body.Markdown)
		assert.False(t, body.Signature)
	})
}
