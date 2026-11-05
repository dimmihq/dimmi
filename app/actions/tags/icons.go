/*
|--------------------------------------------------------------------------
| Tag Icons
|--------------------------------------------------------------------------
|
| A tag may carry an icon beside its color, so people tell tags apart at a
| glance. Icons are named from a fixed set the browser app draws, so a
| stored icon always has a picture; a tag without one shows the plain tag
| icon.
|
*/

package tags

import "slices"

/*
Icons are the icons a tag may carry, by name.
*/
func Icons() []string {
	return []string{
		"price-tag-3", "bug", "bank-card", "chat-smile-2", "question", "user", "sparkling", "lightbulb",
		"error-warning", "rocket", "shield-check", "settings-3", "truck", "shopping-cart", "refund-2", "lock",
		"mail", "phone", "global", "code-s-slash", "book-open", "heart", "star", "fire", "flag", "time",
		"calendar", "gift", "tools", "palette",
	}
}

/*
knownIcon reports whether the icon is one a tag may carry; no icon is fine.
*/
func knownIcon(icon *string) bool {
	return icon == nil || slices.Contains(Icons(), *icon)
}
