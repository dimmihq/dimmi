/*
|--------------------------------------------------------------------------
| Freezing The Audience
|--------------------------------------------------------------------------
|
| When sending starts, everyone the audience matches becomes a
| recipient row, a page at a time. People who cannot or must not be
| reached are recorded too, with the reason, so the report says exactly
| why someone was left out: no address, a suppressed address, left the
| list, got another broadcast too recently, never used the Messenger.
| Freezing can run again after a crash: rows already frozen stay.
|
*/

package broadcasts

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Why a recipient was left out.
*/
const (
	ReasonNoEmail        = "no_email"
	ReasonSuppressed     = "suppressed"
	ReasonOptedOut       = "unsubscribed"
	ReasonTopic          = "not_subscribed"
	ReasonFrequencyCap   = "frequency_cap"
	ReasonDuplicate      = "duplicate_address"
	ReasonNoMessenger    = "no_messenger"
	ReasonBlocked        = "blocked"
	ReasonRoomInactive   = "room_inactive"
	ReasonRoomNoCompany  = "room_no_company"
	ReasonRoomOptedOut   = "room_unsubscribed"
	ReasonBroadcastEnded = "broadcast_stopped"
)

/*
freezePage is how many contacts one freezing transaction reads.
*/
const freezePage = 1000

/*
candidate is one person or Slack channel about to become a recipient.
*/
type candidate struct {
	contact ids.UUID
	company ids.UUID
	room    ids.UUID
	address string
	status  string
	reason  string
	due     time.Time
}

/*
freeze turns the broadcast's audience into recipients.
*/
func (b *Broadcasts) freeze(ctx context.Context, row models.Broadcast) error {
	if row.Channel == ChannelSlack {
		return b.freezeRooms(ctx, row)
	}

	seen := map[string]bool{}
	after := ids.Nil()

	for {
		var page audiences.MatchPage

		err := b.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
			var err error

			page, err = b.Audiences.Match(ctx, q, audiences.MatchInput{
				WorkspaceID: row.WorkspaceID, AudienceID: row.AudienceID, Rules: row.Rules,
				After: after, Size: freezePage,
			})
			if err != nil {
				return err
			}

			people, err := b.reach(ctx, q, row, matched{contacts: page.Contacts, seen: seen})
			if err != nil {
				return err
			}

			return b.add(ctx, q, row, people)
		})
		if err != nil {
			return err
		}

		if page.Done {
			return nil
		}

		after = page.Last
	}
}

/*
matched is one page of contacts the audience matched, with the
addresses already taken by earlier pages.
*/
type matched struct {
	contacts []models.Contact
	seen     map[string]bool
}

/*
reach decides, for each matched contact, whether the channel can reach
them and whether they want to be reached.
*/
func (b *Broadcasts) reach(
	ctx context.Context, q *models.Queries, row models.Broadcast, m matched) ([]candidate, error,
) {
	page, seen := m.contacts, m.seen
	if len(page) == 0 {
		return nil, nil
	}

	f, err := b.factsOf(ctx, q, row, page)
	if err != nil {
		return nil, err
	}

	zone := f.zone
	out := make([]candidate, 0, len(page))

	for _, c := range page {
		person := candidate{contact: c.ID, status: "queued", due: dueFor(row, c, zone, b.Clock.Now())}
		if c.CompanyID != nil {
			person.company = *c.CompanyID
		}

		if row.Channel == ChannelEmail && c.Email != nil {
			person.address = strings.ToLower(strings.TrimSpace(*c.Email))
		}

		if reason := f.leaveOut(row, c, person.address, seen); reason != "" {
			person.status, person.reason, person.address = skippedStatus(reason), reason, ""
		}

		out = append(out, person)
	}

	return out, nil
}

/*
add stores a page of candidates as recipients.
*/
func (b *Broadcasts) add(ctx context.Context, q *models.Queries, row models.Broadcast, people []candidate) error {
	if len(people) == 0 {
		return nil
	}

	p := models.AddBroadcastRecipientsParams{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, Now: b.Clock.Now()}
	for _, c := range people {
		p.ContactIds = append(p.ContactIds, c.contact)
		p.CompanyIds = append(p.CompanyIds, c.company)
		p.RoomIds = append(p.RoomIds, c.room)
		p.Addresses = append(p.Addresses, c.address)
		p.Statuses = append(p.Statuses, c.status)
		p.Reasons = append(p.Reasons, c.reason)
		p.ReplyTokens = append(p.ReplyTokens, inbound.NewReplyToken())
		p.DueAts = append(p.DueAts, c.due)
	}

	_, err := q.AddBroadcastRecipients(ctx, p)

	return wrap("add broadcast recipients", err)
}

/*
skippedStatus is the status a left-out recipient keeps: suppressed for
an address that must not be mailed, skipped for everything else.
*/
func skippedStatus(reason string) string {
	if reason == ReasonSuppressed {
		return "suppressed"
	}

	return "skipped"
}

/*
dueFor is when a recipient's message leaves: now, at the scheduled
time, or at the scheduled wall-clock time in the contact's own zone,
never in the past.
*/
func dueFor(row models.Broadcast, c models.Contact, workspace *time.Location, now time.Time) time.Time {
	if row.ScheduleKind != "local" || row.ScheduledAt == nil {
		return now
	}

	zone := workspace
	if c.Timezone != nil {
		if loc, err := time.LoadLocation(*c.Timezone); err == nil {
			zone = loc
		}
	}

	wall := row.ScheduledAt.In(workspace)
	local := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), 0, 0, zone)

	if local.Before(now) {
		return now
	}

	return local
}
