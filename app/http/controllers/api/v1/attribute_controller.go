/*
|--------------------------------------------------------------------------
| Attribute Definition Controller
|--------------------------------------------------------------------------
|
| GET /attribute-definitions lists the custom attributes of contacts and
| companies; POST defines one; PATCH changes its words, list values,
| status or whether browsers may write it; DELETE removes an archived one
| with its values. GET /attribute-problems lists the writes dropped in
| the last 30 days.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/attributes"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AttributeController serves attribute definitions.
*/
type AttributeController struct {
	Definitions *attributes.Definitions
	IDs         *ids.Registry
}

/*
AttributesOutput is the workspace's definitions.
*/
type AttributesOutput struct {
	Body fhttp.Collection[resources.Attribute]
}

/*
AttributeOutput is one definition.
*/
type AttributeOutput struct {
	Body fhttp.Resource[resources.Attribute]
}

/*
AttributeProblemsOutput is the dropped writes.
*/
type AttributeProblemsOutput struct {
	Body fhttp.Collection[resources.AttributeProblem]
}

/*
Index lists the definitions.
*/
func (c *AttributeController) Index(ctx context.Context, in *requests.ListAttributesRequest,
) (*AttributesOutput, error) {
	var object *string
	if in.Object != "" {
		object = &in.Object
	}

	rows, err := c.Definitions.List(ctx, membership(ctx), object)
	if err != nil {
		return nil, err
	}

	items := make([]resources.Attribute, len(rows))
	for i, r := range rows {
		items[i] = resources.NewAttribute(r)
	}

	page := pagination.Page[resources.Attribute]{Items: items, Limit: 1000}

	return &AttributesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store defines an attribute.
*/
func (c *AttributeController) Store(ctx context.Context, in *requests.StoreAttributeRequest) (*AttributeOutput, error) {
	b := in.Body
	d, err := c.Definitions.Create(ctx, attributes.CreateInput{
		Membership: membership(ctx), Object: b.Object, Key: b.Key, Label: b.Label, Description: b.Description,
		DataType: b.DataType, Options: b.Options,
	})

	return attributeOutput(d, err)
}

/*
Update changes a definition.
*/
func (c *AttributeController) Update(ctx context.Context, in *requests.UpdateAttributeRequest,
) (*AttributeOutput, error) {
	id, err := c.decode(in.Attribute)
	if err != nil {
		return nil, err
	}

	b := in.Body
	d, err := c.Definitions.Update(ctx, attributes.UpdateInput{
		Membership: membership(ctx), ID: id, Label: b.Label, Description: b.Description, Options: b.Options,
		Status: b.Status, ClientWritable: b.ClientWritable,
	})

	return attributeOutput(d, err)
}

/*
Destroy removes an archived definition and its values.
*/
func (c *AttributeController) Destroy(ctx context.Context, in *requests.AttributeRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Attribute)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Definitions.Delete(ctx, membership(ctx), id)
}

/*
Problems lists the dropped writes.
*/
func (c *AttributeController) Problems(ctx context.Context, _ *requests.NoInput) (*AttributeProblemsOutput, error) {
	rows, err := c.Definitions.Problems(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.AttributeProblem, len(rows))
	for i, r := range rows {
		items[i] = resources.NewAttributeProblem(r)
	}

	page := pagination.Page[resources.AttributeProblem]{Items: items, Limit: 500}

	return &AttributeProblemsOutput{Body: fhttp.NewCollection(page)}, nil
}

func attributeOutput(d attributes.Definition, err error) (*AttributeOutput, error) {
	if err != nil {
		return nil, err
	}

	return &AttributeOutput{Body: fhttp.NewResource(resources.NewAttribute(d))}, nil
}

func (c *AttributeController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAttribute), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
