/*
|--------------------------------------------------------------------------
| WhatsApp Inbox Resources
|--------------------------------------------------------------------------
|
| A template as the composer shows it, with what it needs filled in
| and how the customer reads it, and where a conversation stands on
| WhatsApp: whether free text can still go out, and until when.
|
*/

package resources

import (
	"encoding/json"
	"time"
)

/*
WhatsAppTemplate is a template as the composer shows it.
*/
type WhatsAppTemplate struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Language       string             `json:"language"`
	Category       string             `json:"category"`
	Status         string             `json:"status"`
	Sendable       bool               `json:"sendable"`
	RejectedReason *string            `json:"rejected_reason"`
	Variables      []WhatsAppVariable `json:"variables"`
	Preview        string             `json:"preview"`
	Content        WhatsAppContent    `json:"content"`
	Bindings       json.RawMessage    `json:"bindings"`
	SyncedAt       time.Time          `json:"synced_at"`
}

/*
WhatsAppVariable is a value a template needs: the part it is in and its
key.
*/
type WhatsAppVariable struct {
	Part string `json:"part"`
	Key  string `json:"key"`
}

/*
WhatsAppConversation is where a conversation stands on WhatsApp.
*/
type WhatsAppConversation struct {
	ChannelID string             `json:"channel_id"`
	Window    WhatsAppWindow     `json:"window"`
	Contact   WhatsAppContact    `json:"contact"`
	Templates []WhatsAppTemplate `json:"templates"`
}

/*
WhatsAppWindow is whether free text can still go out, and until when.
*/
type WhatsAppWindow struct {
	Open      bool       `json:"open"`
	ExpiresAt *time.Time `json:"expires_at"`
}

/*
WhatsAppContact is what a template's variables are usually filled in
with.
*/
type WhatsAppContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
