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
| like the help center does. Articles also carry their blocks: an embed
| and a button come as their label and the absolute address they open,
| and a details block as its summary with its own blocks inside. An
| address that is not a web or mail one never reaches the messenger.
|
*/

package helpcenters

import (
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
Block is one block of an article as the messenger draws it. Tokens is
code split for coloring, empty when its language is unknown. URL is
where an embed or a button goes, and Blocks what a details block holds.
*/
type Block struct {
	Kind   string           `json:"kind"`
	Text   string           `json:"text,omitempty"`
	Items  []string         `json:"items,omitempty"`
	Tokens []markdown.Token `json:"tokens,omitempty"`
	URL    string           `json:"url,omitempty"`
	Blocks []Block          `json:"blocks,omitempty"`
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
Blocks turns Markdown, such as a changelog entry's, into the
messenger's blocks.
*/
func Blocks(body string) []Block {
	return blocksOf(markdown.Parse([]byte(body)).Children)
}

/*
ArticleBlocks turns an article's Markdown, with its embeds, buttons and
details, into the messenger's blocks. Links on the help center's own
pages, such as "/billing", resolve against base, the help center's
address.
*/
func ArticleBlocks(body, base string) []Block {
	return absolute(blocksOf(markdown.ParseBlocks([]byte(body)).Children), base)
}

/*
absolute resolves the blocks' links against base and turns a link that
cannot reach a visitor into its label.
*/
func absolute(blocks []Block, base string) []Block {
	root, err := url.Parse(base)
	if err != nil || !root.IsAbs() {
		root = nil
	}
	for i, b := range blocks {
		b.Blocks = absolute(b.Blocks, base)
		if b.Kind != "embed" && b.Kind != "button" {
			blocks[i] = b
			continue
		}
		link, err := url.Parse(b.URL)
		switch {
		case err != nil || !markdown.SafeLink(b.URL) || (b.Kind == "embed" && !link.IsAbs()):
			b = Block{Kind: "p", Text: b.Text}
		case !link.IsAbs() && root == nil:
			b = Block{Kind: "p", Text: b.Text}
		case !link.IsAbs():
			b.URL = root.ResolveReference(link).String()
		}
		blocks[i] = b
	}
	return blocks
}

func blocksOf(nodes []*markdown.Node) []Block {
	out := []Block{}
	for _, n := range nodes {
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
	case markdown.KindEmbed, markdown.KindButton, markdown.KindDetails:
		return articleBlockOf(n)
	case markdown.KindDocument, markdown.KindListItem, markdown.KindThematicBreak, markdown.KindTable,
		markdown.KindTableRow, markdown.KindTableCell, markdown.KindText, markdown.KindSoftBreak,
		markdown.KindLineBreak, markdown.KindEmphasis, markdown.KindStrong, markdown.KindStrikethrough,
		markdown.KindCode, markdown.KindLink, markdown.KindCheckbox, markdown.KindHTML:
		return Block{}, false
	}
	return Block{}, false
}

/*
articleBlockOf turns an embed, a button or a details block into its
block. A button without a link is a paragraph of its label.
*/
func articleBlockOf(n *markdown.Node) (Block, bool) {
	switch {
	case n.Kind == markdown.KindEmbed:
		return linkBlock("embed", n), n.URL != ""
	case n.Kind == markdown.KindButton && n.URL == "":
		text := strings.TrimSpace(n.Text)
		return Block{Kind: "p", Text: text}, text != ""
	case n.Kind == markdown.KindButton:
		return linkBlock("button", n), true
	default:
		return Block{Kind: "details", Text: strings.TrimSpace(n.Text), Blocks: blocksOf(n.Children)}, true
	}
}

/*
linkBlock is an embed or a button: its label, or its address when it
has none, and the address.
*/
func linkBlock(kind string, n *markdown.Node) Block {
	text := strings.TrimSpace(n.Text)
	if text == "" {
		text = n.URL
	}
	return Block{Kind: kind, Text: text, URL: n.URL}
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
		markdown.KindImage, markdown.KindCheckbox, markdown.KindDetails:
		return children(n)
	case markdown.KindEmbed, markdown.KindButton:
		return n.Text
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
