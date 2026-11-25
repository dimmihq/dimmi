/*
Package readable lays a reply out so it can be scanned on a phone.
*/
package readable

/*
|--------------------------------------------------------------------------
| Readable Replies
|--------------------------------------------------------------------------
|
| Customers read a reply mid-task and scan it. The agent is asked to
| write short paragraphs, and mostly does; this makes sure of it without
| asking a model again. Each rule here only changes how a reply is laid
| out. None adds, removes or reorders a word, so a reply means exactly
| what the agent wrote.
|
| The first rule breaks up a paragraph that runs past three sentences.
| Sentences are found by the Unicode sentence-boundary rules, which hold
| for every script, and never by a language's words. Those rules take
| the full stop of an abbreviation for the end of a sentence, so a break
| is never made after a closing word of only a few letters.
|
| Anything that is not plain prose is left as it was written: a list, a
| table, a heading, a quote, a code block, and a paragraph the agent
| already laid out over several lines.
|
*/

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
MaxSentences is the most sentences a paragraph keeps.
*/
const MaxSentences = 3

/*
rules are the layout rules, applied in order.
*/
func rules() []func(string) string {
	return []func(string) string{shortParagraphs}
}

/*
Format lays a reply out by every rule.
*/
func Format(reply string) string {
	for _, rule := range rules() {
		reply = rule(reply)
	}

	return reply
}

/*
shortParagraphs breaks each paragraph of plain prose that runs past
MaxSentences into paragraphs of about equal size.
*/
func shortParagraphs(reply string) string {
	blocks := strings.Split(reply, "\n\n")
	fenced := false
	for i, block := range blocks {
		opens := strings.Count(block, "```")%2 == 1
		if !fenced && !opens && prose(block) {
			blocks[i] = split(block)
		}
		if opens {
			fenced = !fenced
		}
	}

	return strings.Join(blocks, "\n\n")
}

/*
prose reports whether a block is one line of plain running text: not
laid out over several lines, and not a list item, a heading, a quote or
a table row.
*/
func prose(block string) bool {
	text := strings.TrimSpace(block)
	if text == "" || strings.Contains(text, "\n") {
		return false
	}
	if strings.ContainsRune("#>|-*+", rune(text[0])) && !strings.HasPrefix(text, "**") {
		return false
	}
	digits := strings.TrimLeft(text, "0123456789")

	numbered := digits != text && (strings.HasPrefix(digits, ". ") || strings.HasPrefix(digits, ") "))

	return !numbered
}

/*
split breaks one paragraph into paragraphs of at most MaxSentences,
sized as evenly as its sentences allow. A break is only made at a safe
place: where the text before it leaves no bold, code, link or bracket
open, and not after what looks like an abbreviation. When the place a
break would fall is not safe, the nearest safe one is used instead.
*/
func split(paragraph string) string {
	sentences := str.Sentences(paragraph)
	if len(sentences) <= MaxSentences {
		return paragraph
	}
	groups := (len(sentences) + MaxSentences - 1) / MaxSentences
	size := (len(sentences) + groups - 1) / groups

	var out []string
	for start := 0; start < len(sentences); {
		end := cut(sentences, start, size)
		out = append(out, strings.TrimSpace(strings.Join(sentences[start:end], "")))
		start = end
	}

	return strings.Join(out, "\n\n")
}

/*
cut says where the paragraph that begins at a sentence ends: after the
last safe place within its size, or else at the first safe place past
it, or at the end of the text when there is none.
*/
func cut(sentences []string, start, size int) int {
	if len(sentences)-start <= size {
		return len(sentences)
	}
	best := -1
	var text strings.Builder
	for i := start; i < len(sentences)-1; i++ {
		text.WriteString(sentences[i])
		held := i - start + 1
		safe := closed(text.String()) && !abbreviated(sentences[i])
		if safe && held <= size {
			best = i + 1
		}
		if held >= size && best >= 0 {
			return best
		}
		if safe && held > size {
			return i + 1
		}
	}

	return len(sentences)
}

/*
shortest is the fewest letters a closing word has before the full stop
after it is trusted to end a sentence.
*/
const shortest = 4

/*
abbreviated reports whether a sentence may not be one: it ends in a
full stop after a word of a few letters, as "Dr.", "vs.", "etc.", an
initial, or their like in any script that shortens words with a full
stop. The Unicode rules take such a full stop for the end of a
sentence whenever a capital follows, so a break is never made there. A
real sentence that ends on a short word only loses a place to break,
which costs nothing.
*/
func abbreviated(sentence string) bool {
	text := strings.TrimRightFunc(sentence, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("\"')]*_”’»", r)
	})
	if !strings.HasSuffix(text, ".") {
		return false
	}
	words := strings.Fields(strings.TrimRight(text, "."))
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	if strings.ContainsFunc(last, unicode.IsDigit) {
		return false
	}
	letters := len(slices.DeleteFunc([]rune(last), func(r rune) bool { return !unicode.IsLetter(r) }))

	return letters > 0 && letters < shortest
}

/*
closed reports whether a piece of text leaves no formatting open: every
bold marker and backtick is paired, and every bracket is shut.
*/
func closed(text string) bool {
	if strings.Count(text, "**")%2 != 0 || strings.Count(text, "`")%2 != 0 {
		return false
	}
	depth := 0
	for _, r := range text {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		}
	}

	return depth == 0
}
