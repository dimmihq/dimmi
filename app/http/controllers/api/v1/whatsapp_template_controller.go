/*
|--------------------------------------------------------------------------
| WhatsApp Template Endpoints
|--------------------------------------------------------------------------
|
| Writing a number's templates from settings: creating one sends it to
| Meta for review, updating one sends it back to review, and deleting one
| removes it from Meta. Each answers with the template as dimmi now keeps
| it; an id of another number's template is simply not found.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WhatsAppTemplateOutput is one template of a number.
*/
type WhatsAppTemplateOutput struct {
	Body fhttp.Resource[resources.WhatsAppTemplate]
}

/*
StoreTemplate sends a new template to Meta for review.
*/
func (c *WhatsAppChannelController) StoreTemplate(ctx context.Context, in *requests.StoreWhatsAppTemplateRequest) (
	*WhatsAppTemplateOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	d := draftOf(in.Body.WhatsAppTemplateBody)
	d.Name, d.Language = strings.TrimSpace(in.Body.Name), in.Body.Language

	t, err := c.WhatsApp.CreateTemplate(ctx, membership(ctx), id, d)
	if err != nil {
		return nil, err
	}

	return &WhatsAppTemplateOutput{Body: fhttp.NewResource(templateResources([]channels.WhatsAppTemplate{t})[0])}, nil
}

/*
UpdateTemplate sends a template's new content back to Meta's review.
*/
func (c *WhatsAppChannelController) UpdateTemplate(ctx context.Context, in *requests.UpdateWhatsAppTemplateRequest) (
	*WhatsAppTemplateOutput, error,
) {
	id, templateID, err := c.template(in.Channel, in.Template)
	if err != nil {
		return nil, err
	}

	ref := channels.WhatsAppTemplateRef{Channel: id, Template: templateID}

	t, err := c.WhatsApp.UpdateTemplate(ctx, membership(ctx), ref, draftOf(in.Body))
	if err != nil {
		return nil, err
	}

	return &WhatsAppTemplateOutput{Body: fhttp.NewResource(templateResources([]channels.WhatsAppTemplate{t})[0])}, nil
}

/*
DestroyTemplate removes a template from Meta.
*/
func (c *WhatsAppChannelController) DestroyTemplate(ctx context.Context, in *requests.WhatsAppTemplateRequest) (
	*struct{}, error,
) {
	id, templateID, err := c.template(in.Channel, in.Template)
	if err != nil {
		return nil, err
	}

	return nil, c.WhatsApp.DeleteTemplate(ctx, membership(ctx),
		channels.WhatsAppTemplateRef{Channel: id, Template: templateID})
}

/*
template decodes a number's and its template's ids.
*/
func (c *WhatsAppChannelController) template(channel, template string) (ids.UUID, ids.UUID, error) {
	id, err := c.channel(channel)
	if err != nil {
		return ids.Nil(), ids.Nil(), err
	}

	templateID, err := c.IDs.Decode(string(models.PrefixWhatsAppTemplate), template)
	if err != nil {
		return ids.Nil(), ids.Nil(), exceptions.ErrNotFound
	}

	return id, templateID, nil
}

/*
draftOf reads a written template.
*/
func draftOf(in requests.WhatsAppTemplateBody) channels.WhatsAppDraft {
	buttons := make([]wadriver.Action, 0, len(in.Buttons))
	for _, b := range in.Buttons {
		buttons = append(buttons, wadriver.Action{
			Type: strings.ToUpper(b.Type), Text: strings.TrimSpace(b.Text), URL: strings.TrimSpace(b.URL),
			PhoneNumber: strings.ReplaceAll(strings.TrimSpace(b.PhoneNumber), " ", ""),
		})
	}

	bindings := make(map[string]channels.WhatsAppBinding, len(in.Bindings))
	for at, b := range in.Bindings {
		bindings[at] = channels.WhatsAppBinding{Key: b.Key, Fallback: b.Fallback}
	}

	return channels.WhatsAppDraft{
		Draft: wadriver.Draft{
			Category: in.Category, Header: strings.TrimSpace(in.Header),
			Body:   wadriver.ToWhatsApp(strings.TrimSpace(in.Body)),
			Footer: strings.TrimSpace(in.Footer), Buttons: buttons, Samples: in.Samples,
		},
		Bindings: bindings,
	}
}

/*
contentOf returns a template's parts for the editor, with the samples
Meta reviewed it with.
*/
func contentOf(t wadriver.Template) resources.WhatsAppContent {
	out := resources.WhatsAppContent{
		Buttons: []resources.WhatsAppButton{}, Samples: map[string]string{},
		Editable: editable(t),
	}

	for _, c := range t.Components {
		switch c.Type {
		case "HEADER":
			out.Header = c.Text
			samplesOf(out.Samples, "header", c.Text, headerSamples(c))
		case "BODY":
			out.Body = wadriver.ToMarkdown(c.Text)
			samplesOf(out.Samples, "body", c.Text, bodySamples(c))
		case "FOOTER":
			out.Footer = c.Text
		case "BUTTONS":
			for _, b := range c.Buttons {
				out.Buttons = append(out.Buttons, resources.WhatsAppButton{
					Type: strings.ToLower(b.Type), Text: b.Text, URL: b.URL, PhoneNumber: b.PhoneNumber,
				})
			}
		}
	}

	return out
}

func headerSamples(c wadriver.Component) []string {
	if c.Example == nil {
		return nil
	}

	return c.Example.HeaderText
}

func bodySamples(c wadriver.Component) []string {
	if c.Example == nil || len(c.Example.BodyText) == 0 {
		return nil
	}

	return c.Example.BodyText[0]
}

/*
samplesOf keys the part's samples by its variables, in order.
*/
func samplesOf(into map[string]string, part, text string, samples []string) {
	for i, key := range wadriver.Placeholders(text) {
		if i < len(samples) {
			into[part+"."+key] = samples[i]
		}
	}
}

/*
editable reports whether the editor holds every part of the template: a
text header without variables or none, and only quick replies, fixed
links and phone numbers.
*/
func editable(t wadriver.Template) bool {
	if strings.EqualFold(t.Category, "authentication") {
		return false
	}

	for _, c := range t.Components {
		if !editablePart(c) {
			return false
		}
	}

	return true
}

/*
editablePart reports whether the editor holds one part of a template.
*/
func editablePart(c wadriver.Component) bool {
	switch c.Type {
	case "HEADER":
		return (c.Format == "" || c.Format == "TEXT") && len(wadriver.Placeholders(c.Text)) == 0
	case "BODY", "FOOTER":
		return true
	case "BUTTONS":
		for _, b := range c.Buttons {
			fixedLink := b.Type == wadriver.ButtonURL && !strings.Contains(b.URL, "{{")
			if b.Type != wadriver.ButtonQuickReply && b.Type != wadriver.ButtonPhone && !fixedLink {
				return false
			}
		}

		return true
	default:
		return false
	}
}

/*
WhatsAppTemplateReplyOutput is the template as the timeline entry it
became and the conversation as it now is, under 201, or 200 for a
replayed request.
*/
type WhatsAppTemplateReplyOutput struct {
	Status   int
	Replayed string `header:"Idempotent-Replayed"`
	Body     struct {
		Data resources.TimelineEntry     `json:"data"`
		Meta resources.WhatsAppReplyMeta `json:"meta"`
	}
}
