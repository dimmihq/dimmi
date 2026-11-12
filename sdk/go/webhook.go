package dimmi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

/*
Event is one webhook delivery: what happened, when, in which workspace,
who caused it, and the object as the REST API shows it under
Data["object"]. Raw is the exact body that was signed.
*/
type Event struct {
	ID          string         `json:"id"`
	Object      string         `json:"object"`
	Type        string         `json:"type"`
	APIVersion  string         `json:"api_version"`
	CreatedAt   time.Time      `json:"created_at"`
	WorkspaceID string         `json:"workspace_id"`
	Sequence    int64          `json:"sequence"`
	Test        bool           `json:"test"`
	TestMode    bool           `json:"test_mode"`
	Truncated   bool           `json:"truncated"`
	Actor       EventActor     `json:"actor"`
	Data        map[string]any `json:"data"`
	Raw         []byte         `json:"-"`
}

// EventActor is who caused an event: member, contact, agent, system or integration.
type EventActor struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
}

// DecodeObject decodes data.object into out, such as an *api.Conversation.
func (e *Event) DecodeObject(out any) error {
	var envelope struct {
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(e.Raw, &envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data.Object, out)
}

// ErrWebhookVerification is returned, wrapped, for any request that fails verification.
var ErrWebhookVerification = errors.New("dimmi: webhook verification failed")

/*
Webhook verifies dimmi's signed webhook requests: Standard Webhooks
signatures over the raw body with the endpoint's whsec_ secret, and a
timestamp no more than five minutes off.
*/
type Webhook struct {
	verifier *standardwebhooks.Webhook
}

// NewWebhook makes a verifier for the endpoint's signing secret, whsec_...
func NewWebhook(secret string) (*Webhook, error) {
	v, err := standardwebhooks.NewWebhook(secret)
	if err != nil {
		return nil, fmt.Errorf("dimmi: webhook secret invalid: %w", err)
	}
	return &Webhook{verifier: v}, nil
}

/*
Verify checks the signature on payload, the raw request body, and
returns the event. Never re-encode the body before verifying it.
*/
func (w *Webhook) Verify(payload []byte, headers http.Header) (*Event, error) {
	if err := w.verifier.Verify(payload, headers); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWebhookVerification, err)
	}
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("%w: body is not an event: %v", ErrWebhookVerification, err)
	}
	event.Raw = payload
	return &event, nil
}

/*
Handler verifies each request and calls handle with its event. A request
that fails verification gets 400; an error from handle gets 500, so
dimmi retries it; anything else gets 204.
*/
func (w *Webhook) Handler(handle func(*http.Request, *Event) error) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(rw, "unreadable body", http.StatusBadRequest)
			return
		}
		event, err := w.Verify(body, r.Header)
		if err != nil {
			http.Error(rw, "invalid signature", http.StatusBadRequest)
			return
		}
		if err := handle(r, event); err != nil {
			http.Error(rw, "handler failed", http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	})
}
