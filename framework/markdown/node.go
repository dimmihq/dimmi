/*
|--------------------------------------------------------------------------
| Neutral Syntax Tree
|--------------------------------------------------------------------------
|
| Chat platforms each speak their own Markdown-like markup, and none of
| them speaks HTML. Converters for them walk this tree: a small, stable
| description of a document that does not change when the Markdown
| engine underneath does. Adjacent runs of text are merged, so a
| converter sees "a [b" as one text node rather than three.
|
*/

package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

/*
NodeKind is the kind of a node in the neutral syntax tree.
*/
type NodeKind int

/*
The kinds of nodes. Block kinds hold other blocks or inline nodes;
inline kinds hold inline nodes or, for Text, Code and HTML, only text.
*/
const (
	KindDocument NodeKind = iota
	KindParagraph
	KindHeading
	KindBlockquote
	KindAlert
	KindList
	KindListItem
	KindCodeBlock
	KindThematicBreak
	KindTable
	KindTableRow
	KindTableCell
	KindText
	KindSoftBreak
	KindLineBreak
	KindEmphasis
	KindStrong
	KindStrikethrough
	KindCode
	KindLink
	KindImage
	KindCheckbox
	KindHTML
)

/*
Node is one node of the neutral syntax tree. Only the fields that belong
to its kind are set:

  - Text: the text of Text, Code, CodeBlock and HTML nodes, and the
    lowercase kind of an Alert, such as "warning".
  - Level: the level of a Heading, 1 to 6.
  - URL and Title: the target of a Link or Image; the children of an
    Image are its alt text.
  - Language: the language of a fenced CodeBlock.
  - Ordered and Start: whether a List is numbered and its first number.
  - Header: whether a TableRow is the header row.
  - Checked: whether a Checkbox of a task list item is ticked.
*/
type Node struct {
	Kind     NodeKind
	Text     string
	Level    int
	URL      string
	Title    string
	Language string
	Ordered  bool
	Start    int
	Header   bool
	Checked  bool
	Children []*Node
}

/*
Parse reads Markdown source in the GFM dialect with footnotes and
alerts and returns its neutral syntax tree. Raw HTML in the source
becomes HTML nodes that converters should treat as text. Footnote
references are dropped and footnote texts follow the document as
paragraphs.
*/
func Parse(src []byte) *Node {
	root := &Node{Kind: KindDocument}
	root.Children = convertChildren(parseGFM(src), src)

	return root
}

/*
convertChildren converts the children of n. Nodes without a neutral
kind are replaced by their own converted children.
*/
func convertChildren(n ast.Node, src []byte) []*Node {
	var out []*Node

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		node, ok := convertBlock(c, src)
		if !ok {
			node, ok = convertInline(c, src)
		}

		if !ok {
			out = appendNodes(out, convertChildren(c, src)...)

			continue
		}

		if node.Children == nil && hasChildren(node.Kind) {
			node.Children = convertChildren(c, src)
		}

		out = appendNodes(out, node)

		if brk := lineBreak(c); brk != nil {
			out = append(out, brk)
		}
	}

	return out
}

/*
convertBlock converts a block node.
*/
func convertBlock(n ast.Node, src []byte) (*Node, bool) {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return &Node{Kind: KindParagraph}, true
	case *ast.Heading:
		return &Node{Kind: KindHeading, Level: n.Level}, true
	case *ast.Blockquote:
		if kind, ok := alertOf(n); ok {
			return &Node{Kind: KindAlert, Text: kind}, true
		}

		return &Node{Kind: KindBlockquote}, true
	case *ast.List:
		return &Node{Kind: KindList, Ordered: n.IsOrdered(), Start: n.Start}, true
	case *ast.ListItem:
		return &Node{Kind: KindListItem}, true
	case *ast.FencedCodeBlock:
		return &Node{Kind: KindCodeBlock, Text: linesText(n, src), Language: string(n.Language(src))}, true
	case *ast.CodeBlock:
		return &Node{Kind: KindCodeBlock, Text: linesText(n, src)}, true
	case *ast.HTMLBlock:
		return &Node{Kind: KindHTML, Text: htmlBlockText(n, src)}, true
	case *ast.ThematicBreak:
		return &Node{Kind: KindThematicBreak}, true
	default:
		return convertTable(n)
	}
}

/*
convertTable converts the parts of a table.
*/
func convertTable(n ast.Node) (*Node, bool) {
	switch n.(type) {
	case *extast.Table:
		return &Node{Kind: KindTable}, true
	case *extast.TableHeader:
		return &Node{Kind: KindTableRow, Header: true}, true
	case *extast.TableRow:
		return &Node{Kind: KindTableRow}, true
	case *extast.TableCell:
		return &Node{Kind: KindTableCell}, true
	default:
		return nil, false
	}
}

/*
convertInline converts an inline node.
*/
func convertInline(n ast.Node, src []byte) (*Node, bool) {
	switch n := n.(type) {
	case *ast.Text:
		return &Node{Kind: KindText, Text: string(unescape(n.Segment.Value(src)))}, true
	case *ast.String:
		return &Node{Kind: KindText, Text: string(n.Value)}, true
	case *ast.Emphasis:
		if n.Level >= 2 {
			return &Node{Kind: KindStrong}, true
		}

		return &Node{Kind: KindEmphasis}, true
	case *extast.Strikethrough:
		return &Node{Kind: KindStrikethrough}, true
	case *ast.CodeSpan:
		var code strings.Builder

		writeCode(&code, n, src)

		return &Node{Kind: KindCode, Text: code.String()}, true
	case *ast.Link:
		return &Node{Kind: KindLink, URL: string(n.Destination), Title: string(n.Title)}, true
	case *ast.AutoLink:
		return autoLinkNode(n, src), true
	case *ast.Image:
		return &Node{Kind: KindImage, URL: string(n.Destination), Title: string(n.Title)}, true
	case *extast.TaskCheckBox:
		return &Node{Kind: KindCheckbox, Checked: n.IsChecked}, true
	case *ast.RawHTML:
		return &Node{Kind: KindHTML, Text: rawHTMLText(n, src)}, true
	default:
		return nil, false
	}
}

/*
lineBreak returns the break that ends a run of text at the end of a
line, or nil.
*/
func lineBreak(n ast.Node) *Node {
	t, ok := n.(*ast.Text)

	switch {
	case !ok:
		return nil
	case t.HardLineBreak():
		return &Node{Kind: KindLineBreak}
	case t.SoftLineBreak():
		return &Node{Kind: KindSoftBreak}
	default:
		return nil
	}
}

/*
autoLinkNode converts a bare URL or email address to a link holding its
text.
*/
func autoLinkNode(n *ast.AutoLink, src []byte) *Node {
	url := string(n.URL(src))
	if n.AutoLinkType == ast.AutoLinkEmail {
		url = "mailto:" + url
	}

	return &Node{
		Kind:     KindLink,
		URL:      url,
		Children: []*Node{{Kind: KindText, Text: string(n.Label(src))}},
	}
}

/*
appendNodes appends nodes to a list of siblings, merging a run of text
into the text before it and dropping empty text.
*/
func appendNodes(siblings []*Node, nodes ...*Node) []*Node {
	for _, node := range nodes {
		last := len(siblings) - 1

		switch {
		case node.Kind != KindText:
			siblings = append(siblings, node)
		case node.Text == "":
		case last >= 0 && siblings[last].Kind == KindText:
			siblings[last].Text += node.Text
		default:
			siblings = append(siblings, node)
		}
	}

	return siblings
}

/*
hasChildren reports whether nodes of a kind hold other nodes.
*/
func hasChildren(kind NodeKind) bool {
	switch kind {
	case KindText, KindCode, KindCodeBlock, KindHTML, KindThematicBreak, KindSoftBreak, KindLineBreak, KindCheckbox:
		return false
	case KindDocument, KindParagraph, KindHeading, KindBlockquote, KindAlert, KindList, KindListItem,
		KindTable, KindTableRow, KindTableCell, KindEmphasis, KindStrong, KindStrikethrough, KindLink, KindImage:
		return true
	}

	return false
}

/*
rawHTMLText returns inline HTML exactly as written.
*/
func rawHTMLText(n *ast.RawHTML, src []byte) string {
	return string(n.Segments.Value(src))
}

/*
htmlBlockText returns a block of HTML exactly as written.
*/
func htmlBlockText(n *ast.HTMLBlock, src []byte) string {
	text := linesText(n, src)
	if n.HasClosure() {
		text += string(n.ClosureLine.Value(src))
	}

	return strings.TrimRight(text, "\r\n")
}
