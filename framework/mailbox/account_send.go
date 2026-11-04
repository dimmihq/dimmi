/*
|--------------------------------------------------------------------------
| Sending and Watching Accounts
|--------------------------------------------------------------------------
|
| A connected mailbox also sends, so replies sit in its own Sent folder,
| and may announce changes so a sync starts early. An announcement only
| wakes a sync: the cursor stays the truth.
|
*/

package mailbox

import "time"

/*
Sent is what the provider says about a sent message. Any field may be
empty when the provider tells only later.
*/
type Sent struct {
	ProviderID   string
	RFCMessageID string
	ThreadID     string
}

/*
SendInput is a message to send: the raw MIME, and the provider's thread
to keep it in when known.
*/
type SendInput struct {
	Raw      []byte
	ThreadID string
}

/*
WatchInput asks a provider to announce changes. Topic and the URLs are
each used by the providers that need them; Existing is the state of the
watch being renewed, empty for a new one.
*/
type WatchInput struct {
	Topic           string
	NotificationURL string
	LifecycleURL    string
	ClientState     string
	Existing        []byte
}

/*
Watching is a running watch: its state to keep and when it ends.
*/
type Watching struct {
	State     []byte
	ExpiresAt time.Time
}
