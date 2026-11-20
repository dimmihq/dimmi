/*
|--------------------------------------------------------------------------
| Customer Messages
|--------------------------------------------------------------------------
|
| A message a customer sent, with the customer as WhatsApp knows them and
| what it carries: text, a file, or a place.
|
*/

package whatsapp

import (
	"encoding/json"
	"time"
)

/*
Contact is a customer as WhatsApp knows them: their name, their phone's
WhatsApp id when shared, and their business-scoped user id.
*/
type Contact struct {
	Profile struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"profile"`
	WaID   string `json:"wa_id"`
	UserID string `json:"user_id"`
}

/*
Message is one message a customer sent.
*/
type Message struct {
	From        string          `json:"from"`
	FromUserID  string          `json:"from_user_id"`
	To          string          `json:"to"`
	ID          string          `json:"id"`
	Timestamp   string          `json:"timestamp"`
	Type        string          `json:"type"`
	Text        *Text           `json:"text"`
	Image       *MediaRef       `json:"image"`
	Video       *MediaRef       `json:"video"`
	Audio       *MediaRef       `json:"audio"`
	Document    *MediaRef       `json:"document"`
	Sticker     *MediaRef       `json:"sticker"`
	Location    *Location       `json:"location"`
	Contacts    []SharedCard    `json:"contacts"`
	Interactive *Interactive    `json:"interactive"`
	Button      *Button         `json:"button"`
	Reaction    *Reaction       `json:"reaction"`
	Context     *Context        `json:"context"`
	System      *System         `json:"system"`
	Unsupported *Unsupported    `json:"unsupported"`
	Referral    json.RawMessage `json:"referral"`
}

/*
Text is a text message's body.
*/
type Text struct {
	Body string `json:"body"`
}

/*
MediaRef is a file in a message: Meta's id for it, its type, its name
for a document, its caption, and whether audio is a voice note.
*/
type MediaRef struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
	Voice    bool   `json:"voice"`
}

/*
Location is a place a customer shared.
*/
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	URL       string  `json:"url"`
}

/*
SentAt reads the message's time, or the fallback when it has none.
*/
func (m Message) SentAt(fallback time.Time) time.Time {
	return unixTime(m.Timestamp, fallback)
}

/*
Media returns the file the message carries, nil for one without.
*/
func (m Message) Media() *MediaRef {
	switch m.Type {
	case "image":
		return m.Image
	case "video":
		return m.Video
	case "audio":
		return m.Audio
	case "document":
		return m.Document
	case "sticker":
		return m.Sticker
	default:
		return nil
	}
}
