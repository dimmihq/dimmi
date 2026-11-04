/*
|--------------------------------------------------------------------------
| Search Query
|--------------------------------------------------------------------------
|
| People search the way they type: a few words, a "quoted phrase", -word
| to leave something out, OR between alternatives, and a word still being
| typed at the end. This reads that into a PostgreSQL tsquery made only
| of quoted lexemes of letters and digits, which the database receives
| as a bound parameter, so nothing a person types can change the query's
| structure.
|
| A conversation number (#1042, or a bare number alone) or a public id
| jumps straight to that row instead. is:open and its kin narrow the
| list instead of searching for the words.
|
*/

package searchquery

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
Limits on what is read: the longest query, the most terms used, and the
shortest last word matched as a prefix.
*/
const (
	MaxLength    = 256
	maxTerms     = 16
	prefixLength = 2
)

/*
Query is a search as the database matches it.

TSQuery is the text predicate, empty when the search has no words.
Number and ID are a jump to one row, when the search was only that.
Statuses, Unread, Unassigned, Mine and Priority are the is: filters.
PriorityLevel and PriorityMin are the level priority: asks for, exactly
or at least, as typed.
Words are the plain words typed, for matching names and addresses.
*/
type Query struct {
	TSQuery    string
	Number     *int64
	ID         string
	Statuses   []string
	Unread     bool
	Unassigned bool
	Mine       bool
	Priority   bool
	Words      []string

	PriorityLevel string
	PriorityMin   string
}

var (
	idPattern     = regexp.MustCompile(`^[a-z]{2,8}_[0-9a-z]{20,40}$`)
	numberPattern = regexp.MustCompile(`^#?(\d{1,15})$`)
)

/*
Parse reads the search. A search longer than MaxLength is cut there;
callers refuse it first when they must.
*/
func Parse(raw string) Query {
	text := strings.TrimSpace(str.Clean(raw))
	if len(text) > MaxLength {
		text = text[:MaxLength]
	}

	if q, ok := jump(text); ok {
		return q
	}

	var q Query

	terms := q.filter(tokenize(text))
	if len(terms) > maxTerms {
		terms = terms[:maxTerms]
	}

	q.TSQuery = compile(terms)

	return q
}

/*
jump reads a search that is only a conversation number or a public id.
*/
func jump(text string) (Query, bool) {
	if m := numberPattern.FindStringSubmatch(text); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err == nil {
			return Query{Number: &n}, true
		}
	}

	if idPattern.MatchString(strings.ToLower(text)) {
		return Query{ID: strings.ToLower(text)}, true
	}

	return Query{}, false
}

/*
token is one piece of a search: words read together as a phrase, left
out, or joined to the previous piece by OR.
*/
type token struct {
	words   []string
	exclude bool
	or      bool
	phrase  bool
}

/*
tokenize splits the search into pieces, quoted phrases kept whole.
*/
func tokenize(text string) []token {
	var (
		tokens []token
		or     bool
	)

	for i, part := range strings.Split(text, `"`) {
		quoted := i%2 == 1
		if quoted {
			tokens = appendToken(tokens, token{words: words(part), phrase: true, or: or})
			or = false

			continue
		}

		for field := range strings.FieldsSeq(part) {
			if field == "OR" {
				or = len(tokens) > 0

				continue
			}

			t := token{or: or}
			if strings.HasPrefix(field, "-") && len(field) > 1 {
				t.exclude, field = true, field[1:]
			}

			t.words = []string{field}
			tokens = append(tokens, t)
			or = false
		}
	}

	return tokens
}

/*
appendToken adds the token unless it holds no word.
*/
func appendToken(tokens []token, t token) []token {
	if len(t.words) == 0 {
		return tokens
	}

	return append(tokens, t)
}

/*
filter takes the is: filters out of the pieces and the plain words into
Words, and returns the pieces left to match as text.
*/
func (q *Query) filter(tokens []token) []token {
	kept := tokens[:0]

	for _, t := range tokens {
		if !t.phrase && !t.exclude && q.operator(t.words[0]) {
			continue
		}

		t.words = splitWords(t.words)
		if len(t.words) == 0 {
			continue
		}

		if !t.exclude {
			q.Words = append(q.Words, t.words...)
		}

		kept = append(kept, t)
	}

	return kept
}

/*
operator applies the word when it is an is: filter, and reports whether
it was one.
*/
func (q *Query) operator(word string) bool {
	if level, ok := strings.CutPrefix(strings.ToLower(word), "priority:"); ok && level != "" {
		if least, ok := strings.CutPrefix(level, ">="); ok {
			q.PriorityMin = least
		} else {
			q.PriorityLevel = level
		}

		return true
	}

	value, ok := strings.CutPrefix(strings.ToLower(word), "is:")
	if !ok {
		return false
	}

	switch value {
	case "open", "closed", "snoozed":
		q.Statuses = append(q.Statuses, value)
	case "unread":
		q.Unread = true
	case "unassigned":
		q.Unassigned = true
	case "mine":
		q.Mine = true
	case "priority":
		q.Priority = true
	default:
		return false
	}

	return true
}

/*
splitWords breaks the pieces into lowercase runs of letters and digits,
the only characters a lexeme is ever made of.
*/
func splitWords(pieces []string) []string {
	var out []string

	for _, piece := range pieces {
		out = append(out, words(piece)...)
	}

	return out
}

/*
words returns the runs of letters and digits in the text, lowercase.
*/
func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

/*
compile writes the pieces as a tsquery: pieces joined by AND, or by OR
where asked, phrases as followed-by chains, left-out pieces negated,
and the last plain word matched as a prefix.
*/
func compile(tokens []token) string {
	var b strings.Builder

	for i, t := range tokens {
		if i > 0 {
			b.WriteString(map[bool]string{true: " | ", false: " & "}[t.or])
		}

		if t.exclude {
			b.WriteString("!")
		}

		quoted := make([]string, len(t.words))
		for j, w := range t.words {
			quoted[j] = "'" + w + "'"
		}

		last := i == len(tokens)-1 && !t.phrase && !t.exclude && len(t.words) == 1
		if last && len([]rune(t.words[0])) >= prefixLength {
			quoted[0] += ":*"
		}

		if len(quoted) > 1 {
			b.WriteString("(" + strings.Join(quoted, " <-> ") + ")")

			continue
		}

		b.WriteString(quoted[0])
	}

	return b.String()
}
