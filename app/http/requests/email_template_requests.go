/*
|--------------------------------------------------------------------------
| Email Template Requests
|--------------------------------------------------------------------------
|
| The look of an address's replies as a request names it. A color is a
| six digit hex color, so a branded reply never carries anything else
| into its styles.
|
*/

package requests

/*
ReplyStyle is the look of an address's replies: plain or branded, the
brand color as a hex color, and a footer in the signature's Markdown.
*/
type ReplyStyle struct {
	Template       *string `json:"template,omitempty"        validate:"omitempty,oneof=plain branded"`
	BrandColor     *string `json:"brand_color,omitempty"     validate:"omitempty,len=7,hexcolor"`
	FooterMarkdown *string `json:"footer_markdown,omitempty" validate:"omitempty,max=500"`
}

/*
StoreEmailTemplateRenderingRequest is the input of
POST /email-addresses/{address}/template-renderings.
*/
type StoreEmailTemplateRenderingRequest struct {
	Address string `path:"address"`
	Body    ReplyStyle
}
