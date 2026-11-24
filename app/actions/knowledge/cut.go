/*
|--------------------------------------------------------------------------
| Cutting Articles
|--------------------------------------------------------------------------
|
| A search hands back passages, and a passage is only useful when it
| holds a whole thought. So an article is cut by its structure and then
| sized: it is cut at its headings, a part too long to rank or to read
| is cut again at its paragraphs, and neighbors too short to mean
| anything alone are joined, their headings kept in the text. A
| three-line "How to fix it" stays beside the "Why it happens" it
| belongs to, and a handbook of many pages does not become one passage.
| A fenced code block is never cut through its middle unless it alone
| is longer than a passage may be.
|
| The sizes are in characters, counted as runes, so every language is
| cut alike.
|
*/

package knowledge

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Passage sizes. A part shorter than passageMin is joined to a neighbor;
parts are joined up to passageTarget; a part longer than passageMax is
cut.
*/
const (
	passageMin    = 300
	passageTarget = 1200
	passageMax    = 2000
)

/*
part is a run of an article's text under its headings, those below the
title: the "## " heading, then the "### " one when there is one.
*/
type part struct {
	headings []string
	text     string
}

/*
size is the part's length in characters.
*/
func (p part) size() int {
	return utf8.RuneCountInString(p.text)
}

/*
cutArticle cuts an article into passages. Empty sections are dropped.
*/
func cutArticle(a Article) []contracts.KnowledgeChunk {
	var sized []part
	for _, p := range parts(a.Body) {
		sized = append(sized, shorten(p)...)
	}
	var out []contracts.KnowledgeChunk
	for _, group := range joined(sized) {
		headings, text := render(group)
		out = append(out, newChunk(a, headings, text))
	}
	return out
}

/*
parts cuts a body at its "## " and "### " headings; the text before
the first is the intro. The "# " title line is dropped, and a heading
inside a fenced code block is code, not a heading.
*/
func parts(body string) []part {
	var out []part
	var headings []string
	var lines []string
	fenced := false
	flush := func() {
		if text := strings.TrimSpace(strings.Join(lines, "\n")); text != "" {
			out = append(out, part{headings: slices.Clone(headings), text: text})
		}
		lines = nil
	}
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		depth, name := heading(line)
		if fenced || depth == 0 {
			lines = append(lines, line)
			continue
		}
		if depth == 1 {
			continue
		}
		flush()
		headings = append(headings[:min(depth-2, len(headings))], name)
	}
	flush()
	return out
}

/*
heading reads a "# ", "## " or "### " line: its depth and its name.
Any other line has depth zero.
*/
func heading(line string) (int, string) {
	for depth, prefix := range []string{"# ", "## ", "### "} {
		if name, ok := strings.CutPrefix(line, prefix); ok {
			return depth + 1, strings.TrimSpace(name)
		}
	}
	return 0, ""
}

/*
shorten cuts a part longer than a passage may be into parts of about
the target size, at its paragraphs, each under the same headings.
*/
func shorten(p part) []part {
	if p.size() <= passageMax {
		return []part{p}
	}
	var out []part
	var held []string
	length := 0
	flush := func() {
		if len(held) > 0 {
			out = append(out, part{headings: p.headings, text: strings.Join(held, "\n\n")})
		}
		held, length = nil, 0
	}
	for _, block := range blocks(p.text) {
		n := utf8.RuneCountInString(block)
		if length > 0 && length+n > passageTarget {
			flush()
		}
		held = append(held, block)
		length += n
	}
	flush()
	return out
}

/*
blocks are a text's paragraphs, a fenced code block kept whole. A
block longer than a passage may be is cut at its lines, and a line
that long at the size itself.
*/
func blocks(text string) []string {
	var out []string
	var held []string
	fenced := false
	flush := func() {
		if block := strings.TrimSpace(strings.Join(held, "\n")); block != "" {
			out = append(out, pieces(block)...)
		}
		held = nil
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		if !fenced && strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		held = append(held, line)
	}
	flush()
	return out
}

/*
pieces cuts one block that is longer than a passage may be: at its
lines, and a single line that long at the size itself.
*/
func pieces(block string) []string {
	if utf8.RuneCountInString(block) <= passageMax {
		return []string{block}
	}
	var out []string
	var held []rune
	flush := func() {
		if len(held) > 0 {
			out = append(out, string(held))
		}
		held = nil
	}
	for line := range strings.SplitSeq(block, "\n") {
		runes := []rune(line)
		if len(held)+len(runes)+1 > passageMax {
			flush()
		}
		for len(runes) > passageMax {
			out = append(out, string(runes[:passageMax]))
			runes = runes[passageMax:]
		}
		if len(held) > 0 {
			held = append(held, '\n')
		}
		held = append(held, runes...)
	}
	flush()
	return out
}

/*
joined groups neighboring parts into passages: a part too short to
stand alone takes the next one, and a short part joins the passage
before it, as long as the passage stays within the target size.
*/
func joined(sized []part) [][]part {
	var out [][]part
	var group []part
	length := 0
	for _, p := range sized {
		n := p.size()
		short := length < passageMin || n < passageMin
		if len(group) > 0 && (!short || length+n > passageTarget) {
			out = append(out, group)
			group, length = nil, 0
		}
		group = append(group, p)
		length += n
	}
	if len(group) > 0 {
		out = append(out, group)
	}
	return out
}

/*
render makes one passage of the parts joined into it: the headings
they all share lead it, and each part's own headings are written into
the text where the part begins, so a reader still sees where it is.
*/
func render(group []part) ([]string, string) {
	shared := group[0].headings
	for _, p := range group[1:] {
		shared = shared[:commonPrefix(shared, p.headings)]
	}
	var text strings.Builder
	var before []string
	for i, p := range group {
		own := p.headings[len(shared):]
		keep := 0
		if i > 0 {
			keep = commonPrefix(before, own)
		}
		for depth := keep; depth < len(own); depth++ {
			text.WriteString(strings.Repeat("#", len(shared)+depth+2) + " " + own[depth] + "\n\n")
		}
		text.WriteString(p.text + "\n\n")
		before = own
	}
	return shared, strings.TrimSpace(text.String())
}

/*
commonPrefix is how many leading headings two lists share.
*/
func commonPrefix(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
