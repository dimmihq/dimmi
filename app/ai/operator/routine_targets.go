/*
|--------------------------------------------------------------------------
| One Change Request, One Target
|--------------------------------------------------------------------------
|
| A routine's suggestions are reviewed inside the editor of what they
| change: an article's or a procedure's. Accepting there must change
| only what is on screen, so a routine's changeset touches one article,
| one new article, or one procedure (with its tests), never several.
|
*/

package operator

import (
	"fmt"
)

/*
targetOf names what an operation changes, for content a routine may
suggest changes to, and nothing for other kinds.
*/
func targetOf(op Operation) string {
	switch {
	case op.Kind == KindArticleNew:
		return "new article"
	case op.Article != nil:
		return "article " + op.Article.String()
	case op.Procedure != nil:
		return "procedure " + op.Procedure.String()
	}
	return ""
}

/*
oneTarget refuses a routine's changeset that changes more than one
article or procedure, or drafts more than one article.
*/
func oneTarget(ops []Operation) error {
	seen, news := "", 0
	for _, op := range ops {
		t := targetOf(op)
		if t == "" {
			continue
		}
		if t == "new article" {
			news++
		}
		if (seen != "" && t != seen) || news > 1 {
			return fmt.Errorf("%w: each changeset may change only one article or one procedure (with its "+
				"tests); suggest each article or procedure in a call of its own", errBadArguments)
		}
		seen = t
	}
	return nil
}
