/*
|--------------------------------------------------------------------------
| Audience Controller
|--------------------------------------------------------------------------
|
| GET /audiences lists the saved audiences; POST adds one; GET one shows
| what uses it; PATCH saves; DELETE removes one nothing live uses.
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
AudienceController serves saved audiences, previews and tests.
*/
type AudienceController struct {
	Audiences *audiences.Audiences
	IDs       *ids.Registry
}

/*
AudiencesOutput is the workspace's saved audiences.
*/
type AudiencesOutput struct {
	Body fhttp.Collection[resources.Audience]
}

/*
AudienceOutput is one saved audience.
*/
type AudienceOutput struct {
	Body fhttp.Resource[resources.Audience]
}

/*
Index lists the saved audiences.
*/
func (c *AudienceController) Index(ctx context.Context, _ *requests.NoInput) (*AudiencesOutput, error) {
	rows, err := c.Audiences.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.Audience, len(rows))
	for i, r := range rows {
		items[i] = resources.NewAudience(r, nil)
	}

	page := pagination.Page[resources.Audience]{Items: items, Limit: 500}

	return &AudiencesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one saved audience with what uses it.
*/
func (c *AudienceController) Show(ctx context.Context, in *requests.AudienceRequest) (*AudienceOutput, error) {
	id, err := c.decode(in.Audience)
	if err != nil {
		return nil, err
	}

	a, users, err := c.Audiences.Show(ctx, membership(ctx), id)
	if users == nil {
		users = []audiences.User{}
	}

	return audienceOutput(a, users, err)
}

/*
Store adds a saved audience.
*/
func (c *AudienceController) Store(ctx context.Context, in *requests.StoreAudienceRequest) (*AudienceOutput, error) {
	a, err := c.Audiences.Create(ctx, saveInput(ctx, in.Body))

	return audienceOutput(a, nil, err)
}

/*
Update saves a saved audience.
*/
func (c *AudienceController) Update(ctx context.Context, in *requests.UpdateAudienceRequest,
) (*AudienceOutput, error) {
	id, err := c.decode(in.Audience)
	if err != nil {
		return nil, err
	}

	save := saveInput(ctx, in.Body)
	save.ID = id
	a, err := c.Audiences.Save(ctx, save)

	return audienceOutput(a, nil, err)
}

/*
Destroy removes a saved audience.
*/
func (c *AudienceController) Destroy(ctx context.Context, in *requests.AudienceRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Audience)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Audiences.Delete(ctx, membership(ctx), id)
}

func saveInput(ctx context.Context, b requests.AudienceBody) audiences.SaveInput {
	return audiences.SaveInput{
		Membership: membership(ctx), LockVersion: b.LockVersion, Name: b.Name, Description: b.Description,
		Rules: b.Rules,
	}
}

func audienceOutput(a models.Audience, users []audiences.User, err error) (*AudienceOutput, error) {
	if err != nil {
		return nil, err
	}

	return &AudienceOutput{Body: fhttp.NewResource(resources.NewAudience(a, users))}, nil
}

func (c *AudienceController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAudience), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
