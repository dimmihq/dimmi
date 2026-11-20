/*
|--------------------------------------------------------------------------
| Deliveries
|--------------------------------------------------------------------------
|
| Meta posts what happens in a Page's or an Instagram account's threads
| as deliveries: an object naming the platform and entries, one per
| account, each holding events. An event sits under "messaging" when this
| app answers the thread, under "standby" when another app does, and
| under "changes" with a field name on newer setups; all three carry the
| same event, so all three are read. An event names two parties, and
| which one is the customer depends on who wrote: the account's own
| messages come back as echoes with the parties swapped.
|
| Meta's ids and timestamps arrive as numbers in some events and as text
| in others, so both are read.
|
*/

package social

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
ErrNotDelivery is returned for a body that is not a Messenger or
Instagram delivery.
*/
var ErrNotDelivery = errors.New("social: not a delivery")

/*
Event is one thing that happened in a thread. Account is the Page or
Instagram account it happened on and Customer the person on the other
side, whoever wrote. Standby says another app answers the thread, so
this one only watches. Exactly one of the parts is set; an event with
none is one the inbox has no use for.
*/
type Event struct {
	Platform Platform
	Account  string
	Customer string
	At       time.Time
	Standby  bool

	Message  *Message
	Reaction *Reaction
	Read     *Read
	Edit     *Edit
	Postback *Postback
	Control  *Control
}

/*
flexible reads a value Meta sends as a number in some events and as
text in others.
*/
type flexible string

/*
UnmarshalJSON reads a JSON string or number as its text.
*/
func (f *flexible) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if text == "null" {
		return nil
	}

	if unquoted, err := strconv.Unquote(text); err == nil {
		text = unquoted
	}

	*f = flexible(text)

	return nil
}

type party struct {
	ID flexible `json:"id"`
}

/*
raw is an event as Meta sends it.
*/
type raw struct {
	Sender    party     `json:"sender"`
	Recipient party     `json:"recipient"`
	Timestamp flexible  `json:"timestamp"`
	Message   *Message  `json:"message"`
	Reaction  *Reaction `json:"reaction"`
	Read      *Read     `json:"read"`
	Edit      *Edit     `json:"message_edit"`
	Postback  *Postback `json:"postback"`
	Passed    *Control  `json:"pass_thread_control"`
	Taken     *Control  `json:"take_thread_control"`
}

type entry struct {
	ID        flexible `json:"id"`
	Time      flexible `json:"time"`
	Messaging []raw    `json:"messaging"`
	Standby   []raw    `json:"standby"`
	Changes   []struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	} `json:"changes"`
}

/*
ParseDelivery reads a delivery into its events, in the order Meta sent
them.
*/
func ParseDelivery(body []byte) ([]Event, error) {
	var delivery struct {
		Object string  `json:"object"`
		Entry  []entry `json:"entry"`
	}

	if err := json.Unmarshal(body, &delivery); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotDelivery, err)
	}

	platform, ok := PlatformOf(delivery.Object)
	if !ok {
		return nil, fmt.Errorf("%w: object %q", ErrNotDelivery, delivery.Object)
	}

	var out []Event

	for _, e := range delivery.Entry {
		at := entryTime(e)

		for _, r := range e.Messaging {
			out = append(out, eventOf(platform, string(e.ID), r, at))
		}

		for _, r := range e.Standby {
			event := eventOf(platform, string(e.ID), r, at)
			event.Standby = true
			out = append(out, event)
		}

		for _, change := range e.Changes {
			var r raw
			if json.Unmarshal(change.Value, &r) == nil && (r.Sender.ID != "" || r.Recipient.ID != "") {
				out = append(out, eventOf(platform, string(e.ID), r, at))
			}
		}
	}

	return out, nil
}

/*
eventOf settles who the customer is and when the event happened.
*/
func eventOf(platform Platform, account string, r raw, fallback time.Time) Event {
	event := Event{
		Platform: platform, Account: account, Customer: string(r.Sender.ID), At: timeOf(r.Timestamp, fallback),
		Message: r.Message, Reaction: r.Reaction, Read: r.Read, Edit: r.Edit, Postback: r.Postback,
	}

	if string(r.Sender.ID) == account || (r.Message != nil && r.Message.Echo) {
		event.Customer = string(r.Recipient.ID)
	}

	if r.Passed != nil {
		event.Control = r.Passed
	}

	if r.Taken != nil {
		event.Control = r.Taken
	}

	return event
}

func entryTime(e entry) time.Time {
	return timeOf(e.Time, time.Time{})
}

/*
timeOf reads Meta's timestamp, milliseconds in most events and seconds
in a few, falling back when it is missing.
*/
func timeOf(stamp flexible, fallback time.Time) time.Time {
	n, err := strconv.ParseInt(string(stamp), 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}

	const firstMillisecond = 100_000_000_000
	if n < firstMillisecond {
		return time.Unix(n, 0).UTC()
	}

	return time.UnixMilli(n).UTC()
}
