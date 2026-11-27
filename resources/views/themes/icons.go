/*
|--------------------------------------------------------------------------
| Icons
|--------------------------------------------------------------------------
|
| The help center draws the same Remix icons as the dashboard, as inline
| SVG, so a page needs no request and no script to show them. icons.txt
| holds one icon per line, its name and its 24px path. An unknown name
| draws the book.
|
*/

package themes

import (
	_ "embed"
	"html"
	"strings"
)

//go:embed icons.txt
var icons string

/*
Icon is the markup of the named icon with the class.
*/
func Icon(name, class string) string {
	return `<svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor" aria-hidden="true" class="` +
		html.EscapeString(class) + `"><path d="` + IconPath(name) + `"></path></svg>`
}

/*
IconPath returns the 24px path of the named icon, the book for a name
it does not know.
*/
func IconPath(name string) string {
	book := ""
	for line := range strings.Lines(icons) {
		key, p, _ := strings.Cut(strings.TrimSpace(line), " ")
		if key == name {
			return p
		}
		if key == "book" {
			book = p
		}
	}
	return book
}
