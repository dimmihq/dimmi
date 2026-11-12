/*
|--------------------------------------------------------------------------
| Download Names
|--------------------------------------------------------------------------
|
| Files are kept under paths of their own, so a browser told nothing more
| saves one under that path. A link can name the file instead, in the
| Content-Disposition the browser reads, with any name encoded the way
| browsers accept.
|
*/

package filesystem

import "mime"

/*
Disposition kinds: a file shown in place or saved.
*/
const (
	DispositionInline     = "inline"
	DispositionAttachment = "attachment"
)

/*
Disposition returns the Content-Disposition of the kind, inline or
attachment, under the name. A name that cannot be encoded is left out
rather than sent broken.
*/
func Disposition(kind, name string) string {
	if value := mime.FormatMediaType(kind, map[string]string{"filename": name}); value != "" {
		return value
	}

	return kind
}
