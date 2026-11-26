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
	"context"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/lang"
)

/*
words is the translator of the help center's text, read once.
*/
var words = sync.OnceValue(func() *i18n.Translator {
	t, err := i18n.New(i18n.Config{Locale: i18n.Source}, lang.Files())
	if err != nil {
		panic("help center words: " + err.Error())
	}
	return t
})

/*
T returns the help center's text for the key in the page's language,
filled with the arguments given as name and value pairs.
*/
func (c Center) T(key string, pairs ...any) string {
	args := i18n.Args{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if name, ok := pairs[i].(string); ok {
			args[name] = pairs[i+1]
		}
	}
	ctx := i18n.WithLocale(context.Background(), pageLang(c))
	return words().T(ctx, "helpcenter."+key, args)
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

/*
Around returns the text for the key split where its named value goes,
so the page can mark up the value itself, such as an email address
kept left to right.
*/
func (c Center) Around(key, name string, pairs ...any) (string, string) {
	before, after, _ := strings.Cut(c.T(key, append(pairs, name, "\x00")...), "\x00")
	return before, after
}
