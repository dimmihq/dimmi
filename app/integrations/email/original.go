/*
|--------------------------------------------------------------------------
| Reading The Original
|--------------------------------------------------------------------------
|
| Stored HTML is sanitized, but it may still point at images elsewhere,
| and opening one would tell the sender that the mail was read. So the
| HTML a person reads names such an image in words instead, keeps images
| that travel inside the mail, and opens links in a new tab that knows
| nothing of where it came from.
|
*/

package email

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/dom"
)

/*
ReadableHTML returns stored HTML ready to show in a sandboxed frame. It
is empty when the HTML cannot be read, as nothing else may stand in for
sanitized markup.
*/
func ReadableHTML(stored string) string {
	doc, err := dom.Parse(strings.NewReader(stored))
	if err != nil {
		return ""
	}

	doc.Find("img").Each(func(img dom.Selection) {
		if src, _ := img.Attr("src"); strings.HasPrefix(strings.ToLower(strings.TrimSpace(src)), "data:image/") {
			return
		}

		alt, _ := img.Attr("alt")
		img.ReplaceWithText(imageWords(alt))
	})

	doc.Find("a[href]").SetAttr("target", "_blank").SetAttr("rel", "noopener noreferrer")

	return doc.Find("body").HTML()
}

/*
imageWords says in words what an image that was not loaded shows.
*/
func imageWords(alt string) string {
	if alt = strings.TrimSpace(alt); alt != "" {
		return "[Image: " + alt + "]"
	}

	return "[Image]"
}
