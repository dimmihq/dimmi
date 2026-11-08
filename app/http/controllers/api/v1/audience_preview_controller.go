/*
|--------------------------------------------------------------------------
| Audience Preview Controller
|--------------------------------------------------------------------------
|
| POST /audiences/{audience}/previews counts a saved audience and keeps
| the count for the list. POST /audience-previews counts any rule, POST
| /audience-evaluations tests a rule on one person, and GET
| /audience-fields lists what rules may read.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AudiencePreviewController counts and tests audience rules.
*/
type AudiencePreviewController struct {
	Audiences *audiences.Audiences
	IDs       *ids.Registry
}

/*
AudiencePreviewOutput is a preview.
*/
type AudiencePreviewOutput struct {
	Body fhttp.Resource[resources.AudiencePreview]
}

/*
AudienceEvaluationOutput is a test of a rule on one person.
*/
type AudienceEvaluationOutput struct {
	Body fhttp.Resource[resources.AudienceEvaluation]
}

/*
AudienceFieldsOutput is what rules may read.
*/
type AudienceFieldsOutput struct {
	Body fhttp.Collection[resources.AudienceField]
}

/*
PreviewSaved counts a saved audience and keeps the count.
*/
func (c *AudiencePreviewController) PreviewSaved(ctx context.Context, in *requests.AudienceRequest,
) (*AudiencePreviewOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAudience), in.Audience)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	m := membership(ctx)
	a, _, err := c.Audiences.Show(ctx, m, id)
	if err != nil {
		return nil, err
	}

	p, err := c.Audiences.Preview(ctx, audiences.PreviewInput{Membership: m, Rules: a.Rules, AudienceID: &id})

	return previewOutput(p, err)
}

/*
Preview counts any rule.
*/
func (c *AudiencePreviewController) Preview(ctx context.Context, in *requests.PreviewAudienceRequest,
) (*AudiencePreviewOutput, error) {
	p, err := c.Audiences.Preview(ctx, audiences.PreviewInput{
		Membership: membership(ctx), Rules: in.Body.Rules, Sample: in.Body.Sample,
	})

	return previewOutput(p, err)
}

/*
Evaluate tests a rule on one person.
*/
func (c *AudiencePreviewController) Evaluate(ctx context.Context, in *requests.EvaluateAudienceRequest,
) (*AudienceEvaluationOutput, error) {
	contact, err := c.IDs.Decode(string(models.PrefixContact), in.Body.ContactID)
	if err != nil {
		return nil, audiences.ErrContactNotFound
	}

	b := in.Body.Context
	e, err := c.Audiences.Evaluate(ctx, audiences.EvaluateInput{
		Membership: membership(ctx), ContactID: contact, Rules: in.Body.Rules,
		Browser: audiences.Browser{PageURL: b.PageURL, Locale: b.Locale, Device: b.Device, Country: b.Country},
	})
	if err != nil {
		return nil, err
	}

	return &AudienceEvaluationOutput{Body: fhttp.NewResource(resources.NewAudienceEvaluation(e))}, nil
}

/*
Fields lists what rules may read.
*/
func (c *AudiencePreviewController) Fields(ctx context.Context, _ *requests.NoInput) (*AudienceFieldsOutput, error) {
	fields, err := c.Audiences.Fields(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.AudienceField, len(fields))
	for i, f := range fields {
		items[i] = resources.NewAudienceField(f)
	}

	page := pagination.Page[resources.AudienceField]{Items: items, Limit: 1000}

	return &AudienceFieldsOutput{Body: fhttp.NewCollection(page)}, nil
}

func previewOutput(p audiences.Preview, err error) (*AudiencePreviewOutput, error) {
	if err != nil {
		return nil, err
	}

	return &AudiencePreviewOutput{Body: fhttp.NewResource(resources.NewAudiencePreview(p))}, nil
}
