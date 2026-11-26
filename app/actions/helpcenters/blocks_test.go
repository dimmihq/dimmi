package helpcenters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArticleBlocks(t *testing.T) {
	t.Parallel()

	got := ArticleBlocks("::embed[Setup tour]{src=\"https://youtu.be/dQw4w9WgXcQ\"}\n\n"+
		"::button[Start]{href=\"https://example.com/start\"}\n\n"+
		":::details[Refunds?]\nFive days.\n\n- Card\n- Bank\n:::\n\n"+
		"::embed{src=\"https://vimeo.com/76979871\"}\n\n::button[No link]\n\n"+
		"::button[Billing]{href=\"/billing\"}\n\n::embed[Bad]{src=\"javascript:alert(1)\"}", "https://acme.dimmi.help")

	assert.Equal(t, []Block{
		{Kind: "embed", Text: "Setup tour", URL: "https://youtu.be/dQw4w9WgXcQ"},
		{Kind: "button", Text: "Start", URL: "https://example.com/start"},
		{Kind: "details", Text: "Refunds?", Blocks: []Block{
			{Kind: "p", Text: "Five days."},
			{Kind: "list", Items: []string{"Card", "Bank"}},
		}},
		{Kind: "embed", Text: "https://vimeo.com/76979871", URL: "https://vimeo.com/76979871"},
		{Kind: "p", Text: "No link"},
		{Kind: "button", Text: "Billing", URL: "https://acme.dimmi.help/billing"},
		{Kind: "p", Text: "Bad"},
	}, got)

	unrooted := ArticleBlocks("::button[Billing]{href=\"/billing\"}", "")
	assert.Equal(t, []Block{{Kind: "p", Text: "Billing"}}, unrooted)

	assert.Equal(t, []Block{{Kind: "p", Text: "::button[Start]{href=\"/x\"}"}}, Blocks("::button[Start]{href=\"/x\"}"))
}
