/*
|--------------------------------------------------------------------------
| Thread And Message Keys
|--------------------------------------------------------------------------
|
| A customer has one thread per business number. It is keyed by their
| business-scoped user id when Meta sends one, since a customer with a
| username may hide their phone, and by their phone's WhatsApp id
| otherwise. A message is recognized by Meta's own id, so a retry, an
| echo and a history import of the same message land once.
|
*/

package whatsapp

import "strings"

/*
Type names the whatsapp connection type.
*/
const Type = "whatsapp"

/*
Prefixes of thread keys: by user id and by phone.
*/
const (
	userKeyPrefix  = "whatsapp:u:"
	phoneKeyPrefix = "whatsapp:p:"
)

/*
ThreadKey is the customer's thread on the number: by their user id when
known, else by their phone's WhatsApp id; empty when neither is.
*/
func ThreadKey(userID, waID string) string {
	switch {
	case userID != "":
		return userKeyPrefix + userID
	case waID != "":
		return phoneKeyPrefix + waID
	default:
		return ""
	}
}

/*
Recipient is who a thread key reaches: their phone's WhatsApp id or
their user id.
*/
type Recipient struct {
	WaID   string
	UserID string
}

/*
ParseThreadKey reads who a thread key reaches, reporting false for a key
of another channel.
*/
func ParseThreadKey(key string) (Recipient, bool) {
	switch {
	case strings.HasPrefix(key, userKeyPrefix) && len(key) > len(userKeyPrefix):
		return Recipient{UserID: strings.TrimPrefix(key, userKeyPrefix)}, true
	case strings.HasPrefix(key, phoneKeyPrefix) && len(key) > len(phoneKeyPrefix):
		return Recipient{WaID: strings.TrimPrefix(key, phoneKeyPrefix)}, true
	default:
		return Recipient{}, false
	}
}

/*
DedupeKey recognizes a message by Meta's id for it.
*/
func DedupeKey(messageID string) string {
	return "whatsapp:" + messageID
}

/*
PhoneOf returns the WhatsApp id as an E.164 phone number.
*/
func PhoneOf(waID string) string {
	if waID == "" || strings.HasPrefix(waID, "+") {
		return waID
	}

	return "+" + waID
}
