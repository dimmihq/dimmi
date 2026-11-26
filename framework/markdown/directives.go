/*
|--------------------------------------------------------------------------
| Directives
|--------------------------------------------------------------------------
|
| The blocks Markdown has no syntax for, written in the generic directive
| syntax: a leaf directive is one line, "::name[label]{key="value"}", and
| a container directive wraps blocks between ":::name[label]" and a
| closing ":::". A container can hold another when the outer fence has
| more colons, as with code fences. Only the names this package knows
| are read as directives; any other line stays the text it is.
|
|   ::embed[Take the tour]{src="https://youtu.be/dQw4w9WgXcQ"}
|   ::button[Start a trial]{href="https://example.com/start" align="center"}
|   :::details[How long do refunds take?]
|   Five working days.
|   :::
|   :::center
|   Centered words.
|   :::
|
*/

package markdown

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

/*
Directive names this package reads.
*/
const (
	DirectiveEmbed   = "embed"
	DirectiveButton  = "button"
	DirectiveDetails = "details"
	DirectiveCenter  = "center"
)

/*
KindDirective is the kind of the syntax tree node a directive parses
into.
*/
var KindDirective = ast.NewNodeKind("Directive") //nolint:gochecknoglobals // a node kind is registered once, at init

/*
directiveNode is a parsed directive. Fence is how many colons opened
it, two for a leaf. Open is the source of its first line and Close of
its closing fence, empty for a leaf or a container never closed.
*/
type directiveNode struct {
	ast.BaseBlock

	name  string
	label string
	attrs map[string]string
	fence int
	open  text.Segment
	close text.Segment
}

/*
Kind returns KindDirective.
*/
func (n *directiveNode) Kind() ast.NodeKind {
	return KindDirective
}

/*
Dump writes the node for debugging.
*/
func (n *directiveNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Name": n.name, "Label": n.label}, nil)
}

/*
attr returns an attribute, empty when it is not set.
*/
func (n *directiveNode) attr(key string) string {
	return n.attrs[key]
}

/*
directive is a directive line split into its parts.
*/
type directive struct {
	fence int
	name  string
	label string
	attrs map[string]string
}

/*
isLeafName and isContainerName tell the directives of each form.
*/
func isLeafName(name string) bool {
	return name == DirectiveEmbed || name == DirectiveButton
}

func isContainerName(name string) bool {
	return name == DirectiveDetails || name == DirectiveCenter
}

/*
parseDirective reads a directive line: colons, a name, an optional
[label] and optional {attributes}, then nothing but spaces. Backslash
escapes in the label are resolved.
*/
func parseDirective(line []byte) (directive, bool) {
	line = bytes.TrimRight(line, " \t\r\n")
	d := directive{fence: countColons(line)}
	if d.fence < 2 {
		return d, false
	}

	i := d.fence + nameLength(line[d.fence:])
	d.name = string(line[d.fence:i])
	if d.name == "" {
		return d, false
	}

	label, i, ok := bracketed(line, i, '[', ']')
	if !ok {
		return d, false
	}
	d.label = string(util.UnescapePunctuations(label))

	attrs, i, ok := bracketed(line, i, '{', '}')
	if !ok {
		return d, false
	}
	if d.attrs, ok = parseAttrs(string(attrs)); !ok {
		return d, false
	}

	return d, i == len(line)
}

/*
countColons counts the colons a line starts with.
*/
func countColons(line []byte) int {
	n := 0
	for n < len(line) && line[n] == ':' {
		n++
	}
	return n
}

/*
nameLength measures a directive name: a lowercase letter, then letters,
digits and dashes.
*/
func nameLength(s []byte) int {
	n := 0
	for n < len(s) && (isLower(s[n]) || (n > 0 && (s[n] == '-' || isDigit(s[n])))) {
		n++
	}
	return n
}

/*
bracketed reads an optional bracketed part starting at i and returns
what it holds and where reading stopped. A part that opens and never
closes fails.
*/
func bracketed(line []byte, i int, open, end byte) ([]byte, int, bool) {
	if i >= len(line) || line[i] != open {
		return nil, i, true
	}
	stop, ok := closing(line, i, open, end)
	if !ok {
		return nil, i, false
	}
	return line[i+1 : stop], stop + 1, true
}

/*
closing returns where the bracket opened at start closes, skipping
escaped characters and counting nested pairs.
*/
func closing(line []byte, start int, open, end byte) (int, bool) {
	depth := 0
	for i := start; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case open:
			depth++
		case end:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

/*
parseAttrs reads key="value", key='value' and key=value pairs, and
bare keys, which are set to an empty value. Keys are lowercase letters
and dashes.
*/
func parseAttrs(s string) (map[string]string, bool) {
	attrs := map[string]string{}
	i := skipSpaces(s, 0)
	for i < len(s) {
		start := i
		for i < len(s) && (isLower(s[i]) || s[i] == '-') {
			i++
		}
		key := s[start:i]
		if key == "" {
			return nil, false
		}
		value := ""
		if i < len(s) && s[i] == '=' {
			var ok bool
			if value, i, ok = attrValue(s, i+1); !ok {
				return nil, false
			}
		}
		attrs[key] = value
		i = skipSpaces(s, i)
	}
	return attrs, true
}

/*
skipSpaces returns where the spaces and tabs from i end.
*/
func skipSpaces(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

/*
attrValue reads one attribute value starting at i, quoted or bare, and
returns it with where reading stopped.
*/
func attrValue(s string, i int) (string, int, bool) {
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		quote := s[i]
		end := strings.IndexByte(s[i+1:], quote)
		if end < 0 {
			return "", 0, false
		}
		return s[i+1 : i+1+end], i + end + 2, true
	}
	start := i
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '"' && s[i] != '\'' {
		i++
	}
	if i == start {
		return "", 0, false
	}
	return s[start:i], i, true
}

func isLower(b byte) bool { return b >= 'a' && b <= 'z' }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

/*
directiveParser reads leaf and container directives.
*/
type directiveParser struct{}

/*
Trigger starts a directive at a colon.
*/
func (directiveParser) Trigger() []byte {
	return []byte{':'}
}

/*
Open starts a directive on a line that is one of the known names in its
form.
*/
func (directiveParser) Open(_ ast.Node, reader text.Reader, _ parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w > 3 {
		return nil, parser.NoChildren
	}
	d, ok := parseDirective(line[pos:])
	if !ok {
		return nil, parser.NoChildren
	}

	node := &directiveNode{name: d.name, label: d.label, attrs: d.attrs, fence: d.fence, open: segment}

	switch {
	case d.fence == 2 && isLeafName(d.name):
		reader.AdvanceToEOL()
		return node, parser.NoChildren
	case d.fence >= 3 && isContainerName(d.name):
		reader.AdvanceToEOL()
		return node, parser.HasChildren
	default:
		return nil, parser.NoChildren
	}
}

/*
Continue keeps a container open until a closing fence of at least as
many colons, which a fenced code block inside it can hold as text.
*/
func (directiveParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	n, ok := node.(*directiveNode)
	if !ok || n.fence == 2 {
		return parser.Close
	}

	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	if insideCode(n, pc) {
		return parser.Continue | parser.HasChildren
	}

	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w <= 3 && isClosingFence(line[pos:], n.fence) {
		n.close = segment
		reader.AdvanceToEOL()
		return parser.Close
	}

	return parser.Continue | parser.HasChildren
}

/*
insideCode reports whether a code block opened inside the container is
still open, so its lines are code and never close the container.
*/
func insideCode(n *directiveNode, pc parser.Context) bool {
	inside := false
	for _, b := range pc.OpenedBlocks() {
		if b.Node == ast.Node(n) {
			inside = true
			continue
		}
		if inside && (b.Node.Kind() == ast.KindFencedCodeBlock || b.Node.Kind() == ast.KindCodeBlock) {
			return true
		}
	}
	return false
}

/*
isClosingFence reports whether a line is only colons, at least as many
as opened the container.
*/
func isClosingFence(line []byte, fence int) bool {
	line = bytes.TrimRight(line, " \t\r\n")
	if len(line) < fence {
		return false
	}
	for _, b := range line {
		if b != ':' {
			return false
		}
	}
	return true
}

/*
Close does nothing; the node is complete.
*/
func (directiveParser) Close(ast.Node, text.Reader, parser.Context) {}

/*
CanInterruptParagraph lets a directive start right below a paragraph
line, as a heading can.
*/
func (directiveParser) CanInterruptParagraph() bool {
	return true
}

/*
CanAcceptIndentedLine is false: an indented directive is code.
*/
func (directiveParser) CanAcceptIndentedLine() bool {
	return false
}

/*
buttonTransformer gives every button its link once the inline content
is parsed, since a block node cannot hold inline nodes while parsing.
*/
type buttonTransformer struct{}

/*
Transform links every button in the document.
*/
func (buttonTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if d, ok := n.(*directiveNode); ok && entering && d.name == DirectiveButton && d.ChildCount() == 0 {
			attachButtonLink(d)
		}
		return ast.WalkContinue, nil
	})
}

/*
attachButtonLink gives a button a link to its target, so the link
passes through the same checks and rewriting as every other link. A
button whose target is not a web, mail or relative address gets none
and shows as its label.
*/
func attachButtonLink(n *directiveNode) {
	href := strings.TrimSpace(n.attr("href"))
	if !safeHref(href) {
		return
	}
	link := ast.NewLink()
	link.Destination = []byte(href)
	label := n.label
	if strings.TrimSpace(label) == "" {
		label = href
	}
	link.AppendChild(link, ast.NewString([]byte(label)))
	link.SetAttributeString("class", []byte("button"))
	n.AppendChild(n, link)
}

/*
SafeLink reports whether a button may link to an address: http, https
and mailto, or a path or fragment on the same site.
*/
func SafeLink(href string) bool {
	return safeHref(href)
}

/*
safeHref is SafeLink.
*/
func safeHref(href string) bool {
	if href == "" {
		return false
	}
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	case "":
		return u.Host == "" && (strings.HasPrefix(href, "/") || strings.HasPrefix(href, "#")) &&
			!strings.HasPrefix(href, "//")
	default:
		return false
	}
}

/*
buttonAlign is the alignment a button asks for, start by default.
*/
func buttonAlign(n *directiveNode) string {
	if a := n.attr("align"); a == "center" || a == "end" {
		return a
	}
	return "start"
}
