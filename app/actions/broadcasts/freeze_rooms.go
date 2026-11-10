/*
|--------------------------------------------------------------------------
| Freezing Slack Channels
|--------------------------------------------------------------------------
|
| A Slack broadcast goes to customer channels, not to people: each
| shared or community channel is one recipient, tied to its account.
| Channels picked by hand are used as they are; otherwise every customer
| channel is a candidate and, when an audience is set, only those of
| accounts with someone in the audience stay, so "every account on the
| Enterprise plan" reaches exactly their channels.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
freezeRooms turns the broadcast's Slack channels into recipients.
*/
func (b *Broadcasts) freezeRooms(ctx context.Context, row models.Broadcast) error {
	companies, filtered, err := b.audienceCompanies(ctx, row)
	if err != nil {
		return err
	}

	return b.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		rooms, err := q.ListBroadcastRooms(ctx, models.ListBroadcastRoomsParams{
			WorkspaceID: row.WorkspaceID, RoomIds: nonNil(row.RoomIds),
		})
		if err != nil {
			return wrap("list broadcast rooms", err)
		}

		people := make([]candidate, 0, len(rooms))
		for _, room := range rooms {
			c := candidate{
				room: room.ID, status: "queued", due: b.Clock.Now(), reason: roomReason(room, companies, filtered),
			}
			if room.CompanyID != nil {
				c.company = *room.CompanyID
			}

			if c.reason != "" {
				c.status = "skipped"
			}

			people = append(people, c)
		}

		return b.add(ctx, q, row, people)
	})
}

/*
roomReason returns why a channel is left out, or "" when it is posted
to.
*/
func roomReason(room models.ChannelRoom, companies map[ids.UUID]bool, filtered bool) string {
	var settings struct {
		Broadcasts *bool `json:"broadcasts"`
	}

	_ = json.Unmarshal(room.Settings, &settings)

	switch {
	case room.Status != "active":
		return ReasonRoomInactive
	case settings.Broadcasts != nil && !*settings.Broadcasts:
		return ReasonRoomOptedOut
	case filtered && room.CompanyID == nil:
		return ReasonRoomNoCompany
	case filtered && !companies[*room.CompanyID]:
		return "not_in_audience"
	}

	return ""
}

/*
audienceCompanies lists the accounts with someone in the audience.
Filtered is false when the broadcast picked channels by hand or names
no audience, so every channel stays.
*/
func (b *Broadcasts) audienceCompanies(ctx context.Context, row models.Broadcast) (map[ids.UUID]bool, bool, error) {
	out := map[ids.UUID]bool{}
	if len(row.RoomIds) > 0 || (row.AudienceID == nil && len(nullJSON(row.Rules)) == 0) {
		return out, false, nil
	}

	after := ids.Nil()

	for {
		var page audiences.MatchPage

		err := b.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
			var err error

			page, err = b.Audiences.Match(ctx, q, audiences.MatchInput{
				WorkspaceID: row.WorkspaceID, AudienceID: row.AudienceID, Rules: row.Rules,
				After: after, Size: freezePage,
			})

			return err
		})
		if err != nil {
			return nil, true, err
		}

		for _, c := range page.Contacts {
			if c.CompanyID != nil {
				out[*c.CompanyID] = true
			}
		}

		if page.Done {
			return out, true, nil
		}

		after = page.Last
	}
}

/*
nonNil returns the list, or an empty one in place of nil.
*/
func nonNil(list []ids.UUID) []ids.UUID {
	if list == nil {
		return []ids.UUID{}
	}

	return list
}
