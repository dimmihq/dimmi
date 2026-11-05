/*
|--------------------------------------------------------------------------
| Email Template Resource
|--------------------------------------------------------------------------
|
| A sample reply in an address's style, as a preview shows it: the HTML
| a mail client renders and the text twin sent beside it.
|
*/

package resources

import "github.com/dimmihq/dimmi/framework/mail"

/*
EmailTemplateRendering is a sample reply in an address's style.
*/
type EmailTemplateRendering struct {
	Object string `json:"object"`
	HTML   string `json:"html"`
	Text   string `json:"text"`
}

/*
NewEmailTemplateRendering returns the resource of a sample reply.
*/
func NewEmailTemplateRendering(m mail.Message) EmailTemplateRendering {
	return EmailTemplateRendering{Object: "email_template_rendering", HTML: m.HTML, Text: m.Text}
}
