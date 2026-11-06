/*
|--------------------------------------------------------------------------
| Inbound Email Resources
|--------------------------------------------------------------------------
|
| A received email as the activity log shows it: who sent it, when, what
| kind of mail it was, whether its sender was proven, and what became of
| it, the conversation and message it turned into or why it was ignored.
| Its content stays in the conversation; the raw message is never part
| of the resource.
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
InboundEmail is one received email.
*/
type InboundEmail struct {
	ID             string          `json:"id"`
	Object         string          `json:"object"`
	Status         string          `json:"status"`
	Classification *string         `json:"classification"`
	IgnoreReason   *string         `json:"ignore_reason"`
	From           *string         `json:"from"`
	Subject        *string         `json:"subject"`
	EnvelopeTo     []string        `json:"envelope_to"`
	Auth           json.RawMessage `json:"auth"`
	SizeBytes      int64           `json:"size_bytes"`
	Conversation   *ObjectRef      `json:"conversation"`
	Message        *ObjectRef      `json:"message"`
	Error          *string         `json:"error"`
	ReceivedAt     time.Time       `json:"received_at"`
	ProcessedAt    *time.Time      `json:"processed_at"`
}

/*
ObjectRef names another resource by its public id.
*/
type ObjectRef struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

/*
NewInboundEmails shapes a page of received emails.
*/
func NewInboundEmails(rows []models.InboundEmail) []InboundEmail {
	out := make([]InboundEmail, 0, len(rows))
	for _, r := range rows {
		auth := r.Auth
		if len(auth) == 0 {
			auth = json.RawMessage(`null`)
		}

		out = append(out, InboundEmail{
			ID: models.PublicID(models.PrefixInboundEmail, r.ID), Object: "inbound_email", Status: r.Status,
			Classification: r.Classification, IgnoreReason: r.IgnoreReason, From: r.FromAddress, Subject: r.Subject,
			EnvelopeTo: r.EnvelopeTo, Auth: auth, SizeBytes: r.SizeBytes,
			Conversation: ref(models.PrefixConversation, "conversation", r.ConversationID),
			Message:      ref(models.PrefixMessage, "message", r.MessageID), Error: r.Error,
			ReceivedAt: r.ReceivedAt, ProcessedAt: r.ProcessedAt,
		})
	}

	return out
}

/*
ref names a row by its public id, or nothing.
*/
func ref(prefix models.Prefix, object string, id *ids.UUID) *ObjectRef {
	if id == nil {
		return nil
	}

	return &ObjectRef{ID: models.PublicID(prefix, *id), Object: object}
}
