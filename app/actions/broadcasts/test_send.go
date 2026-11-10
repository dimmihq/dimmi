/*
|--------------------------------------------------------------------------
| Test Sends
|--------------------------------------------------------------------------
|
| Before a broadcast reaches anyone, a teammate can send it to their own
| inbox or to one Slack channel, filled in with their own details, to
| see exactly what people will get. A test is marked as one, counts in
| no report and sends from the same address the broadcast will.
|
*/

package broadcasts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Errors a test send returns.
*/
var (
	ErrTestTarget = fault.New(fault.Invalid, "broadcast_test_target_missing",
		"Name the address or Slack channel to send the test to.")
	ErrTestChannel = fault.New(fault.Invalid, "broadcast_test_unsupported",
		"Messenger broadcasts are previewed in the composer; test sends work for email and Slack.")
)

/*
TestInput is where a test goes: an email address, or a Slack channel.
*/
type TestInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Email      string
	RoomID     *ids.UUID
}

/*
Test sends the broadcast once to the address or channel.
*/
func (b *Broadcasts) Test(ctx context.Context, in TestInput) error {
	m := in.Membership
	if !m.Can("broadcasts:write") {
		return exceptions.ErrForbidden
	}

	var d Delivery

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		if err := b.ready(ctx, q, row); err != nil {
			return err
		}

		if d, err = testDelivery(ctx, q, row, in); err != nil {
			return err
		}

		d.Message, err = b.render(ctx, q, d)
		d.Message.Subject = "[Test] " + d.Message.Subject

		return err
	})
	if err != nil {
		return err
	}

	sender, ok := b.Senders[d.Broadcast.Channel]
	if !ok {
		return ErrChannel
	}

	_, err = sender.Deliver(ctx, d)

	return err
}

/*
testDelivery is the one delivery a test makes, filled in with the
teammate's own details.
*/
func testDelivery(ctx context.Context, q *models.Queries, row models.Broadcast, in TestInput) (Delivery, error) {
	r := models.BroadcastRecipient{
		ID: ids.New(), WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, ReplyToken: inbound.NewReplyToken(),
		Status: "queued",
	}

	d := Delivery{Broadcast: row, Recipient: r}

	switch row.Channel {
	case ChannelEmail:
		if in.Email == "" {
			return d, ErrTestTarget
		}

		name := in.Membership.Member.ID.String()
		if sender, err := q.FindBroadcastSender(ctx, models.FindBroadcastSenderParams{
			WorkspaceID: row.WorkspaceID, ID: in.Membership.Member.ID,
		}); err == nil {
			name = sender.Name
		}

		d.Recipient.Address = in.Email
		d.Contact = &models.Contact{Name: &name, Email: &in.Email, CustomAttributes: []byte(`{}`)}
	case ChannelSlack:
		if in.RoomID == nil {
			return d, ErrTestTarget
		}

		room, err := q.FindChannelRoom(ctx, models.FindChannelRoomParams{WorkspaceID: row.WorkspaceID, ID: *in.RoomID})
		if err != nil {
			return d, notFound(err, "find room")
		}

		d.Room, d.Recipient.RoomID, d.Recipient.CompanyID = &room, &room.ID, room.CompanyID
	default:
		return d, ErrTestChannel
	}

	return d, nil
}
