package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
text returns a text node.
*/
func text(s string) *markdown.Node {
	return &markdown.Node{Kind: markdown.KindText, Text: s}
}

/*
node returns a node of a kind with children.
*/
func node(kind markdown.NodeKind, children ...*markdown.Node) *markdown.Node {
	return &markdown.Node{Kind: kind, Children: children}
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("describes inline markup", func(t *testing.T) {
		t.Parallel()

		got := markdown.Parse([]byte("Hi *a* **b** ~~c~~ `d` [e](/x \"T\") ![f](/i.png) <u>g</u> \\* &amp;\n" +
			"soft  \nhard https://h.example me@example.com"))

		assert.Equal(t, node(markdown.KindDocument,
			node(markdown.KindParagraph,
				text("Hi "),
				node(markdown.KindEmphasis, text("a")),
				text(" "),
				node(markdown.KindStrong, text("b")),
				text(" "),
				node(markdown.KindStrikethrough, text("c")),
				text(" "),
				&markdown.Node{Kind: markdown.KindCode, Text: "d"},
				text(" "),
				&markdown.Node{Kind: markdown.KindLink, URL: "/x", Title: "T", Children: []*markdown.Node{text("e")}},
				text(" "),
				&markdown.Node{Kind: markdown.KindImage, URL: "/i.png", Children: []*markdown.Node{text("f")}},
				text(" "),
				&markdown.Node{Kind: markdown.KindHTML, Text: "<u>"},
				text("g"),
				&markdown.Node{Kind: markdown.KindHTML, Text: "</u>"},
				text(" * &"),
				node(markdown.KindSoftBreak),
				text("soft"),
				node(markdown.KindLineBreak),
				text("hard "),
				&markdown.Node{Kind: markdown.KindLink, URL: "https://h.example", Children: []*markdown.Node{text("https://h.example")}},
				text(" "),
				&markdown.Node{
					Kind:     markdown.KindLink,
					URL:      "mailto:me@example.com",
					Children: []*markdown.Node{text("me@example.com")},
				},
			),
		), got)
	})

	t.Run("describes blocks", func(t *testing.T) {
		t.Parallel()

		got := markdown.Parse([]byte("## Title\n\n> quote\n\n> [!WARNING]\n> careful\n\n" +
			"3. three\n4. four\n\n- [x] done\n\n---\n\n```go\nx := 1\n```\n\n    indented\n\n" +
			"<div>raw</div>\n\n| a |\n| - |\n| b |\n\nNote[^1].\n\n[^1]: Footnote.\n"))

		assert.Equal(t, node(markdown.KindDocument,
			&markdown.Node{Kind: markdown.KindHeading, Level: 2, Children: []*markdown.Node{text("Title")}},
			node(markdown.KindBlockquote, node(markdown.KindParagraph, text("quote"))),
			&markdown.Node{Kind: markdown.KindAlert, Text: "warning", Children: []*markdown.Node{
				node(markdown.KindParagraph, text("careful")),
			}},
			&markdown.Node{Kind: markdown.KindList, Ordered: true, Start: 3, Children: []*markdown.Node{
				node(markdown.KindListItem, node(markdown.KindParagraph, text("three"))),
				node(markdown.KindListItem, node(markdown.KindParagraph, text("four"))),
			}},
			node(markdown.KindList, node(markdown.KindListItem, node(markdown.KindParagraph,
				&markdown.Node{Kind: markdown.KindCheckbox, Checked: true},
				text("done"),
			))),
			node(markdown.KindThematicBreak),
			&markdown.Node{Kind: markdown.KindCodeBlock, Language: "go", Text: "x := 1\n"},
			&markdown.Node{Kind: markdown.KindCodeBlock, Text: "indented\n"},
			&markdown.Node{Kind: markdown.KindHTML, Text: "<div>raw</div>"},
			node(markdown.KindTable,
				&markdown.Node{Kind: markdown.KindTableRow, Header: true, Children: []*markdown.Node{
					node(markdown.KindTableCell, text("a")),
				}},
				node(markdown.KindTableRow, node(markdown.KindTableCell, text("b"))),
			),
			node(markdown.KindParagraph, text("Note.")),
			node(markdown.KindParagraph, text("Footnote.")),
		), got)
	})

	t.Run("returns an empty document for empty source", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, &markdown.Node{Kind: markdown.KindDocument}, markdown.Parse(nil))
	})
}
