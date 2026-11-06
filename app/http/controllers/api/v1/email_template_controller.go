/*
|--------------------------------------------------------------------------
| Email Template Controller
|--------------------------------------------------------------------------
|
| Previews of how an address's replies look. The style asked for is
| laid over what the address has saved, so a form shows its choice
| before anything is kept.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailTemplateRenderingOutput is a sample reply in an address's style.
*/
type EmailTemplateRenderingOutput struct {
	Body fhttp.Resource[resources.EmailTemplateRendering]
}

/*
EmailTemplateController previews the style of an address's replies.
*/
type EmailTemplateController struct {
	Render *email.RenderEmailTemplate
	IDs    *ids.Registry
}

/*
StoreRendering previews a sample reply from the address in the style.
*/
func (c *EmailTemplateController) StoreRendering(
	ctx context.Context, in *requests.StoreEmailTemplateRenderingRequest,
) (*EmailTemplateRenderingOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailAddress), in.Address)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	sample, err := c.Render.Handle(ctx, membership(ctx), id, replyStyle(in.Body))
	if err != nil {
		return nil, err
	}

	return &EmailTemplateRenderingOutput{Body: fhttp.NewResource(resources.NewEmailTemplateRendering(sample))}, nil
}

/*
replyStyle turns the requested style into the action's input.
*/
func replyStyle(in requests.ReplyStyle) email.ReplyStyleInput {
	return email.ReplyStyleInput{Template: in.Template, BrandColor: in.BrandColor, FooterMarkdown: in.FooterMarkdown}
}
