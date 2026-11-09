/*
|--------------------------------------------------------------------------
| Changelog Labels and Subscribers Controller
|--------------------------------------------------------------------------
|
| A changelog's labels, kept in order, and the people subscribed to it,
| listed a numbered page at a time and removed when asked.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ChangelogExtrasController serves a changelog's labels and, through
People, its subscribers.
*/
type ChangelogExtrasController struct {
	Entries *changelog.Entries
	People  *changelog.Subscribers
	IDs     *ids.Registry
}

/*
ChangelogLabelsOutput is a changelog's labels.
*/
type ChangelogLabelsOutput struct {
	Body fhttp.Collection[resources.ChangelogLabel]
}

/*
ChangelogLabelOutput is one label.
*/
type ChangelogLabelOutput struct {
	Body fhttp.Resource[resources.ChangelogLabel]
}

/*
ChangelogSubscribersOutput is a page of subscribers.
*/
type ChangelogSubscribersOutput struct {
	Body fhttp.Collection[resources.ChangelogSubscriber]
}

/*
Labels lists a changelog's labels.
*/
func (c *ChangelogExtrasController) Labels(ctx context.Context, in *requests.HelpCenterRequest,
) (*ChangelogLabelsOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	rows, err := c.Entries.Labels(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.ChangelogLabel]{Items: resources.NewChangelogLabels(rows), Limit: 100}
	return &ChangelogLabelsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
StoreLabel adds a label.
*/
func (c *ChangelogExtrasController) StoreLabel(ctx context.Context, in *requests.StoreChangelogLabelRequest,
) (*ChangelogLabelOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	l, err := c.Entries.CreateLabel(ctx, changelog.LabelInput{
		Membership: membership(ctx), HelpCenterID: hc, Name: in.Body.Name, Color: in.Body.Color,
	})
	return labelOutput(l, err)
}

/*
UpdateLabel renames, recolors or moves a label.
*/
func (c *ChangelogExtrasController) UpdateLabel(ctx context.Context, in *requests.UpdateChangelogLabelRequest,
) (*ChangelogLabelOutput, error) {
	id, err := c.decode(models.PrefixChangelogLabel, in.Label)
	if err != nil {
		return nil, err
	}
	l, err := c.Entries.UpdateLabel(ctx, changelog.LabelInput{
		Membership: membership(ctx), ID: id, Name: in.Body.Name, Color: in.Body.Color, Position: in.Body.Position,
	})
	return labelOutput(l, err)
}

/*
DestroyLabel deletes a label; its entries stay.
*/
func (c *ChangelogExtrasController) DestroyLabel(ctx context.Context, in *requests.ChangelogLabelRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixChangelogLabel, in.Label)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Entries.DeleteLabel(ctx, membership(ctx), id)
}

/*
Subscribers lists a page of a changelog's subscribers.
*/
func (c *ChangelogExtrasController) Subscribers(ctx context.Context, in *requests.IndexChangelogSubscribersRequest,
) (*ChangelogSubscribersOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	offset, _, err := numberedPage(ctx, changelogPage)
	if err != nil {
		return nil, err
	}
	rows, total, err := c.People.List(ctx, membership(ctx), changelog.SubscriberFilter{
		HelpCenterID: hc, Status: in.Status, Query: in.Query, Limit: changelogPage, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.ChangelogSubscriber]{
		Items: resources.NewChangelogSubscribers(rows), Limit: changelogPage, Total: &total,
	}
	return &ChangelogSubscribersOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
DestroySubscriber forgets a subscriber.
*/
func (c *ChangelogExtrasController) DestroySubscriber(ctx context.Context, in *requests.ChangelogSubscriberRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixChangelogSubscriber, in.Subscriber)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.People.Delete(ctx, membership(ctx), id)
}

func labelOutput(l models.ChangelogLabel, err error) (*ChangelogLabelOutput, error) {
	if err != nil {
		return nil, err
	}
	return &ChangelogLabelOutput{Body: fhttp.NewResource(resources.NewChangelogLabel(l))}, nil
}

func (c *ChangelogExtrasController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
