package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/markdown"
)

func TestReadableText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, src, want string
	}{
		{"keeps the lines the writer typed", "Hi,\nThanks,\nAna", "Hi,\nThanks,\nAna"},
		{"puts a blank line between paragraphs", "One.\n\nTwo.", "One.\n\nTwo."},
		{
			"follows a link with where it goes", "Read [the guide](https://x.test/a) now.",
			"Read the guide (https://x.test/a) now.",
		},
		{
			"writes a link that shows its address once", "See <https://x.test/a> and [https://x.test/b](https://x.test/b).",
			"See https://x.test/a and https://x.test/b.",
		},
		{"keeps a link inside emphasis", "*see [it](https://x.test)*", "see it (https://x.test)"},
		{"numbers a list from where it starts", "3. three\n4. four", "3. three\n4. four"},
		{"bullets an unordered list and indents what follows", "- one\n  more\n- two", "- one\n  more\n- two"},
		{"ticks task items", "- [x] done\n- [ ] open", "- [x] done\n- [ ] open"},
		{"quotes behind a marker", "> said\n> twice", "> said\n> twice"},
		{"keeps raw html as words", "Use the <div> tag", "Use the <div> tag"},
		{"keeps code as written", "```\nx := 1\n```\n\nand `y`", "x := 1\n\nand y"},
		{"writes table rows on one line each", "| a | b |\n|---|---|\n| 1 | 2 |", "a | b\n1 | 2"},
		{"marks a thematic break", "above\n\n---\n\nbelow", "above\n\n---\n\nbelow"},
		{"keeps an html block as written", "<div>\nhello\n</div>", "<div>\nhello\n</div>"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, markdown.ReadableText([]byte(c.src)))
		})
	}
}
