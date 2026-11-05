/*
|--------------------------------------------------------------------------
| Mention Tokens
|--------------------------------------------------------------------------
|
| A note names a teammate as <@mem_...> and a team as <@team_...>, the
| public id inside angle brackets after an at sign. Reading a note gives
| each id once, in the order first written; anything else that looks
| close, such as an email address or an id of another kind, is plain
| text. A note may mention at most MaxTokens ids, so a pasted wall of
| tokens cannot fan out without end.
|
*/

package mentions

import (
	"regexp"
	"strings"
)

/*
MaxTokens is the most ids one note may mention.
*/
const MaxTokens = 50

/*
Kinds of id a mention names.
*/
const (
	KindMember = "member"
	KindTeam   = "team"
)

/*
Token is one mention: the kind of id and the public id.
*/
type Token struct {
	Kind string `json:"type"`
	ID   string `json:"id"`
}

/*
pattern matches a mention token. Public ids are a prefix, an underscore
and base62 letters.
*/
var pattern = regexp.MustCompile(`<@((?:mem|team)_[0-9A-Za-z]{1,40})>`)

/*
Parse returns the mentions in the body, each id once, in the order
first written, at most MaxTokens.
*/
func Parse(body string) []Token {
	seen := map[string]bool{}
	out := []Token{}

	for _, match := range pattern.FindAllStringSubmatch(body, -1) {
		id := match[1]
		if seen[id] {
			continue
		}

		seen[id] = true
		out = append(out, Token{Kind: kindOf(id), ID: id})

		if len(out) == MaxTokens {
			break
		}
	}

	return out
}

/*
kindOf names the kind of id from its prefix.
*/
func kindOf(id string) string {
	if strings.HasPrefix(id, "team_") {
		return KindTeam
	}

	return KindMember
}
