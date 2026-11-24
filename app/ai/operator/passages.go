/*
|--------------------------------------------------------------------------
| Passage Edits
|--------------------------------------------------------------------------
|
| An article edit names the passage to replace by quoting it. Models
| quote from memory, so a quote is matched the way a careful reader
| would: word for word first, then against the closest passage when
| only the wording slipped and every number still agrees. A passage
| that appears more than once is replaced everywhere, since a stale
| fact is stale wherever the article repeats it. What was matched is
| told back to the model, and a miss names the closest passage so the
| next try is a copy.
|
*/

package operator

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

/*
The limits of passage matching: a quote shorter than minRepeatedFind
must appear once, a quote may differ from the article in one character
out of looseness, and the search stops being worth its cost past
maxNearestWork character comparisons.
*/
const (
	maxPassageEdits = 20
	minRepeatedFind = 20
	looseness       = 12
	maxNearestWork  = 60_000_000
)

/*
passageEdit replaces one passage of an article.
*/
type passageEdit struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
}

/*
passageEdits are an edit's passages, read whether the model sent a
list, one edit alone, or either encoded as a string.
*/
type passageEdits []passageEdit

/*
UnmarshalJSON reads the edits in any of the shapes models send.
*/
func (p *passageEdits) UnmarshalJSON(raw []byte) error {
	raw = objectArg(raw)
	var list []passageEdit
	if err := json.Unmarshal(raw, &list); err == nil {
		*p = list
		return nil
	}
	var one passageEdit
	if err := json.Unmarshal(raw, &one); err != nil {
		return fmt.Errorf("%w: edits is a list of {find, replace}", errBadArguments)
	}
	*p = passageEdits{one}
	return nil
}

/*
applyPassages writes each edit into the article's working copy and says
what it matched whenever that was not the quote exactly once.
*/
func applyPassages(body string, edits []passageEdit) (string, []string, error) {
	if len(edits) > maxPassageEdits {
		return "", nil, fmt.Errorf("%w: at most %d edits", errBadArguments, maxPassageEdits)
	}
	var notes []string
	done := map[string]bool{}
	for i, e := range edits {
		if strings.TrimSpace(e.Find) == "" {
			return "", nil, fmt.Errorf("%w: edit %d has nothing to find", errBadArguments, i+1)
		}
		if e.Find == e.Replace || done[e.Find] {
			continue
		}
		done[e.Find] = true
		find, note, err := locate(body, e.Find, i+1)
		if err != nil {
			return "", nil, err
		}
		if n := strings.Count(body, find); n > 1 {
			note = fmt.Sprintf("Edit %d replaced its passage in all %d places it appears.", i+1, n)
		}
		if note != "" {
			notes = append(notes, note)
		}
		if e.Replace == "" {
			body = cut(body, find)
			continue
		}
		body = strings.ReplaceAll(body, find, e.Replace)
	}
	return body, notes, nil
}

/*
cut removes a passage from an article. A passage that is a whole line,
such as a table row or a list item, goes with its line break, so the
lines around it stay together; an empty line left in a table would end
the table there.
*/
func cut(body, find string) string {
	out := strings.ReplaceAll("\n"+body+"\n", "\n"+find+"\n", "\n")
	out = strings.ReplaceAll(out, find, "")
	return strings.TrimSuffix(strings.TrimPrefix(out, "\n"), "\n")
}

/*
locate finds the passage of the article a quote means: the quote
itself, or the closest passage when only its wording slipped.
*/
func locate(body, find string, n int) (string, string, error) {
	switch count := strings.Count(body, find); {
	case count == 1 || (count > 1 && len([]rune(find)) >= minRepeatedFind):
		return find, "", nil
	case count > 1:
		return "", "", fmt.Errorf("%w: edit %d's find is short and appears %d times in the article; send the "+
			"whole sentence around the one you mean as find", errBadArguments, n, count)
	}
	passage, distance := nearest([]rune(body), []rune(strings.TrimSpace(find)))
	if passage == "" {
		return "", "", fmt.Errorf("%w: edit %d's find is not in the article. %s", errBadArguments, n, elsewhere)
	}
	want := []rune(strings.TrimSpace(find))
	if len(want) >= minRepeatedFind && distance*looseness <= len(want) && digits(passage) == digits(find) {
		return passage, fmt.Sprintf("Edit %d's find was worded a little differently from the article; it "+
			"replaced this passage: %q", n, passage), nil
	}
	if distance*3 > len(want) {
		return "", "", fmt.Errorf("%w: edit %d's find is not in the article. %s", errBadArguments, n, elsewhere)
	}
	return "", "", fmt.Errorf("%w: edit %d's find is not in the article. The closest passage is %q: send "+
		"exactly that as find if it is the one you mean", errBadArguments, n, passage)
}

/*
elsewhere is what to do when nothing in the article resembles a quote:
most often the quote belongs to another article than the one named.
*/
const elsewhere = "Nothing in this article is close to it, so the passage most likely belongs to another " +
	"article: check the article_id against the article you read it in. Do not send this find for this " +
	"article again."

/*
nearest is the passage of body closest to want and how many characters
apart they are, found by approximate matching; nothing when the search
would cost too much.
*/
func nearest(body, want []rune) (string, int) {
	m := len(want)
	if m == 0 || len(body) == 0 || len(body)*m > maxNearestWork {
		return "", 0
	}
	prev, next := newColumn(m), newColumn(m)
	for j := range prev.cost {
		prev.cost[j] = j
	}
	best, start, end := m+1, 0, 0
	for i, c := range body {
		next.advance(prev, want, c, i+1)
		if next.cost[m] < best {
			best, start, end = next.cost[m], next.from[m], i+1
		}
		prev, next = next, prev
	}
	if start >= end {
		return "", 0
	}
	return string(body[start:end]), best
}

/*
column is one step of the search: for each length of the quote, the
fewest differences against a passage ending here, and where that
passage starts.
*/
type column struct {
	cost, from []int
}

/*
newColumn is a column for a quote of m characters.
*/
func newColumn(m int) column {
	return column{cost: make([]int, m+1), from: make([]int, m+1)}
}

/*
advance fills the column for the article's next character c, which
ends at position end, from the column before it.
*/
func (n column) advance(prev column, want []rune, c rune, end int) {
	n.cost[0], n.from[0] = 0, end
	for j := 1; j <= len(want); j++ {
		d, f := prev.cost[j-1], prev.from[j-1]
		if want[j-1] != c {
			d++
		}
		if prev.cost[j]+1 < d {
			d, f = prev.cost[j]+1, prev.from[j]
		}
		if n.cost[j-1]+1 < d {
			d, f = n.cost[j-1]+1, n.from[j-1]
		}
		n.cost[j], n.from[j] = d, f
	}
}

/*
digits are the digits of a text in order: the numbers a quote and a
passage must agree on to count as the same passage.
*/
func digits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
