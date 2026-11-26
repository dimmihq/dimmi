/*
|--------------------------------------------------------------------------
| Help Center Words
|--------------------------------------------------------------------------
|
| The help center's own text, its buttons, labels and notices, reads in
| the language of the page, whatever language the visitor's dashboard is
| in. The words live with the product's other messages, in
| lang/<locale>/helpcenter.json, one file per language the help center
| offers; a word a language lacks reads in English.
|
*/

package helpcenter

import (
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
T returns the help center's text for the key in the page's language,
filled with the arguments given as name and value pairs. A page with no
translator reads the key itself.
*/
func (c Center) T(key string, pairs ...any) string {
	if c.Lang == nil {
		return "helpcenter." + key
	}
	args := i18n.Args{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if name, ok := pairs[i].(string); ok {
			args[name] = pairs[i+1]
		}
	}
	return c.Lang.In(pageLang(c), "helpcenter."+key, args)
}

/*
Around returns the text for the key split where its named value goes,
so the page can mark up the value itself, such as an email address
kept left to right.
*/
func (c Center) Around(key, name string, pairs ...any) (string, string) {
	before, after, _ := strings.Cut(c.T(key, append(pairs, name, "\x00")...), "\x00")
	return before, after
}

/*
Date writes a date the way readers of the page's language write it.
*/
func (c Center) Date(t time.Time) string {
	if pageLang(c) == "en" {
		return Day(t)
	}
	return t.Format(c.T("date_format"))
}
