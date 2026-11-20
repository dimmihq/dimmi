/*
|--------------------------------------------------------------------------
| Broadcast Topic Controller
|--------------------------------------------------------------------------
|
| GET /broadcast-topics lists the lists people subscribe to; POST adds
| one; PATCH renames, describes or archives one. The preference page
| reads and saves one person's choices through a signed link, without
| signing in.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastTopicController serves topics and the preference page.
*/
type BroadcastTopicController struct {
	Broadcasts *broadcasts.Broadcasts
	IDs        *ids.Registry
}

/*
BroadcastTopicsOutput is the workspace's topics.
*/
type BroadcastTopicsOutput struct {
	Body fhttp.Collection[resources.BroadcastTopic]
}

/*
BroadcastTopicOutput is one topic.
*/
type BroadcastTopicOutput struct {
	Body fhttp.Resource[resources.BroadcastTopic]
}

/*
Index lists the topics.
*/
func (c *BroadcastTopicController) Index(ctx context.Context, _ *requests.NoInput) (*BroadcastTopicsOutput, error) {
	rows, err := c.Broadcasts.Topics(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	tallies, err := c.Broadcasts.TopicTallies(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.BroadcastTopic, len(rows))
	for i, t := range rows {
		items[i] = resources.NewBroadcastTopic(t, resources.BroadcastTopicCounts(tallies[t.ID]))
	}

	page := pagination.Page[resources.BroadcastTopic]{Items: items, Limit: 500}

	return &BroadcastTopicsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store adds a topic.
*/
func (c *BroadcastTopicController) Store(ctx context.Context, in *requests.StoreBroadcastTopicRequest,
) (*BroadcastTopicOutput, error) {
	t, err := c.Broadcasts.CreateTopic(ctx, broadcasts.TopicInput{
		Membership: membership(ctx), Name: in.Body.Name, Description: in.Body.Description, Consent: in.Body.Consent,
	})
	if err != nil {
		return nil, err
	}

	return c.show(ctx, t)
}

/*
Update renames, describes or archives a topic.
*/
func (c *BroadcastTopicController) Update(ctx context.Context, in *requests.UpdateBroadcastTopicRequest,
) (*BroadcastTopicOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixBroadcastTopic), in.Topic)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	t, err := c.Broadcasts.UpdateTopic(ctx, broadcasts.TopicInput{
		Membership: membership(ctx), ID: id, Name: in.Body.Name, Description: in.Body.Description,
		Archive: in.Body.Archived,
	})
	if err != nil {
		return nil, err
	}

	return c.show(ctx, t)
}

/*
show answers with one topic and its counts.
*/
func (c *BroadcastTopicController) show(ctx context.Context, t models.BroadcastTopic) (*BroadcastTopicOutput, error) {
	tallies, err := c.Broadcasts.TopicTallies(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	topic := resources.NewBroadcastTopic(t, resources.BroadcastTopicCounts(tallies[t.ID]))

	return &BroadcastTopicOutput{Body: fhttp.NewResource(topic)}, nil
}
