/*
|--------------------------------------------------------------------------
| Parsing
|--------------------------------------------------------------------------
|
| A template is text with {{ output }} and {% tags %} in it. A dash
| inside the braces, as in {{- or -%}, trims the white space on that
| side. The text is split into pieces, then the tags are nested into
| blocks, so every if finds its endif and every error names its line.
| raw and comment keep their insides as text, never parsed.
|
*/

package liquid

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

/*
Error is a mistake in a template, at its name and line.
*/
type Error struct {
	Template string
	Line     int
	Message  string
	Err      error
}

/*
Unwrap returns the error underneath, such as ErrLimit.
*/
func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Template, e.Message)
	}
	if e.Template == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s", e.Template, e.Line, e.Message)
}

type pieceKind int

const (
	pieceText pieceKind = iota
	pieceOutput
	pieceTag
)

type piece struct {
	kind pieceKind
	src  string
	line int
}

/*
split cuts a template into text, output and tags, trimming where a dash
asks for it, and keeping raw and comment bodies as text.
*/
func split(src string) ([]piece, error) {
	s := &splitter{src: src, line: 1}
	for s.src != "" {
		start := nextOpening(s.src)
		if start < 0 {
			s.text(s.src)
			break
		}
		if err := s.markup(start); err != nil {
			return nil, err
		}
	}
	return s.out, nil
}

/*
splitter is what split keeps as it reads: the source left, the pieces
so far, the line it is on and whether the next text loses its leading
white space.
*/
type splitter struct {
	src      string
	out      []piece
	line     int
	trimNext bool
}

/*
text keeps a run of text, trimmed when the markup before it asked.
*/
func (s *splitter) text(text string) {
	if s.trimNext {
		text = strings.TrimLeft(text, " \t\r\n")
	}
	if text != "" {
		s.out = append(s.out, piece{kind: pieceText, src: text, line: s.line})
	}
}

/*
markup reads the text before the output or tag opening at start, then
the output or tag itself.
*/
func (s *splitter) markup(start int) error {
	src := s.src
	textPart := src[:start]
	isTag := src[start+1] == '%'
	body := src[start+2:]
	if strings.HasPrefix(body, "-") {
		body = body[1:]
		textPart = strings.TrimRight(textPart, " \t\r\n")
	}
	s.text(textPart)
	s.line += strings.Count(src[:start], "\n")

	kind, closing := pieceOutput, "}}"
	if isTag {
		kind, closing = pieceTag, "%}"
	}
	end := strings.Index(body, closing)
	if end < 0 {
		return &Error{Line: s.line, Message: "missing " + closing}
	}
	inner := body[:end]
	s.trimNext = strings.HasSuffix(inner, "-")
	if s.trimNext {
		inner = inner[:len(inner)-1]
	}
	consumed := len(src) - len(body) + end + 2
	inner = strings.TrimSpace(inner)
	s.out = append(s.out, piece{kind: kind, src: inner, line: s.line})
	s.line += strings.Count(src[start:consumed], "\n")
	s.src = src[consumed:]
	if !isTag {
		return nil
	}
	return s.verbatim(inner)
}

/*
verbatim keeps the body of a raw tag as text and drops that of a
comment, up to its end tag. Other tags it leaves alone.
*/
func (s *splitter) verbatim(tag string) error {
	name, _, _ := strings.Cut(tag, " ")
	if name != "raw" && name != "comment" {
		return nil
	}
	closeTag := "end" + name
	at, after, ok := findClosing(s.src, closeTag)
	if !ok {
		return &Error{Line: s.line, Message: name + " has no " + closeTag}
	}
	if name == "raw" {
		s.out = append(s.out, piece{kind: pieceText, src: s.src[:at], line: s.line})
	}
	s.line += strings.Count(s.src[:after], "\n")
	s.trimNext = strings.HasSuffix(strings.TrimSpace(s.src[at:after-2]), "-")
	s.src = s.src[after:]
	s.out = append(s.out, piece{kind: pieceTag, src: closeTag, line: s.line})
	return nil
}

func nextOpening(src string) int {
	for i := 0; i+1 < len(src); i++ {
		if src[i] == '{' && (src[i+1] == '{' || src[i+1] == '%') {
			return i
		}
	}
	return -1
}

/*
findClosing finds the {% endname %} tag, returning where it starts and
where it ends.
*/
func findClosing(src, name string) (int, int, bool) {
	from := 0
	for {
		i := strings.Index(src[from:], "{%")
		if i < 0 {
			return 0, 0, false
		}
		i += from
		end := strings.Index(src[i:], "%}")
		if end < 0 {
			return 0, 0, false
		}
		inner := strings.Trim(src[i+2:i+end], "- \t\r\n")
		if inner == name {
			return i, i + end + 2, true
		}
		from = i + 2
	}
}

/*
node is a part of a parsed template.
*/
type node interface {
	render(s *state) error
}

type textNode struct{ text string }

type outputNode struct {
	value expr
	line  int
}

/*
builder nests pieces into blocks.
*/
type builder struct {
	pieces  []piece
	at      int
	filters map[string]Filter
	name    string
}

func (b *builder) fail(line int, format string, args ...any) error {
	return &Error{Template: b.name, Line: line, Message: fmt.Sprintf(format, args...)}
}

/*
ending is the tag that ended a block: its name, its arguments and its
line.
*/
type ending struct {
	tag  string
	args string
	line int
}

/*
block reads nodes until one of the end tags, returning the tag that
ended it.
*/
func (b *builder) block(ends ...string) ([]node, ending, error) {
	var nodes []node
	for b.at < len(b.pieces) {
		p := b.pieces[b.at]
		b.at++
		if p.kind == pieceTag {
			if name, args := tagParts(p.src); slices.Contains(ends, name) {
				return nodes, ending{tag: name, args: args, line: p.line}, nil
			}
		}
		n, err := b.piece(p)
		if err != nil {
			return nil, ending{}, err
		}
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	if len(ends) > 0 {
		return nil, ending{}, b.fail(b.lastLine(), "missing {%% %s %%}", ends[len(ends)-1])
	}
	return nodes, ending{}, nil
}

/*
piece turns one piece into its node, nil for a tag that leaves none,
such as a comment.
*/
func (b *builder) piece(p piece) (node, error) {
	switch p.kind {
	case pieceText:
		return textNode{p.src}, nil
	case pieceOutput:
		value, err := b.outputExpr(p.src, p.line)
		if err != nil {
			return nil, err
		}
		return outputNode{value: value, line: p.line}, nil
	case pieceTag:
		name, args := tagParts(p.src)
		if strings.HasPrefix(name, "#") {
			return nil, nil
		}
		return b.tag(name, args, p.line)
	}
	return nil, nil
}

/*
tagParts cuts a tag into its name and its arguments.
*/
func tagParts(src string) (string, string) {
	name, args, _ := strings.Cut(src, " ")
	return name, strings.TrimSpace(args)
}

func (b *builder) lastLine() int {
	if len(b.pieces) == 0 {
		return 1
	}
	return b.pieces[len(b.pieces)-1].line
}

/*
parse compiles a template's source.
*/
func parse(name, src string, filters map[string]Filter) ([]node, error) {
	pieces, err := split(src)
	if err != nil {
		e := &Error{}
		if errors.As(err, &e) {
			e.Template = name
		}
		return nil, err
	}
	b := &builder{pieces: pieces, filters: filters, name: name}
	nodes, _, err := b.block()
	return nodes, err
}
