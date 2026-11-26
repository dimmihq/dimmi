/*
|--------------------------------------------------------------------------
| Portal Icons
|--------------------------------------------------------------------------
|
| The portal draws the same Remix icons as the dashboard, as inline SVG,
| so a page needs no request and no script to show them. The themes
| package holds them; collections and ideas name their icon by a word,
| and an unknown word draws the book.
|
*/

package helpcenter

import "github.com/dimmihq/dimmi/resources/views/themes"

/*
iconPath returns the 24px path of the named icon.
*/
func iconPath(name string) string {
	return themes.IconPath(name)
}
