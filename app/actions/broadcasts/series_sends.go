/*
|--------------------------------------------------------------------------
| Sending For A Series
|--------------------------------------------------------------------------
|
| A series message step is a broadcast that never freezes an audience:
| each time a traveler reaches the step, the people or Slack channels
| it reaches are added as recipients of that one visit and sent at once,
| through the same senders, consent checks, pacing, tracking and reply
| handling as any broadcast. Whatever later happens to one of those
| recipients, sent, failed, opened, clicked, answered or unsubscribed,
| is passed to the series so the journey can move on.
|
*/

package broadcasts

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SeriesHook hears about recipients that belong to a series journey.
*/
type SeriesHook interface {
	/*
		Touched says something happened to a recipient of a journey: it
		went out, failed, or was opened, clicked, answered or
		unsubscribed from.
	*/
	Touched(ctx context.Context, ws, enrollment ids.UUID) error
}

/*
SeriesSend is one visit of a series message step: the step's broadcast,
the journey and the step record it belongs to, and the people or Slack
channels it reaches.
*/
type SeriesSend struct {
	Broadcast    models.Broadcast
	EnrollmentID ids.UUID
	StepID       ids.UUID
	Contacts     []models.Contact
	Rooms        []models.ChannelRoom
}

/*
SeriesQueued is what a visit queued: how many recipients go out and the
reasons the others were left out.
*/
type SeriesQueued struct {
	Queued  int
	Reasons []string
}

/*
CheckVariables refuses a variable outside the details dimmi fills in.
*/
func CheckVariables(text string) error {
	return knownVariables(text)
}

/*
CheckSeriesMessage returns the first reason a series message step
cannot send: no sending teammate, or a channel that is not ready, such
as email without a connected, verified address.
*/
func (b *Broadcasts) CheckSeriesMessage(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	if err := senderReady(ctx, q, row); err != nil {
		return err
	}

	sender, ok := b.Senders[row.Channel]
	if !ok {
		return ErrChannel
	}

	return sender.Ready(ctx, q, row)
}

/*
SendForSeries records the recipients of one visit inside the series'
transaction and queues their sends, which leave once it commits.
Someone the channel cannot reach, or who does not want the message, is
recorded as skipped with the reason.
*/
func (b *Broadcasts) SendForSeries(ctx context.Context, tx *database.Tx, s SeriesSend) (SeriesQueued, error) {
	q := models.New(tx)
	row := s.Broadcast

	people, err := b.reach(ctx, q, row, matched{contacts: s.Contacts, seen: map[string]bool{}})
	if err != nil {
		return SeriesQueued{}, err
	}

	for _, room := range s.Rooms {
		c := candidate{room: room.ID, status: "queued", reason: roomReason(room, nil, false)}
		if room.CompanyID != nil {
			c.company = *room.CompanyID
		}

		if c.reason != "" {
			c.status = "skipped"
		}

		people = append(people, c)
	}

	rows, err := q.AddSeriesRecipients(ctx, seriesRecipients(row, s, people, b.Clock.Now()))
	if err != nil {
		return SeriesQueued{}, wrap("add series recipients", err)
	}

	var out SeriesQueued

	for _, r := range rows {
		if r.Status != "queued" {
			out.Reasons = append(out.Reasons, deref(r.Reason))
			continue
		}

		out.Queued++
		if err := b.Queue.PushTx(ctx, tx, jobs.SendBroadcastRecipientArgs{
			WorkspaceID: row.WorkspaceID, RecipientID: r.ID,
		}); err != nil {
			return out, wrap("queue series send", err)
		}
	}

	return out, nil
}

/*
seriesRecipients lays the candidates of a visit out as one insert.
*/
func seriesRecipients(
	row models.Broadcast, s SeriesSend, people []candidate, now time.Time,
) models.AddSeriesRecipientsParams {
	p := models.AddSeriesRecipientsParams{
		WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, SeriesEnrollmentID: s.EnrollmentID,
		SeriesStepID: s.StepID, Now: now,
	}

	for _, c := range people {
		p.ContactIds = append(p.ContactIds, c.contact)
		p.CompanyIds = append(p.CompanyIds, c.company)
		p.RoomIds = append(p.RoomIds, c.room)
		p.Addresses = append(p.Addresses, c.address)
		p.Statuses = append(p.Statuses, c.status)
		p.Reasons = append(p.Reasons, c.reason)
		p.ReplyTokens = append(p.ReplyTokens, inbound.NewReplyToken())
	}

	return p
}

/*
ResendDue queues again the series messages held past their time whose
retry was lost, and returns how many.
*/
func (b *Broadcasts) ResendDue(ctx context.Context, tx *database.Tx, ws ids.UUID) (int, error) {
	rows, err := models.New(tx).ListDueSeriesRecipients(ctx, models.ListDueSeriesRecipientsParams{
		WorkspaceID: ws, Before: b.Clock.Now().Add(-2 * time.Minute),
	})
	if err != nil {
		return 0, wrap("list held series sends", err)
	}

	for _, id := range rows {
		if err := b.Queue.PushTx(ctx, tx, jobs.SendBroadcastRecipientArgs{
			WorkspaceID: ws,
			RecipientID: id,
		}); err != nil {
			return 0, wrap("queue held series send", err)
		}
	}

	return len(rows), nil
}

/*
touched tells the series a recipient of one of its journeys changed.
*/
func (b *Broadcasts) touched(ctx context.Context, r models.BroadcastRecipient) error {
	if b.Series == nil || r.SeriesEnrollmentID == nil {
		return nil
	}

	return b.Series.Touched(ctx, r.WorkspaceID, *r.SeriesEnrollmentID)
}

/*
deref reads an optional reason.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return strings.TrimSpace(*s)
}
