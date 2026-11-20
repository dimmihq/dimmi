/*
|--------------------------------------------------------------------------
| Context And Account Events
|--------------------------------------------------------------------------
|
| What a message answers, changes WhatsApp reports about a customer,
| messages the Cloud API does not pass on, and template changes.
|
*/

package whatsapp

import (
	"encoding/json"
	"fmt"
)

/*
Context is the message a message answers.
*/
type Context struct {
	From string `json:"from"`
	ID   string `json:"id"`
}

/*
System is a change WhatsApp reports about the customer, such as a new
number or a new user id.
*/
type System struct {
	Body           string `json:"body"`
	Type           string `json:"type"`
	WaID           string `json:"wa_id"`
	UserID         string `json:"user_id"`
	PreviousUserID string `json:"previous_user_id"`
}

/*
Unsupported names a kind of message the Cloud API does not deliver, such
as the album notice that comes before each of an album's pictures.
*/
type Unsupported struct {
	Type string `json:"type"`
}

/*
TemplateUpdate is a change Meta made to one of the account's templates.
*/
type TemplateUpdate struct {
	Event    string `json:"event"`
	ID       int64  `json:"message_template_id"`
	Name     string `json:"message_template_name"`
	Language string `json:"message_template_language"`
	Reason   string `json:"reason"`
}

/*
ParseTemplateUpdate reads a template status change.
*/
func ParseTemplateUpdate(raw []byte) (TemplateUpdate, error) {
	var t TemplateUpdate
	if err := json.Unmarshal(raw, &t); err != nil {
		return TemplateUpdate{}, fmt.Errorf("parse whatsapp template update: %w", err)
	}

	return t, nil
}
