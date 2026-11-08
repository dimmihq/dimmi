/*
|--------------------------------------------------------------------------
| Confirmed Resolution
|--------------------------------------------------------------------------
|
| A conversation counts as resolved only when the customer says so. A
| short reply such as "yes", "that helped" or "thanks!" right after an
| answer confirms it; anything longer, a question, or a reply with a
| "but" or a "not" in it is a new turn of the conversation, never a
| confirmation. Being strict costs a few missed confirmations, which go
| down the quiet path instead; being loose would call unsolved
| problems solved.
|
*/

package resolution

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
maxWords bounds how long a confirmation may be.
*/
const maxWords = 6

/*
Prompt is the line an answer ends with when the customer is asked to
confirm it.
*/
const Prompt = "Did that solve it? Reply yes if it did."

/*
phrases are the confirmations, folded, without punctuation.
*/
func phrases() []string {
	return []string{
		"yes", "y", "yep", "yeah", "yup", "yes please", "yes it did", "it did", "yes thanks", "yes thank you",
		"that helped", "that helps", "it helped", "helped", "solved", "that solved it", "it solved it",
		"resolved", "all good", "all sorted", "sorted", "perfect", "great", "works", "it works", "that works",
		"got it", "ok thanks", "okay thanks", "thanks", "thank you", "thanks a lot", "thank you so much",
		"many thanks", "cheers", "awesome thanks", "great thanks", "perfect thanks",
	}
}

/*
blockers are words that turn a short reply into something else.
*/
func blockers() []string {
	return []string{"no", "not", "but", "still", "doesnt", "didnt", "dont", "isnt", "wasnt", "cant", "however"}
}

/*
Confirms reports whether the customer's reply confirms the answer
before it solved their question.
*/
func Confirms(text string) bool {
	if strings.Contains(text, "?") {
		return false
	}

	words := strings.Fields(clean(text))
	if len(words) == 0 || len(words) > maxWords {
		return false
	}

	for _, w := range words {
		if slices.Contains(blockers(), w) {
			return false
		}
	}

	return slices.Contains(phrases(), strings.Join(words, " "))
}

/*
clean folds the text and drops everything but letters, digits and
spaces, so "Thanks!!" and "thanks" read the same and "didn't" reads as
"didnt".
*/
func clean(text string) string {
	folded := str.FoldAccents(str.Fold(text))

	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		case unicode.IsSpace(r):
			return ' '
		}

		return -1
	}, folded)
}
