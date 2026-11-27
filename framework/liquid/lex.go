/*
|--------------------------------------------------------------------------
| Expression Tokens
|--------------------------------------------------------------------------
|
| An expression is cut into tokens before it is parsed: names, quoted
| text, numbers, comparison operators and punctuation. Anything else,
| such as a lone = or a stray symbol, is an error.
|
*/

package liquid

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tokIdent tokenKind = iota
	tokString
	tokNumber
	tokOp
	tokPunct
	tokEOF
)

type token struct {
	kind tokenKind
	text string
}

/*
lexExpr splits an expression into its tokens.
*/
func lexExpr(src string) ([]token, error) {
	var out []token
	for i := 0; i < len(src); {
		if isSpace(src[i]) {
			i++
			continue
		}
		t, next, err := lexToken(src, i)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		i = next
	}
	return append(out, token{kind: tokEOF}), nil
}

/*
lexToken reads the token starting at i, returning where the next one
may start.
*/
func lexToken(src string, i int) (token, int, error) {
	c := src[i]
	switch {
	case c == '\'' || c == '"':
		end := strings.IndexByte(src[i+1:], c)
		if end < 0 {
			return token{}, 0, fmt.Errorf("unterminated string in %q", src)
		}
		return token{tokString, src[i+1 : i+1+end]}, i + end + 2, nil
	case startsNumber(src, i):
		j := i + 1
		for j < len(src) && inNumber(src, j) {
			j++
		}
		return token{tokNumber, src[i:j]}, j, nil
	case isIdentStart(c):
		j := i + 1
		for j < len(src) && isIdentPart(src[j]) {
			j++
		}
		if j < len(src) && src[j] == '?' {
			j++
		}
		return token{tokIdent, src[i:j]}, j, nil
	case strings.HasPrefix(src[i:], ".."):
		return token{tokPunct, ".."}, i + 2, nil
	case strings.ContainsRune(".[]()|:,", rune(c)):
		return token{tokPunct, string(c)}, i + 1, nil
	case strings.ContainsRune("=!<>", rune(c)):
		return lexOp(src, i)
	}
	return token{}, 0, fmt.Errorf("unexpected character %q in %q", c, src)
}

/*
lexOp reads a comparison operator: ==, !=, <>, <, >, <= or >=.
*/
func lexOp(src string, i int) (token, int, error) {
	c := src[i]
	j := i + 1
	if j < len(src) && (src[j] == '=' || (c == '<' && src[j] == '>')) {
		j++
	}
	op := src[i:j]
	if op == "=" || op == "!" {
		return token{}, 0, fmt.Errorf("unexpected %q in %q", op, src)
	}
	return token{tokOp, op}, j, nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

/*
startsNumber says whether a number starts at i: a digit, or a minus
sign before one.
*/
func startsNumber(src string, i int) bool {
	return isDigit(src[i]) || (src[i] == '-' && i+1 < len(src) && isDigit(src[i+1]))
}

/*
inNumber says whether the byte at j goes on a number: a digit, or a
dot that does not start a range's two dots.
*/
func inNumber(src string, j int) bool {
	return isDigit(src[j]) || (src[j] == '.' && j+1 < len(src) && src[j+1] != '.')
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '-'
}
