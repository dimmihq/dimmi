package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
fakeHTTP returns a client whose requests a fresh fake answers, refusing
any request no stub matches.
*/
func fakeHTTP(t *testing.T) (*httpclient.Client, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	return client, httpclienttest.New(client).PreventStrayRequests()
}

/*
sse builds a server-sent event stream response from event name and
data pairs; an empty name writes a data-only event.
*/
func sse(events ...string) httpclient.Response {
	var b strings.Builder
	for i := 0; i+1 < len(events); i += 2 {
		if events[i] != "" {
			b.WriteString("event: " + events[i] + "\n")
		}
		b.WriteString("data: " + events[i+1] + "\n\n")
	}
	return httpclient.Response{
		Status: fhttp.StatusOK,
		Header: fhttp.Header{"Content-Type": {"text/event-stream"}},
		Body:   []byte(b.String()),
	}
}

/*
jsonResponse builds a JSON response with a status and headers.
*/
func jsonResponse(status int, body string, headers ...string) httpclient.Response {
	h := fhttp.Header{"Content-Type": {"application/json"}}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Set(headers[i], headers[i+1])
	}
	return httpclient.Response{Status: status, Header: h, Body: []byte(body)}
}

/*
apiError is a provider error body.
*/
func apiError(errType, message string) string {
	return `{"type":"error","error":{"type":"` + errType + `","code":"` + errType + `","message":"` + message + `"}}`
}

/*
drain reads a stream to its end and returns its events.
*/
func drain(t *testing.T, stream contracts.ChatStream) []contracts.StreamEvent {
	t.Helper()

	var events []contracts.StreamEvent
	for {
		ev, err := stream.Next(t.Context())
		if errors.Is(err, io.EOF) {
			return events
		}
		require.NoError(t, err)
		events = append(events, ev)
	}
}

/*
drainErr reads a stream until it fails and returns the error.
*/
func drainErr(t *testing.T, stream contracts.ChatStream) error {
	t.Helper()

	for {
		if _, err := stream.Next(t.Context()); err != nil {
			return err
		}
	}
}

/*
kinds lists the events' kinds in order.
*/
func kinds(events []contracts.StreamEvent) []contracts.StreamEventKind {
	out := make([]contracts.StreamEventKind, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Kind)
	}
	return out
}

/*
find returns the first event of the kind.
*/
func find(events []contracts.StreamEvent, kind contracts.StreamEventKind) contracts.StreamEvent {
	for _, ev := range events {
		if ev.Kind == kind {
			return ev
		}
	}
	return contracts.StreamEvent{}
}

/*
llmError unwraps the typed failure.
*/
func llmError(t *testing.T, err error) *contracts.LLMError {
	t.Helper()

	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	require.True(t, ok, "error %v is not an LLMError", err)
	return llmErr
}

/*
body decodes the request's JSON body.
*/
func body(t *testing.T, req httpclienttest.SentRequest) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.Unmarshal(req.Body, &out))
	return out
}

/*
object reads a nested JSON object.
*/
func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

/*
list reads a nested JSON array.
*/
func list(v any) []any {
	l, _ := v.([]any)
	return l
}

/*
request builds a chat request with one user turn.
*/
func request(model string) contracts.ChatRequest {
	return contracts.ChatRequest{
		Model:  model,
		System: []contracts.SystemBlock{{Text: "Be brief.", Cacheable: true}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "hi"}}},
		},
	}
}

/*
canceled waits for the context to end, for scripted drivers.
*/
func canceled(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
