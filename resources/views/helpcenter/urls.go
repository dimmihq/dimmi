/*
|--------------------------------------------------------------------------
| Help Center Addresses
|--------------------------------------------------------------------------
|
| Every link of a page is a path under the help center's base, so the
| same pages work on a subdomain, a custom domain or under the app's /hc
| path.
|
*/

package helpcenter

import (
	"strings"

	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
Href returns the address of a path under the help center's base.
*/
func Href(c Center, path string) string {
	return themes.Href(c.Base, path)
}

/*
Path returns a path under the help center's base, as Href does.
*/
func Path(c Center, path string) string {
	return Href(c, path)
}

/*
BrandStyle sets the brand color on the page's root, nothing for a color
that is not a #RRGGBB hex code.
*/
func BrandStyle(c Center) string {
	if _, ok := luminance(c.Brand); !ok {
		return ""
	}
	return "--brand: " + c.Brand + ";"
}

/*
Asset returns where a file the page loads lives: a full address as it
is, such as the workspace's logo on the app, and a path under the help
center's base otherwise.
*/
func Asset(c Center, path string) string {
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		return path
	}
	return Path(c, path)
}

/*
pageLang is the language the page is in.
*/
func pageLang(c Center) string {
	if c.Locale == "" {
		return "en"
	}
	return c.Locale
}

/*
pageDir is the direction the page's language is written in.
*/
func pageDir(c Center) string {
	lang, _, _ := strings.Cut(pageLang(c), "-")
	switch lang {
	case "ar", "he", "fa", "ur":
		return "rtl"
	}
	return "ltr"
}

/*
HelpLabel names the way to get help: Ask AI where the AI agent answers
first, Contact us where the team does.
*/
func HelpLabel(c Center) string {
	if c.MessengerAI {
		return c.T("ask_ai")
	}
	return c.T("contact_us")
}
