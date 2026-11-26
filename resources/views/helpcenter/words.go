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

/*
Ago says how long before now a time was, as help centers say when an
article last changed: today, yesterday, a few days ago, then over so
many weeks, months or years ago, rounded down, in the page's language.
*/
func (c Center) Ago(t, now time.Time) string {
	days := int(now.Sub(t).Hours() / 24)
	switch {
	case days < 1:
		return c.T("ago_today")
	case days == 1:
		return c.T("ago_yesterday")
	case days < 7:
		return c.T("ago_days", "count", days)
	case days < 30:
		return c.T("ago_weeks", "count", days/7)
	case days < 365:
		return c.T("ago_months", "count", days/30)
	default:
		return c.T("ago_years", "count", days/365)
	}
}

/*
ShortDate writes a date without its year where the page's language
does, such as "Oct 28", and in full otherwise.
*/
func (c Center) ShortDate(t time.Time) string {
	if pageLang(c) == "en" {
		return ShortDay(t)
	}
	return c.Date(t)
}

/*
Month is the short name of the month in the page's language, such as
Jan, empty for none.
*/
func (c Center) Month(m time.Month) string {
	if m < time.January || m > time.December {
		return ""
	}
	names := strings.Fields(c.T("months_short"))
	if len(names) != 12 {
		return m.String()[:3]
	}
	return names[m-1]
}

/*
ChangelogTitle is the changelog's name: the team's own, or Changelog in
the page's language when they kept the default.
*/
func (c Center) ChangelogTitle(title string) string {
	if title == "" || title == "Changelog" {
		return c.T("changelog")
	}
	return title
}
