/*
|--------------------------------------------------------------------------
| Text Diffs
|--------------------------------------------------------------------------
|
| Two views of what changed between two versions of a text. Lines gives
| the unified diff a reviewer scans, grouped into hunks with a few lines
| of context. Words gives the changes inside a passage, aligned to whole
| words so an edit reads as "form" replaced by "from" rather than as
| single letters moved around.
|
| Both only describe the change. Deciding what a hunk means for a review
| belongs to the caller.
|
*/

package diff

import (
	"unicode"

	"github.com/aymanbagabas/go-udiff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

/*
Op says whether a piece of text is in both versions, only the new one,
or only the old one.
*/
type Op int

/*
The three kinds of change.
*/
const (
	Equal Op = iota
	Insert
	Delete
)

/*
contextLines is how many unchanged lines surround each change in a hunk.
*/
const contextLines = 3

/*
Line is one line of a hunk. Text keeps its line ending, so joining the
lines of a hunk reproduces that stretch of the text.
*/
type Line struct {
	Op   Op
	Text string
}

/*
Hunk is a run of changed lines with their context. FromLine and ToLine
are the 1-based numbers of its first line in the old and the new text.
*/
type Hunk struct {
	FromLine int
	ToLine   int
	Lines    []Line
}

/*
Segment is a stretch of text with the same change. Joining the Equal
and Delete segments gives the old text, Equal and Insert the new one.
*/
type Segment struct {
	Op   Op
	Text string
}

/*
Lines returns the hunks of a unified diff from a to b, each with three
lines of context, or none when the texts are equal.

The edits come from the same library that builds the hunks and always
apply to a, so its validation error cannot occur and is discarded.
*/
func Lines(a, b string) []Hunk {
	unified, _ := udiff.ToUnifiedDiff("a", "b", a, udiff.Lines(a, b), contextLines)

	hunks := make([]Hunk, 0, len(unified.Hunks))
	for _, h := range unified.Hunks {
		lines := make([]Line, len(h.Lines))
		for i, l := range h.Lines {
			lines[i] = Line{Op: lineOp(l.Kind), Text: l.Content}
		}

		hunks = append(hunks, Hunk{FromLine: h.FromLine, ToLine: h.ToLine, Lines: lines})
	}

	return hunks
}

/*
Words returns the segments that turn a into b. Words, runs of space and
single punctuation marks are compared as units, and the result is
cleaned up so small coincidental matches inside a rewritten passage do
not split it into fragments.
*/
func Words(a, b string) []Segment {
	dmp := diffmatchpatch.New()
	vocab := newVocabulary()

	left, leftOK := vocab.encode(a)
	right, rightOK := vocab.encode(b)

	var diffs []diffmatchpatch.Diff
	if leftOK && rightOK {
		diffs = vocab.decode(dmp.DiffCleanupSemantic(dmp.DiffMainRunes(left, right, false)))
	} else {
		diffs = dmp.DiffCleanupSemantic(dmp.DiffMain(a, b, false))
	}

	segments := make([]Segment, len(diffs))
	for i, d := range diffs {
		segments[i] = Segment{Op: wordOp(d.Type), Text: d.Text}
	}

	return segments
}

/*
lineOp maps the line library's operation onto Op.
*/
func lineOp(kind udiff.OpKind) Op {
	switch kind {
	case udiff.Delete:
		return Delete
	case udiff.Insert:
		return Insert
	case udiff.Equal:
	}

	return Equal
}

/*
wordOp maps the word library's operation onto Op.
*/
func wordOp(op diffmatchpatch.Operation) Op {
	switch op {
	case diffmatchpatch.DiffDelete:
		return Delete
	case diffmatchpatch.DiffInsert:
		return Insert
	case diffmatchpatch.DiffEqual:
	}

	return Equal
}

/*
tokens splits text into words, runs of white space and single other
characters, keeping every byte so the tokens join back into the text.
*/
func tokens(text string) []string {
	var out []string

	start, prev := 0, tokenClass(0)

	for i, r := range text {
		class := tokenClass(r)
		if i > start && (class != prev || class == classOther) {
			out = append(out, text[start:i])
			start = i
		}

		prev = class
	}

	if start < len(text) {
		out = append(out, text[start:])
	}

	return out
}

/*
Character classes that decide where one token ends and the next begins.
*/
const (
	classOther = iota
	classWord
	classSpace
)

/*
tokenClass says which kind of token a character belongs to.
*/
func tokenClass(r rune) int {
	switch {
	case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
		return classWord
	case unicode.IsSpace(r):
		return classSpace
	default:
		return classOther
	}
}
