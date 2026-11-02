/*
|--------------------------------------------------------------------------
| Names
|--------------------------------------------------------------------------
|
| A generator is given a name such as "tags/CreateTag": optional folders,
| which become Go packages and so are lower case, then the type name. The
| type name is accepted in any case and turned into Go's exported form.
|
*/

package make

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

/*
ErrInvalidName is returned for a name that cannot become a folder and a
Go type.
*/
var ErrInvalidName = errors.New("make: invalid name")

var (
	folderRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	typeRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

/*
name is a parsed name: what was typed, the folders and the type name.
*/
type name struct {
	raw    string
	folder string
	base   string
}

/*
parseName splits the input into folders and a type name.
*/
func parseName(input string) (name, error) {
	raw := strings.Trim(strings.TrimSpace(input), "/")
	folder, base := "", raw

	if i := strings.LastIndex(raw, "/"); i >= 0 {
		folder, base = raw[:i], raw[i+1:]
	}

	for segment := range strings.SplitSeq(folder, "/") {
		if folder != "" && !folderRe.MatchString(segment) {
			return name{}, fmt.Errorf("%w: folder %q must be lower case letters and digits", ErrInvalidName, segment)
		}
	}

	return name{raw: raw, folder: folder, base: strcase.ToCamel(base)}, nil
}

/*
without removes a type suffix, so "CreateTagTest" and "CreateTag" both
name the test of CreateTag.
*/
func (n name) without(suffix string) name {
	if trimmed := strings.TrimSuffix(n.base, suffix); trimmed != "" {
		n.base = trimmed
	}

	return n
}

func (n name) snake() string {
	return strcase.ToSnake(n.base)
}

/*
data returns the stub data for the name in the package.
*/
func (n name) data(pkg string) Data {
	return Data{
		Package: pkg,
		Name:    n.base,
		Snake:   n.snake(),
		Title:   title(n.base),
	}
}

/*
title spells the type name as capitalized words: "CreateTag" becomes
"Create Tag".
*/
func title(typeName string) string {
	words := strings.Fields(strcase.ToDelimited(typeName, ' '))
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}

	return strings.Join(words, " ")
}

/*
valid reports whether the type name is an exported Go identifier.
*/
func (n name) valid() bool {
	return typeRe.MatchString(n.base)
}
