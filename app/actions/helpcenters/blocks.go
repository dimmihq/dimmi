/*
|--------------------------------------------------------------------------
| Article Blocks
|--------------------------------------------------------------------------
|
| The messenger shows articles as plain blocks, never as HTML, so no
| customer markup ever reaches the host page: paragraphs, headings,
| lists, numbered steps, code and notes, each only text. Formatting
| inside a paragraph is dropped; links keep their words. Code in a known
| language also comes as tokens, still only text, each marked keyword,
| string, number or comment when it is one, which the messenger colors
| like the help center does.
|
*/

package helpcenters

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
Block is one block of an article as the messenger draws it. Tokens is
code split for coloring, empty when its language is unknown.
*/
type Block struct {
	Kind   string           `json:"kind"`
	Text   string           `json:"text,omitempty"`
	Items  []string         `json:"items,omitempty"`
	Tokens []markdown.Token `json:"tokens,omitempty"`
}

/*
kinds names the highlighter's classes by the first letter they share
for the four kinds the messenger colors; every other class is plain.
*/
func kinds() map[byte]string {
	return map[byte]string{'k': "keyword", 's': "string", 'm': "number", 'c': "comment"}
}

/*
coloring keeps only the kinds the messenger colors, joining neighbors
of the same kind so the code arrives in as few runs as it can.
*/
func coloring(tokens []markdown.Token) []markdown.Token {
	var out []markdown.Token
	for _, t := range tokens {
		kind := ""
		if t.Class != "" {
			kind = kinds()[t.Class[0]]
		}
		if n := len(out); n > 0 && out[n-1].Class == kind {
			out[n-1].Text += t.Text
			continue
		}
		out = append(out, markdown.Token{Class: kind, Text: t.Text})
	}
	return out
}

/*
Blocks turns an article's Markdown into the messenger's blocks.
*/
func Blocks(body string) []Block {
	out := []Block{}
	for _, n := range markdown.Parse([]byte(body)).Children {
		if b, ok := blockOf(n); ok {
			out = append(out, b)
		}
	}
	return out
}

func blockOf(n *markdown.Node) (Block, bool) {
	switch n.Kind {
	case markdown.KindHeading:
		return Block{Kind: "h2", Text: inline(n)}, true
	case markdown.KindList:
		kind := "list"
		if n.Ordered {
			kind = "steps"
		}
		items := make([]string, 0, len(n.Children))
		for _, item := range n.Children {
			items = append(items, strings.TrimSpace(inline(item)))
		}
		return Block{Kind: kind, Items: items}, true
	case markdown.KindCodeBlock:
		text := strings.TrimRight(n.Text, "\n")
		return Block{Kind: "code", Text: text, Tokens: coloring(markdown.Tokens(n.Language, text))}, true
	case markdown.KindAlert, markdown.KindBlockquote:
		return Block{Kind: "note", Text: strings.TrimSpace(inline(n))}, true
	case markdown.KindImage:
		return Block{Kind: "image", Text: inline(n)}, true
	case markdown.KindParagraph:
		if len(n.Children) == 1 && n.Children[0].Kind == markdown.KindImage {
			return Block{Kind: "image", Text: inline(n.Children[0])}, true
		}
		text := strings.TrimSpace(inline(n))
		return Block{Kind: "p", Text: text}, text != ""
	case markdown.KindDocument, markdown.KindListItem, markdown.KindThematicBreak, markdown.KindTable,
		markdown.KindTableRow, markdown.KindTableCell, markdown.KindText, markdown.KindSoftBreak,
		markdown.KindLineBreak, markdown.KindEmphasis, markdown.KindStrong, markdown.KindStrikethrough,
		markdown.KindCode, markdown.KindLink, markdown.KindCheckbox, markdown.KindHTML:
		return Block{}, false
	}
	return Block{}, false
}

/*
inline is the plain text of a node and everything under it.
*/
func inline(n *markdown.Node) string {
	switch n.Kind {
	case markdown.KindText, markdown.KindCode, markdown.KindHTML:
		return n.Text
	case markdown.KindSoftBreak, markdown.KindLineBreak:
		return " "
	case markdown.KindDocument, markdown.KindParagraph, markdown.KindHeading, markdown.KindBlockquote,
		markdown.KindAlert, markdown.KindList, markdown.KindListItem, markdown.KindCodeBlock,
		markdown.KindThematicBreak, markdown.KindTable, markdown.KindTableRow, markdown.KindTableCell,
		markdown.KindEmphasis, markdown.KindStrong, markdown.KindStrikethrough, markdown.KindLink,
		markdown.KindImage, markdown.KindCheckbox:
		return children(n)
	}
	return children(n)
}

/*
children joins the plain text of a node's children, spacing out blocks.
*/
func children(n *markdown.Node) string {
	var b strings.Builder
	for i, c := range n.Children {
		if i > 0 && isBlock(c) {
			b.WriteByte(' ')
		}
		b.WriteString(inline(c))
	}
	return b.String()
}

func isBlock(n *markdown.Node) bool {
	return n.Kind == markdown.KindParagraph || n.Kind == markdown.KindList
}
