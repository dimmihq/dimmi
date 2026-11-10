/*
|--------------------------------------------------------------------------
| Broadcast Drafts
|--------------------------------------------------------------------------
|
| Saving a draft checks only what can be wrong in isolation: a name, a
| channel's own fields and a schedule in the future. Whether it can be
| sent, with a reachable audience and a verified sending address, is
| checked when someone presses send, so a half-written draft always
| saves.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Errors saving a draft returns.
*/
var (
	ErrSchedule = fault.New(fault.Invalid, "broadcast_schedule_invalid",
		"Choose when to send: now, at a time, or at a time in each person's own time zone, in the future.")
	ErrSlackAs = fault.New(fault.Invalid, "broadcast_slack_as_invalid",
		"Send Slack broadcasts as the dimmi app or as yourself.")
	ErrTooLong = fault.New(fault.Invalid, "broadcast_too_long",
		"Keep the subject and preview under 200 characters and the message under 50,000.")
)

/*
SaveInput is a draft to store over the broadcast.
*/
type SaveInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Draft      Draft
}

/*
Save stores the composer's draft while the broadcast has not started.
*/
func (b *Broadcasts) Save(ctx context.Context, in SaveInput) (models.Broadcast, error) {
	m := in.Membership
	if !m.Can("broadcasts:write") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	d := in.Draft
	if err := checkDraft(&d, b.Clock.Now()); err != nil {
		return models.Broadcast{}, err
	}

	var out models.Broadcast

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		if row.Status != StatusDraft {
			return ErrNotDraft
		}

		out, err = q.SaveBroadcastDraft(ctx, saveParams(row, d, b.Clock.Now()))

		return wrap("save broadcast", err)
	})

	return out, err
}

/*
checkDraft tidies a draft and refuses what can never be sent.
*/
func checkDraft(d *Draft, now time.Time) error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || len([]rune(d.Name)) > 120 {
		return ErrName
	}

	if len([]rune(d.Subject)) > 200 || len([]rune(d.Preview)) > 200 || len([]rune(d.Body)) > 50000 {
		return ErrTooLong
	}

	if d.SlackAs == "" {
		d.SlackAs = "bot"
	}

	if d.SlackAs != "bot" && d.SlackAs != "member" {
		return ErrSlackAs
	}

	return checkSchedule(d, now)
}

/*
checkSchedule refuses a schedule kind nobody knows or a time in the
past.
*/
func checkSchedule(d *Draft, now time.Time) error {
	switch d.ScheduleKind {
	case "", "now":
		d.ScheduleKind, d.ScheduledAt = "now", nil

		return nil
	case "at", "local":
		if d.ScheduledAt == nil || !d.ScheduledAt.After(now) {
			return ErrSchedule
		}

		return nil
	}

	return ErrSchedule
}

/*
saveParams are the query parameters that store a draft over a row.
*/
func saveParams(row models.Broadcast, d Draft, now time.Time) models.SaveBroadcastDraftParams {
	rooms := d.RoomIDs
	if rooms == nil {
		rooms = []ids.UUID{}
	}

	return models.SaveBroadcastDraftParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Name: d.Name, AudienceID: d.AudienceID, Rules: nullJSON(d.Rules),
		RoomIds: rooms, TopicID: d.TopicID, Service: d.Service, Subject: d.Subject, Preview: d.Preview, Body: d.Body,
		SenderMemberID: d.SenderMemberID, EmailAddressID: d.EmailAddressID, SlackAs: d.SlackAs,
		ReplyTeamID: d.ReplyTeamID, ReplyMemberID: d.ReplyMemberID, AiReplies: d.AIReplies, TrackClicks: d.TrackClicks,
		SkipFrequencyCap: d.SkipFrequencyCap, ScheduleKind: d.ScheduleKind, ScheduledAt: d.ScheduledAt,
		Goal: nullJSON(d.Goal), Now: now,
	}
}

/*
DraftOf is the draft a stored broadcast holds.
*/
func DraftOf(row models.Broadcast) Draft {
	return Draft{
		Name: row.Name, AudienceID: row.AudienceID, Rules: row.Rules, RoomIDs: row.RoomIds, TopicID: row.TopicID,
		Service: row.Service, Subject: row.Subject, Preview: row.Preview, Body: row.Body,
		SenderMemberID: row.SenderMemberID, EmailAddressID: row.EmailAddressID, SlackAs: row.SlackAs,
		ReplyTeamID: row.ReplyTeamID, ReplyMemberID: row.ReplyMemberID, AIReplies: row.AiReplies,
		TrackClicks: row.TrackClicks, SkipFrequencyCap: row.SkipFrequencyCap, ScheduleKind: row.ScheduleKind,
		ScheduledAt: row.ScheduledAt, Goal: row.Goal,
	}
}

/*
nullJSON keeps an empty or null JSON value as SQL null.
*/
func nullJSON(v json.RawMessage) json.RawMessage {
	if len(v) == 0 || string(v) == "null" {
		return nil
	}

	return v
}
