/*
|--------------------------------------------------------------------------
| Contact And Inbox Resources
|--------------------------------------------------------------------------
|
| A customer as the team sees them: who they are, how to reach them, the
| workspace's own attributes about them and whether they are blocked.
| Lists and conversations carry the short form. Inboxes show their name,
| and channels their type and name.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Contact is one customer.
*/
type Contact struct {
	ID               string          `json:"id"`
	Object           string          `json:"object"`
	Kind             string          `json:"kind"`
	Name             *string         `json:"name"`
	Email            *string         `json:"email"`
	EmailVerified    bool            `json:"email_verified"`
	ExternalID       *string         `json:"external_id"`
	Phone            *string         `json:"phone"`
	AvatarURL        *string         `json:"avatar_url"`
	Locale           *string         `json:"locale"`
	Timezone         *string         `json:"timezone"`
	CustomAttributes json.RawMessage `json:"custom_attributes"`
	Blocked          bool            `json:"blocked"`
	FirstSeenAt      *time.Time      `json:"first_seen_at"`
	LastSeenAt       *time.Time      `json:"last_seen_at"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

/*
NewContact shapes the contact.
*/
func NewContact(c models.Contact) Contact {
	attrs := c.CustomAttributes
	if len(attrs) == 0 {
		attrs = json.RawMessage(`{}`)
	}

	return Contact{
		ID: models.PublicID(models.PrefixContact, c.ID), Object: "contact", Kind: c.Kind, Name: c.Name,
		Email: c.Email, EmailVerified: c.EmailVerified, ExternalID: c.ExternalID, Phone: c.Phone,
		AvatarURL: c.AvatarUrl, Locale: c.Locale, Timezone: c.Timezone, CustomAttributes: attrs,
		Blocked: c.BlockedAt != nil, FirstSeenAt: c.FirstSeenAt, LastSeenAt: c.LastSeenAt, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

/*
NewContacts shapes a list of contacts, in the order given.
*/
func NewContacts(rows []models.Contact) []Contact {
	out := make([]Contact, 0, len(rows))
	for _, c := range rows {
		out = append(out, NewContact(c))
	}

	return out
}

/*
ContactSummary is the short form of a contact.
*/
type ContactSummary struct {
	ID        string  `json:"id"`
	Object    string  `json:"object"`
	Kind      string  `json:"kind"`
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

/*
NewContactSummary shapes the short form of the contact.
*/
func NewContactSummary(c models.Contact) ContactSummary {
	return ContactSummary{
		ID: models.PublicID(models.PrefixContact, c.ID), Object: "contact", Kind: c.Kind, Name: c.Name,
		Email: c.Email, AvatarURL: c.AvatarUrl,
	}
}

/*
Inbox is one inbox of the workspace.
*/
type Inbox struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

/*
NewInbox shapes the inbox.
*/
func NewInbox(i models.Inbox) Inbox {
	return Inbox{
		ID: models.PublicID(models.PrefixInbox, i.ID), Object: "inbox", Name: i.Name, CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

/*
NewInboxes shapes a list of inboxes, in the order given.
*/
func NewInboxes(rows []models.Inbox) []Inbox {
	out := make([]Inbox, 0, len(rows))
	for _, i := range rows {
		out = append(out, NewInbox(i))
	}

	return out
}

/*
Named is the short form of a row known by its name: an inbox, a team or
a member.
*/
type Named struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

/*
Channel is the short form of a channel connection.
*/
type Channel struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Type   string `json:"type"`
	Name   string `json:"name"`
}

/*
NewChannel shapes the short form of the connection.
*/
func NewChannel(c models.ChannelConnection) Channel {
	return Channel{
		ID: models.PublicID(models.PrefixChannelConnection, c.ID), Object: "channel_connection", Type: c.Type,
		Name: c.Name,
	}
}
