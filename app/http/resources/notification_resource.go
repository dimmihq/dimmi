/*
|--------------------------------------------------------------------------
| Notification Resources
|--------------------------------------------------------------------------
|
| A member's notification as the bell shows it, their settings matrix,
| their settings, and their push devices. Ids are public ones; a device
| never shows its keys.
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
Notification is one bell row. Resolved says how an action call it asks
about ended, once someone decided it.
*/
type Notification struct {
	ID                 string    `json:"id"`
	Object             string    `json:"object"`
	Trigger            string    `json:"trigger"`
	Title              string    `json:"title"`
	Body               string    `json:"body"`
	Link               string    `json:"link"`
	ActorName          string    `json:"actor_name"`
	ConversationID     *string   `json:"conversation_id"`
	ConversationNumber int64     `json:"conversation_number,omitempty"`
	ActionCallID       *string   `json:"action_call_id"`
	Resolved           string    `json:"resolved,omitempty"`
	Count              int32     `json:"count"`
	Read               bool      `json:"read"`
	Seen               bool      `json:"seen"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

/*
notificationData is what an alert stored in its row.
*/
type notificationData struct {
	Trigger            string `json:"trigger"`
	Title              string `json:"title"`
	Body               string `json:"body"`
	Link               string `json:"link"`
	ActorName          string `json:"actor_name"`
	ConversationID     string `json:"conversation_id"`
	ConversationNumber int64  `json:"conversation_number"`
	ActionCallID       string `json:"action_call_id"`
	Resolved           string `json:"resolved"`
}

/*
NewNotification shapes a row for the API.
*/
func NewNotification(n models.Notification) Notification {
	var d notificationData
	_ = json.Unmarshal(n.Data, &d)
	out := Notification{
		ID: models.PublicID(models.PrefixNotification, n.ID), Object: "notification", Trigger: n.Type,
		Title: d.Title, Body: d.Body, Link: d.Link, ActorName: d.ActorName, ConversationNumber: d.ConversationNumber,
		Resolved: d.Resolved, Count: n.Count, Read: n.ReadAt != nil, Seen: n.SeenAt != nil, CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
	if d.Trigger != "" {
		out.Trigger = d.Trigger
	}
	out.ConversationID = publicOf(models.PrefixConversation, d.ConversationID)
	out.ActionCallID = publicOf(models.PrefixAiActionCall, d.ActionCallID)
	return out
}

/*
NewNotifications shapes rows for the API.
*/
func NewNotifications(rows []models.Notification) []Notification {
	out := make([]Notification, len(rows))
	for i, r := range rows {
		out[i] = NewNotification(r)
	}
	return out
}

func publicOf(prefix models.Prefix, raw string) *string {
	id, err := ids.Parse(raw)
	if err != nil {
		return nil
	}
	public := models.PublicID(prefix, id)
	return &public
}

/*
NotificationCounts are the bell's counts.
*/
type NotificationCounts struct {
	UnreadCount int64 `json:"unread_count"`
	UnseenCount int64 `json:"unseen_count"`
}
