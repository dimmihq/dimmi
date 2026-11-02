/*
|--------------------------------------------------------------------------
| Migration Files
|--------------------------------------------------------------------------
|
| A migration file is goose SQL: an up section, an optional down section
| and goose annotations, plus a header line that declares its phase. Only
| the up section ever runs on a deploy, so only it is planned and linted.
|
*/

package migrate

import (
	"regexp"
	"strings"
)

/*
The phases a migration header can declare.
*/
const (
	phaseExpand   = "expand"
	phaseContract = "contract"
)

var (
	headerRe        = regexp.MustCompile(`(?im)^--\s*migrate:.*\bphase=(\w+)`)
	downRe          = regexp.MustCompile(`(?im)^--\s*\+goose\s+down\b`)
	noTransactionRe = regexp.MustCompile(`(?im)^--\s*\+goose\s+no\s+transaction\b`)
	lineCommentRe   = regexp.MustCompile(`--[^\n]*`)
	blockCommentRe  = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

/*
sqlFile is a parsed migration. The up field is the up section as
written, and code is the same section without comments, for the lint
rules to match against.
*/
type sqlFile struct {
	path          string
	phase         string
	transactional bool
	up            string
	code          string
}

func parseFile(path, text string) sqlFile {
	up := text
	if loc := downRe.FindStringIndex(text); loc != nil {
		up = text[:loc[0]]
	}

	file := sqlFile{
		path:          path,
		transactional: !noTransactionRe.MatchString(text),
		up:            strings.TrimSpace(up),
		code:          lineCommentRe.ReplaceAllString(blockCommentRe.ReplaceAllString(up, ""), ""),
	}

	if match := headerRe.FindStringSubmatch(text); match != nil {
		switch phase := strings.ToLower(match[1]); phase {
		case phaseExpand, phaseContract:
			file.phase = phase
		}
	}

	return file
}
