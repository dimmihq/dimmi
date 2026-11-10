/*
|--------------------------------------------------------------------------
| Broadcast Controller
|--------------------------------------------------------------------------
|
| GET /broadcasts lists broadcasts; POST starts a draft; GET one reads
| it; PATCH saves the draft a field at a time; DELETE removes a draft.
| The actions on one broadcast send, pause, resume, stop or cancel it,
| copy it, test it, estimate its reach and read its report and
| recipients.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastController serves broadcasts.
*/
type BroadcastController struct {
	Broadcasts *broadcasts.Broadcasts
	IDs        *ids.Registry
}

/*
Index lists broadcasts, newest first.
*/
func (c *BroadcastController) Index(
	ctx context.Context, in *requests.ListBroadcastsRequest) (*BroadcastsOutput, error,
) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := c.Broadcasts.List(ctx, broadcasts.ListInput{
		Membership: membership(ctx), Status: textOrNil(in.Status), Channel: textOrNil(in.Channel),
		Search: textOrNil(in.Search), Limit: limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]resources.Broadcast, len(rows))
	for i, r := range rows {
		items[i] = resources.NewBroadcast(r)
	}

	page := pagination.Page[resources.Broadcast]{Items: items, Limit: limit}

	return &BroadcastsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store starts a draft.
*/
func (c *BroadcastController) Store(ctx context.Context, in *requests.StoreBroadcastRequest) (*BroadcastOutput, error) {
	return broadcastOutput(c.Broadcasts.Create(ctx, membership(ctx), in.Body.Name, in.Body.Channel))
}

/*
Show reads one broadcast.
*/
func (c *BroadcastController) Show(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	return broadcastOutput(c.Broadcasts.Show(ctx, membership(ctx), id))
}

/*
Update saves the fields the request names over the draft.
*/
func (c *BroadcastController) Update(
	ctx context.Context, in *requests.UpdateBroadcastRequest) (*BroadcastOutput, error,
) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)

	row, err := c.Broadcasts.Show(ctx, m, id)
	if err != nil {
		return nil, err
	}

	draft, err := c.merge(broadcasts.DraftOf(row), in.Body)
	if err != nil {
		return nil, err
	}

	return broadcastOutput(c.Broadcasts.Save(ctx, broadcasts.SaveInput{Membership: m, ID: id, Draft: draft}))
}

/*
Destroy removes a draft.
*/
func (c *BroadcastController) Destroy(ctx context.Context, in *requests.BroadcastRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Broadcasts.Delete(ctx, membership(ctx), id)
}

/*
Send starts the broadcast or schedules it.
*/
func (c *BroadcastController) Send(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Send)
}

/*
Pause holds a sending broadcast.
*/
func (c *BroadcastController) Pause(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Pause)
}

/*
Resume sends a paused broadcast on.
*/
func (c *BroadcastController) Resume(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Resume)
}

/*
Stop ends a broadcast for good.
*/
func (c *BroadcastController) Stop(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Stop)
}

/*
Cancel turns a scheduled broadcast back into a draft.
*/
func (c *BroadcastController) Cancel(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Cancel)
}

/*
Duplicate copies a broadcast into a new draft.
*/
func (c *BroadcastController) Duplicate(ctx context.Context, in *requests.BroadcastRequest) (*BroadcastOutput, error) {
	return c.act(ctx, in, c.Broadcasts.Duplicate)
}

/*
Test sends the broadcast once to an address or a Slack channel.
*/
func (c *BroadcastController) Test(ctx context.Context, in *requests.TestBroadcastRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	test := broadcasts.TestInput{Membership: membership(ctx), ID: id, Email: in.Body.Email}
	if in.Body.RoomID != "" {
		room, err := c.IDs.Decode(string(models.PrefixChannelRoom), in.Body.RoomID)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		test.RoomID = &room
	}

	return &EmptyOutput{}, c.Broadcasts.Test(ctx, test)
}

/*
Estimate counts who the broadcast would reach now.
*/
func (c *BroadcastController) Estimate(
	ctx context.Context, in *requests.BroadcastRequest) (*BroadcastEstimateOutput, error,
) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	e, err := c.Broadcasts.Estimate(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &BroadcastEstimateOutput{Body: fhttp.NewResource(resources.BroadcastEstimate{
		Object: "broadcast_estimate", Reach: e.Reach, Skipped: e.Skipped, Capped: e.Capped,
	})}, nil
}

/*
Report reads the broadcast's results.
*/
func (c *BroadcastController) Report(
	ctx context.Context, in *requests.BroadcastRequest) (*BroadcastReportOutput, error,
) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	r, err := c.Broadcasts.Report(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &BroadcastReportOutput{Body: fhttp.NewResource(resources.NewBroadcastReport(r))}, nil
}

/*
Recipients lists a page of the broadcast's recipients.
*/
func (c *BroadcastController) Recipients(ctx context.Context, in *requests.BroadcastRecipientsRequest,
) (*BroadcastRecipientsOutput, error) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	list := broadcasts.RecipientsInput{
		Membership: membership(ctx), ID: id, Status: textOrNil(in.Status), Limit: limit, Offset: in.Offset,
	}
	if list.CompanyID, err = c.optional(models.PrefixCompany, in.CompanyID); err != nil {
		return nil, err
	}

	rows, err := c.Broadcasts.Recipients(ctx, list)
	if err != nil {
		return nil, err
	}

	items := make([]resources.BroadcastRecipient, len(rows))
	for i, r := range rows {
		items[i] = resources.NewBroadcastRecipient(r)
	}

	page := pagination.Page[resources.BroadcastRecipient]{Items: items, Limit: limit}

	return &BroadcastRecipientsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
act runs an action on one broadcast and answers it.
*/
func (c *BroadcastController) act(ctx context.Context, in *requests.BroadcastRequest,
	fn func(context.Context, authz.Membership, ids.UUID) (models.Broadcast, error),
) (*BroadcastOutput, error) {
	id, err := c.decode(in.Broadcast)
	if err != nil {
		return nil, err
	}

	return broadcastOutput(fn(ctx, membership(ctx), id))
}

func broadcastOutput(b models.Broadcast, err error) (*BroadcastOutput, error) {
	if err != nil {
		return nil, err
	}

	return &BroadcastOutput{Body: fhttp.NewResource(resources.NewBroadcast(b))}, nil
}

func (c *BroadcastController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixBroadcast), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}

/*
optional decodes a public id the request may leave empty.
*/
func (c *BroadcastController) optional(prefix models.Prefix, public string) (*ids.UUID, error) {
	if public == "" {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &id, nil
}

/*
textOrNil is nil for empty text.
*/
func textOrNil(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
