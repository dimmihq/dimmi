package knowledge

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
texts are the passages' texts.
*/
func texts(a Article) []string {
	var out []string
	for _, c := range cutArticle(a) {
		out = append(out, c.Text)
	}
	return out
}

func TestCutArticle(t *testing.T) {
	t.Parallel()

	t.Run("keeps short sections that belong together in one passage", func(t *testing.T) {
		t.Parallel()
		body := "# The print helper is offline\n\nYou may see this error in the dashboard as " +
			"The print helper is offline, " +
			"or as the code PRINT_HELPER_OFFLINE with HTTP status 503 when you buy labels through the API.\n\n" +
			"## What it means\n\nLabels could not be sent to your printer.\n\n" +
			"## Why it happens\n\n- The computer is asleep.\n- The helper was signed out.\n\n" +
			"## How to fix it\n\n1. Open the helper and sign in again.\n" +
			"2. Print from the browser in the meantime.\n\n" +
			"## In the API\n\n" + strings.Repeat("The response body names the error and the field. ", 8)
		chunks := cutArticle(Article{Title: "The print helper is offline", Body: body})
		require.Len(t, chunks, 2)

		first := chunks[0]
		assert.Equal(t, []string{"The print helper is offline"}, first.HeadingPath)
		assert.Contains(t, first.Text, "## Why it happens\n\n- The computer is asleep.")
		assert.Contains(t, first.Text,
			"## How to fix it\n\n1. Open the helper and sign in again.\n2. Print from the browser")
		assert.True(t, strings.HasPrefix(first.Text, "You may see this error"),
			"the intro leads, with no heading of its own")
		assert.NotContains(t, first.Text, "# The print helper is offline\n", "the title line is not text")

		assert.Equal(t, []string{"The print helper is offline", "In the API"}, chunks[1].HeadingPath)
		assert.NotContains(t, chunks[1].Text, "## In the API",
			"a passage of one section carries its heading in the path")
	})

	t.Run("leaves sections that stand alone as they are", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("Refunds reach the card in five business days. ", 10)
		chunks := cutArticle(Article{Title: "Refunds", Body: "## Timing\n\n" + long + "\n\n## Limits\n\n" + long})
		require.Len(t, chunks, 2)
		assert.Equal(t, []string{"Refunds", "Timing"}, chunks[0].HeadingPath)
		assert.Equal(t, []string{"Refunds", "Limits"}, chunks[1].HeadingPath)
	})

	t.Run("cuts a long section at its subheadings and paragraphs, never one huge passage", func(t *testing.T) {
		t.Parallel()
		paragraph := strings.Repeat("Every rate is priced on billable weight. ", 12)
		body := "## The handbook\n\nEverything a new team needs.\n\n### Weights\n\n" +
			strings.Join([]string{paragraph, paragraph, paragraph, paragraph, paragraph, paragraph}, "\n\n") +
			"\n\n### Printers\n\n" + paragraph
		chunks := cutArticle(Article{Title: "Guide", Body: body})
		require.Greater(t, len(chunks), 3)
		for _, c := range chunks {
			assert.LessOrEqual(t, utf8.RuneCountInString(c.Text), passageMax, c.HeadingPath)
		}
		assert.Equal(t, []string{"Guide", "The handbook", "Weights"}, chunks[1].HeadingPath)
		assert.Equal(t, []string{"Guide", "The handbook", "Printers"}, chunks[len(chunks)-1].HeadingPath)
		assert.Contains(t, chunks[0].Text, "### Weights",
			"the short intro joins the first part, which names its heading")
	})

	t.Run("keeps a code block whole and reads no heading inside it", func(t *testing.T) {
		t.Parallel()
		code := "```bash\n## not a heading\n\ncurl https://api.example.test\n```"
		prose := strings.Repeat("Send the request with your key. ", 40)
		body := "## Calling\n\n" + prose + "\n\n" + code + "\n\n" + prose + "\n\n" + prose
		got := texts(Article{Title: "API", Body: body})
		require.Greater(t, len(got), 1)
		whole := 0
		for _, text := range got {
			if strings.Contains(text, code) {
				whole++
			}
		}
		assert.Equal(t, 1, whole, "the fenced block sits in one passage, uncut")
	})

	t.Run("cuts text with no paragraphs, and counts characters in any script", func(t *testing.T) {
		t.Parallel()
		got := texts(Article{Title: "規約", Body: strings.Repeat("返金は五営業日以内に行われます。", 400)})
		require.Greater(t, len(got), 1)
		for _, text := range got {
			assert.LessOrEqual(t, utf8.RuneCountInString(text), passageMax)
			assert.True(t, utf8.ValidString(text), "no character is cut in half")
		}
		assert.Equal(t, strings.Repeat("返金は五営業日以内に行われます。", 400), strings.Join(got, ""), "nothing is lost")
	})

	t.Run("drops empty sections and an empty article", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, cutArticle(Article{Title: "Empty", Body: "# Empty\n\n## Nothing here\n\n"}))
	})
}
