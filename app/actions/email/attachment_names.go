/*
|--------------------------------------------------------------------------
| Attachment Names
|--------------------------------------------------------------------------
|
| Plenty of mail parts arrive without a file name: attached mails, meeting
| invitations, delivery reports, images. A teammate who downloads one
| still needs a file their computer can open, so the name falls back on
| what the part is. Whether a file may be stored at all is judged by its
| name too, cleaned of the tricks that make it read as something else.
|
*/

package email

import (
	"path"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

/*
attachmentName returns the name a part is stored under, numbering a part
that has none by its position.
*/
func attachmentName(part parse.Attachment, position int) string {
	if part.Filename != "" {
		return part.Filename
	}

	if part.ContentType == "message/delivery-status" {
		return "delivery-status.txt"
	}

	return "attachment-" + strconv.Itoa(position+1) + extensionOf(part.ContentType)
}

/*
extensionOf returns the usual file extension of a media type, or "".
*/
func extensionOf(mediaType string) string {
	switch mediaType {
	case "message/rfc822":
		return ".eml"
	case "text/calendar":
		return ".ics"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	}

	return ""
}

/*
executable reports whether a file name has an extension that runs code
when opened.
*/
func executable(filename string) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".exe", ".bat", ".cmd", ".com", ".scr", ".pif", ".vbs", ".vbe", ".js", ".jse", ".msi", ".jar", ".ps1",
		".wsf", ".hta", ".cpl", ".reg", ".lnk":
		return true
	}

	return false
}
