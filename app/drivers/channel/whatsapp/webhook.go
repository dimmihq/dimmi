/*
|--------------------------------------------------------------------------
| Webhook Deliveries
|--------------------------------------------------------------------------
|
| One delivery from Meta holds entries, one per WhatsApp account, each
| holding changes, one per subscribed field. A change of the messages
| field carries the number it reached, the customers who wrote with the
| name and ids WhatsApp knows them by, their messages, and status
| reports of messages the business sent. Template changes carry the
| template and its new status. Everything is read into plain shapes
| here; nothing outside the package parses Meta's JSON.
|
*/

package whatsapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

/*
The webhook fields the channel reads.
*/
const (
	FieldMessages       = "messages"
	FieldTemplateStatus = "message_template_status_update"
	FieldAccount        = "account_update"
	FieldQuality        = "phone_number_quality_update"
	FieldName           = "phone_number_name_update"
	FieldEchoes         = "smb_message_echoes"
)

/*
Delivery is one webhook delivery.
*/
type Delivery struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

/*
Entry is what changed on one WhatsApp account.
*/
type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

/*
Change is one field's change, its value kept raw until its field says
how to read it.
*/
type Change struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

/*
ParseDelivery reads a delivery.
*/
func ParseDelivery(raw []byte) (Delivery, error) {
	var d Delivery
	if err := json.Unmarshal(raw, &d); err != nil {
		return Delivery{}, fmt.Errorf("parse whatsapp delivery: %w", err)
	}

	return d, nil
}

/*
Messages is the value of a messages change: the number it reached, the
customers in it, their messages and status reports of the business's.
Echoes are the business's own messages typed on the phone app.
*/
type Messages struct {
	Product  string    `json:"messaging_product"`
	Metadata Metadata  `json:"metadata"`
	Contacts []Contact `json:"contacts"`
	Messages []Message `json:"messages"`
	Statuses []Status  `json:"statuses"`
	Echoes   []Message `json:"message_echoes"`
}

/*
Metadata names the business number a change reached.
*/
type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

/*
ParseMessages reads a messages change.
*/
func ParseMessages(raw []byte) (Messages, error) {
	var m Messages
	if err := json.Unmarshal(raw, &m); err != nil {
		return Messages{}, fmt.Errorf("parse whatsapp messages: %w", err)
	}

	return m, nil
}

/*
unixTime reads a Unix time in seconds, or the fallback.
*/
func unixTime(seconds string, fallback time.Time) time.Time {
	n, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}

	return time.Unix(n, 0).UTC()
}
