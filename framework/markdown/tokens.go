/*
|--------------------------------------------------------------------------
| Code Tokens
|--------------------------------------------------------------------------
|
| Highlighted code as plain data instead of HTML, for clients that draw
| it themselves and never take markup from the server: each token is its
| text and the short class the highlighter gives its kind, the same class
| the HTML output carries after its prefix, so one stylesheet colors both.
|
*/

package markdown

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

/*
Token is one run of highlighted code: its text, and the class of its
kind without the prefix, such as "k" for a keyword, empty for text that
takes no color.
*/
type Token struct {
	Class string `json:"class,omitempty"`
	Text  string `json:"text"`
}

/*
Tokens splits code of the language into tokens, or returns none when the
language is unknown or the code does not split, so the caller shows it
plain.
*/
func Tokens(language, code string) []Token {
	if language == "" {
		return nil
	}

	lexer := lexers.Get(language)
	if lexer == nil {
		return nil
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return nil
	}

	var out []Token
	for t := iterator(); t != chroma.EOF; t = iterator() {
		class := chroma.StandardTypes[t.Type]
		if n := len(out); n > 0 && out[n-1].Class == class {
			out[n-1].Text += t.Value
			continue
		}
		out = append(out, Token{Class: class, Text: t.Value})
	}

	return out
}
