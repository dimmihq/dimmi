/*
|--------------------------------------------------------------------------
| Schedule Controller
|--------------------------------------------------------------------------
|
| GET /schedules lists the workspace's office hours, the default first;
| POST adds a schedule; PATCH changes one or makes it the default;
| DELETE removes one nothing uses. PUT /inboxes/{inbox}/hours gives an
| inbox hours of its own or makes it follow the default. Every schedule
| answers with whether it is open right now.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ScheduleController serves schedules.
*/
type ScheduleController struct {
	Schedules *schedules.Schedules
	IDs       *ids.Registry
	Clock     clock.Clock
}

/*
SchedulesOutput is the workspace's schedules.
*/
type SchedulesOutput struct {
	Body fhttp.Collection[resources.Schedule]
}

/*
ScheduleOutput is one schedule.
*/
type ScheduleOutput struct {
	Body fhttp.Resource[resources.Schedule]
}

/*
Index lists the schedules.
*/
func (c *ScheduleController) Index(ctx context.Context, _ *requests.NoInput) (*SchedulesOutput, error) {
	rows, err := c.Schedules.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Schedule]{Items: resources.NewSchedules(rows, c.Clock.Now()), Limit: 100}

	return &SchedulesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one schedule.
*/
func (c *ScheduleController) Show(ctx context.Context, in *requests.ScheduleRequest) (*ScheduleOutput, error) {
	id, err := c.decode(in.Schedule)
	if err != nil {
		return nil, err
	}

	s, err := c.Schedules.Show(ctx, membership(ctx), id)

	return c.output(s, err)
}

/*
Store adds a schedule.
*/
func (c *ScheduleController) Store(ctx context.Context, in *requests.StoreScheduleRequest) (*ScheduleOutput, error) {
	s, err := c.Schedules.Create(ctx, schedules.Input{
		Membership: membership(ctx), Name: &in.Body.Name, Timezone: &in.Body.Timezone, Weekly: in.Body.Weekly,
		Holidays: in.Body.Holidays, Default: in.Body.IsDefault,
	})

	return c.output(s, err)
}

/*
Update changes a schedule.
*/
func (c *ScheduleController) Update(ctx context.Context, in *requests.UpdateScheduleRequest) (*ScheduleOutput, error) {
	id, err := c.decode(in.Schedule)
	if err != nil {
		return nil, err
	}

	s, err := c.Schedules.Update(ctx, schedules.Input{
		Membership: membership(ctx), ID: id, Name: in.Body.Name, Timezone: in.Body.Timezone,
		Weekly: in.Body.Weekly, Holidays: in.Body.Holidays, Default: in.Body.IsDefault,
	})

	return c.output(s, err)
}

/*
Destroy removes a schedule.
*/
func (c *ScheduleController) Destroy(ctx context.Context, in *requests.ScheduleRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Schedule)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Schedules.Delete(ctx, membership(ctx), id)
}

/*
UpdateInboxHours gives an inbox hours of its own or makes it follow
the workspace's default, and answers the schedule it now uses.
*/
func (c *ScheduleController) UpdateInboxHours(
	ctx context.Context, in *requests.InboxHoursRequest,
) (*ScheduleOutput, error) {
	inbox, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	s, err := c.Schedules.SetInboxHours(ctx, schedules.InboxHoursInput{
		Membership: membership(ctx), InboxID: inbox, Own: in.Body.Own,
	})

	return c.output(s, err)
}

func (c *ScheduleController) output(s models.Schedule, err error) (*ScheduleOutput, error) {
	if err != nil {
		return nil, err
	}

	return &ScheduleOutput{Body: fhttp.NewResource(resources.NewSchedule(s, c.Clock.Now()))}, nil
}

func (c *ScheduleController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixSchedule), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
