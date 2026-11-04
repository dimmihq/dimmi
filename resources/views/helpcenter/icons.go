/*
|--------------------------------------------------------------------------
| Portal Icons
|--------------------------------------------------------------------------
|
| The portal draws the same Remix icons as the dashboard, as inline SVG,
| so a page needs no request and no script to show them. icons.txt holds
| one icon per line, its name and its 24px path. Collections and ideas
| name their icon by one of these words; an unknown word draws the book.
|
*/

package helpcenter

import (
	_ "embed"
	"strings"
)

//go:embed icons.txt
var icons string

/*
iconPath returns the 24px path of the named icon.
*/
func iconPath(name string) string {
	book := ""

	for line := range strings.Lines(icons) {
		key, path, _ := strings.Cut(strings.TrimSpace(line), " ")
		if key == name {
			return path
		}

		if key == "book" {
			book = path
		}
	}

	return book
}
