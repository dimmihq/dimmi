/*
|--------------------------------------------------------------------------
| Broadcast Resources
|--------------------------------------------------------------------------
|
| How broadcasts, their reports, recipients and topics read over the
| API. Ids are public ids; counts are the latest tally, the same numbers
| open screens receive live while a broadcast sends.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Broadcast is one broadcast with its draft, its status and its counts.
*/
type Broadcast struct {
	ID               string          `json:"id"`
	Object           string          `json:"object"`
	Name             string          `json:"name"`
	Channel          string          `json:"channel"`
	Status           string          `json:"status"`
	PauseReason      *string         `json:"pause_reason"`
	AudienceID       *string         `json:"audience_id"`
	Rules            json.RawMessage `json:"rules"`
	RoomIDs          []string        `json:"room_ids"`
	TopicID          *string         `json:"topic_id"`
	Service          bool            `json:"service"`
	Subject          string          `json:"subject"`
	Preview          string          `json:"preview"`
	Body             string          `json:"body"`
	SenderID         *string         `json:"sender_id"`
	EmailAddressID   *string         `json:"email_address_id"`
	SlackAs          string          `json:"slack_as"`
	ReplyTeamID      *string         `json:"reply_team_id"`
	ReplyMemberID    *string         `json:"reply_member_id"`
	AIReplies        bool            `json:"ai_replies"`
	TrackClicks      bool            `json:"track_clicks"`
	SkipFrequencyCap bool            `json:"skip_frequency_cap"`
	ScheduleKind     string          `json:"schedule_kind"`
	ScheduledAt      *time.Time      `json:"scheduled_at"`
	Goal             json.RawMessage `json:"goal"`
	Counts           json.RawMessage `json:"counts"`
	StartedAt        *time.Time      `json:"started_at"`
	FinishedAt       *time.Time      `json:"finished_at"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

/*
NewBroadcast shapes a broadcast.
*/
func NewBroadcast(b models.Broadcast) Broadcast {
	rooms := make([]string, len(b.RoomIds))
	for i, id := range b.RoomIds {
		rooms[i] = models.PublicID(models.PrefixChannelRoom, id)
	}

	return Broadcast{
		ID: models.PublicID(models.PrefixBroadcast, b.ID), Object: "broadcast", Name: b.Name, Channel: b.Channel,
		Status: b.Status, PauseReason: b.PauseReason, AudienceID: broadcastRef(models.PrefixAudience, b.AudienceID),
		Rules: b.Rules, RoomIDs: rooms, TopicID: broadcastRef(models.PrefixBroadcastTopic, b.TopicID),
		Service: b.Service, Subject: b.Subject, Preview: b.Preview, Body: b.Body,
		SenderID:       broadcastRef(models.PrefixMember, b.SenderMemberID),
		EmailAddressID: broadcastRef(models.PrefixEmailAddress, b.EmailAddressID), SlackAs: b.SlackAs,
		ReplyTeamID:   broadcastRef(models.PrefixTeam, b.ReplyTeamID),
		ReplyMemberID: broadcastRef(models.PrefixMember, b.ReplyMemberID),
		AIReplies:     b.AiReplies, TrackClicks: b.TrackClicks, SkipFrequencyCap: b.SkipFrequencyCap,
		ScheduleKind: b.ScheduleKind, ScheduledAt: b.ScheduledAt, Goal: b.Goal, Counts: b.Counts,
		StartedAt: b.StartedAt, FinishedAt: b.FinishedAt, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

/*
BroadcastRecipient is one person or Slack channel a broadcast went to,
and what happened.
*/
type BroadcastRecipient struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	Status         string     `json:"status"`
	Reason         *string    `json:"reason"`
	ContactID      *string    `json:"contact_id"`
	ContactName    string     `json:"contact_name"`
	ContactEmail   string     `json:"contact_email"`
	CompanyID      *string    `json:"company_id"`
	CompanyName    string     `json:"company_name"`
	RoomID         *string    `json:"room_id"`
	RoomName       string     `json:"room_name"`
	Address        string     `json:"address"`
	ConversationID *string    `json:"conversation_id"`
	SentAt         *time.Time `json:"sent_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
	OpenedAt       *time.Time `json:"opened_at"`
	ClickedAt      *time.Time `json:"clicked_at"`
	RepliedAt      *time.Time `json:"replied_at"`
	Reactions      int32      `json:"reactions"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at"`
	ConvertedAt    *time.Time `json:"converted_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

/*
NewBroadcastRecipient shapes one recipient.
*/
func NewBroadcastRecipient(r models.ListBroadcastRecipientPageRow) BroadcastRecipient {
	return BroadcastRecipient{
		ID: models.PublicID(models.PrefixBroadcastRecipient, r.ID), Object: "broadcast_recipient", Status: r.Status,
		Reason: r.Reason, ContactID: broadcastRef(models.PrefixContact, r.ContactID), ContactName: r.ContactName,
		ContactEmail: r.ContactEmail, CompanyID: broadcastRef(models.PrefixCompany, r.CompanyID),
		CompanyName: r.CompanyName,
		RoomID:      broadcastRef(models.PrefixChannelRoom, r.RoomID), RoomName: r.RoomName, Address: r.Address,
		ConversationID: broadcastRef(models.PrefixConversation, r.ConversationID), SentAt: r.SentAt,
		DeliveredAt: r.DeliveredAt, OpenedAt: r.OpenedAt, ClickedAt: r.ClickedAt, RepliedAt: r.RepliedAt,
		Reactions: r.Reactions, UnsubscribedAt: r.UnsubscribedAt, ConvertedAt: r.ConvertedAt, UpdatedAt: r.UpdatedAt,
	}
}

/*
BroadcastEstimate is how many a broadcast would reach now and why the
rest would be left out.
*/
type BroadcastEstimate struct {
	Object  string         `json:"object"`
	Reach   int            `json:"reach"`
	Skipped map[string]int `json:"skipped"`
	Capped  bool           `json:"capped"`
}

/*
BroadcastTopic is one topic people subscribe to.
*/
type BroadcastTopic struct {
	ID          string    `json:"id"`
	Object      string    `json:"object"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Consent     string    `json:"consent"`
	CreatedAt   time.Time `json:"created_at"`
}

/*
NewBroadcastTopic shapes a topic.
*/
func NewBroadcastTopic(t models.BroadcastTopic) BroadcastTopic {
	return BroadcastTopic{
		ID: models.PublicID(models.PrefixBroadcastTopic, t.ID), Object: "broadcast_topic", Name: t.Name,
		Description: t.Description, Consent: t.Consent, CreatedAt: t.CreatedAt,
	}
}

/*
broadcastRef is the public id of an optional row.
*/
func broadcastRef(prefix models.Prefix, id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := models.PublicID(prefix, *id)

	return &public
}
