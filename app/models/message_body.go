/*
|--------------------------------------------------------------------------
| Message Bodies
|--------------------------------------------------------------------------
|
| What a message says, read the way it was written. Plain text is already
| the words. Markdown gives up its markup first, so excerpts and quotes
| show no stars or backslashes. What was kept about an email lives under
| the channel's key in a message's metadata; a message from any other
| channel simply reads as having none of it.
|
*/

package models

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
BodyWords returns the words of a message's body in the given format.
*/
func BodyWords(body, format string) string {
	if enums.BodyFormat(format) == enums.BodyFormatPlain {
		return body
	}

	return markdown.PlainText([]byte(body))
}

/*
EmailMetadata is what a message keeps about how much of its email it
holds. Truncated means its body was cut at the size limit, FullTruncated
that its full text was, and HTMLTooLarge that its HTML was not kept.
*/
type EmailMetadata struct {
	Truncated     bool `json:"truncated"`
	FullTruncated bool `json:"full_truncated"`
	HTMLTooLarge  bool `json:"html_too_large"`
}

/*
ReadEmailMetadata reads the email part of a message's metadata. Metadata
this code did not write reads as empty.
*/
func ReadEmailMetadata(raw []byte) EmailMetadata {
	var out struct {
		Email EmailMetadata `json:"email"`
	}

	_ = json.Unmarshal(raw, &out)

	return out.Email
}
