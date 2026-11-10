/*
|--------------------------------------------------------------------------
| Notification Destination Controller
|--------------------------------------------------------------------------
|
| The Slack channels the team hears in and each teammate's own Slack
| direct message: listed, made, changed, removed and tested, plus the
| channels the installation's bot can post in. Every id is a public one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationDestinationController serves Slack destinations.
*/
type NotificationDestinationController struct {
	Destinations *notifications.ManageDestinations
	IDs          *ids.Registry
}

/*
NotificationDestinationsOutput is the destination list.
*/
type NotificationDestinationsOutput struct {
	Body fhttp.Collection[resources.NotificationDestination]
}

/*
NotificationDestinationOutput is one destination.
*/
type NotificationDestinationOutput struct {
	Body fhttp.Resource[resources.NotificationDestination]
}

/*
NotificationDestinationTestOutput is a test post's outcome.
*/
type NotificationDestinationTestOutput struct {
	Body fhttp.Resource[resources.NotificationDestinationTest]
}

/*
SlackPostablesOutput is the Slack channels a destination can post in.
*/
type SlackPostablesOutput struct {
	Body fhttp.Collection[resources.SlackPostable]
}

/*
Index lists the workspace's destinations and the caller's own.
*/
func (c *NotificationDestinationController) Index(ctx context.Context, _ *requests.NoInput,
) (*NotificationDestinationsOutput, error) {
	rows, err := c.Destinations.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := resources.NewNotificationDestinations(rows)

	return &NotificationDestinationsOutput{Body: fhttp.NewCollection(pagination.Page[resources.NotificationDestination]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Show answers one destination.
*/
func (c *NotificationDestinationController) Show(ctx context.Context, in *requests.NotificationDestinationRequest,
) (*NotificationDestinationOutput, error) {
	id, err := c.id(in.Destination)
	if err != nil {
		return nil, err
	}

	row, err := c.Destinations.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return destinationOutput(row), nil
}

/*
Store makes a destination.
*/
func (c *NotificationDestinationController) Store(ctx context.Context,
	in *requests.StoreNotificationDestinationRequest,
) (*NotificationDestinationOutput, error) {
	b := in.Body

	installation, err := integrationID(c.IDs, b.IntegrationID)
	if err != nil {
		return nil, err
	}

	h := notifications.Hearing{
		Name: b.Name, Triggers: b.Triggers, IncludeAIHandled: b.IncludeAIHandled, ShowPreviews: true,
		Digests: b.Digests, DigestTime: b.DigestTime, Enabled: true,
	}
	if b.ShowPreviews != nil {
		h.ShowPreviews = *b.ShowPreviews
	}

	if h.DigestTime == "" {
		h.DigestTime = "09:00"
	}

	if b.InboxIDs != nil {
		if h.InboxIDs, err = c.inboxes(*b.InboxIDs); err != nil {
			return nil, err
		}
	}

	row, err := c.Destinations.Create(ctx, membership(ctx), notifications.DestinationInput{
		Type: b.Type, InstallationID: installation, ChannelID: b.ChannelID, Hearing: h,
	})
	if err != nil {
		return nil, err
	}

	return destinationOutput(row), nil
}

/*
Update changes a destination; fields left out stay as they are.
*/
func (c *NotificationDestinationController) Update(ctx context.Context,
	in *requests.UpdateNotificationDestinationRequest,
) (*NotificationDestinationOutput, error) {
	id, err := c.id(in.Destination)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)

	current, err := c.Destinations.Show(ctx, m, id)
	if err != nil {
		return nil, err
	}

	h, err := c.merged(current, in.Body)
	if err != nil {
		return nil, err
	}

	row, err := c.Destinations.Update(ctx, m, id, h)
	if err != nil {
		return nil, err
	}

	return destinationOutput(row), nil
}

/*
merged lays the change over the destination as it is.
*/
func (c *NotificationDestinationController) merged(row models.NotificationDestination,
	b requests.UpdateNotificationDestinationBody,
) (notifications.Hearing, error) {
	h := notifications.Hearing{
		Name: pick(b.Name, row.Name), Triggers: pick(b.Triggers, row.Triggers), InboxIDs: row.InboxIds,
		IncludeAIHandled: pick(b.IncludeAIHandled, row.IncludeAiHandled),
		ShowPreviews:     pick(b.ShowPreviews, row.ShowPreviews), Digests: pick(b.Digests, row.Digests),
		DigestTime: pick(b.DigestTime, row.DigestTime),
		Enabled:    pick(b.Enabled, row.Status == notifications.StatusActive),
	}

	var err error

	switch {
	case b.AllInboxes:
		h.InboxIDs = nil
	case b.InboxIDs != nil:
		h.InboxIDs, err = c.inboxes(*b.InboxIDs)
	}

	return h, err
}

/*
Destroy removes a destination.
*/
func (c *NotificationDestinationController) Destroy(ctx context.Context, in *requests.NotificationDestinationRequest,
) (*EmptyOutput, error) {
	id, err := c.id(in.Destination)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Destinations.Delete(ctx, membership(ctx), id)
}

/*
Test posts a test card to the destination now.
*/
func (c *NotificationDestinationController) Test(ctx context.Context, in *requests.NotificationDestinationRequest,
) (*NotificationDestinationTestOutput, error) {
	id, err := c.id(in.Destination)
	if err != nil {
		return nil, err
	}

	result, err := c.Destinations.Test(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &NotificationDestinationTestOutput{
		Body: fhttp.NewResource(resources.NewNotificationDestinationTest(id, result)),
	}, nil
}

/*
SlackChannels lists the channels the installation's bot can post in.
*/
func (c *NotificationDestinationController) SlackChannels(ctx context.Context,
	in *requests.IntegrationRequest,
) (*SlackPostablesOutput, error) {
	installation, err := integrationID(c.IDs, in.Integration)
	if err != nil {
		return nil, err
	}

	list, err := c.Destinations.Channels(ctx, membership(ctx), installation)
	if err != nil {
		return nil, err
	}

	items := resources.NewSlackPostables(list)

	return &SlackPostablesOutput{Body: fhttp.NewCollection(pagination.Page[resources.SlackPostable]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
id decodes a destination's public id; an unknown one is not found.
*/
func (c *NotificationDestinationController) id(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixDestination), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}

/*
inboxes decodes inbox public ids.
*/
func (c *NotificationDestinationController) inboxes(public []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(public))

	for _, p := range public {
		id, err := c.IDs.Decode(string(models.PrefixInbox), p)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		out = append(out, id)
	}

	return out, nil
}

/*
destinationOutput wraps one destination.
*/
func destinationOutput(row models.NotificationDestination) *NotificationDestinationOutput {
	return &NotificationDestinationOutput{Body: fhttp.NewResource(resources.NewNotificationDestination(row))}
}

/*
pick returns the given value, or the current one when none was given.
*/
func pick[T any](given *T, current T) T {
	if given == nil {
		return current
	}

	return *given
}
