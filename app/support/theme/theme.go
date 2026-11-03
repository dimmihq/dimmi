/*
|--------------------------------------------------------------------------
| Theme
|--------------------------------------------------------------------------
|
| Every page comes in light and dark. The browser's choice lives in a
| cookie the page itself writes when the person flips the theme, so the
| server renders each page in that theme from the first byte and nothing
| flashes on load. A browser that never chose follows its system
| setting, which only the page can read.
|
*/

package theme

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Cookie is the name of the cookie holding the chosen theme. The page
reads and writes it, so it is not HttpOnly; it holds nothing secret.
*/
const Cookie = "dimmi_theme"

/*
The themes a browser can choose.
*/
const (
	Light = "light"
	Dark  = "dark"
)

/*
Of returns the theme the request's browser chose, or "" when it chose
none and follows its system setting.
*/
func Of(r *fhttp.Request) string {
	cookie, err := r.Cookie(Cookie)
	if err != nil || (cookie.Value != Light && cookie.Value != Dark) {
		return ""
	}

	return cookie.Value
}
