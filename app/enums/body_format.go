/*
|--------------------------------------------------------------------------
| Body Format
|--------------------------------------------------------------------------
|
| A message's text is either Markdown, which teammates write and HTML
| email converts to, or plain text as a customer typed it into a mail
| client. Plain text is never read as markup: a "*" stays a star and a
| "<div>" stays those five characters.
|
*/

package enums

/*
BodyFormat says how a message's text is to be read.
*/
type BodyFormat string

/*
The body formats.
*/
const (
	BodyFormatMarkdown BodyFormat = "markdown"
	BodyFormatPlain    BodyFormat = "plain"
)
