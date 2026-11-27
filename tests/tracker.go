/*
|--------------------------------------------------------------------------
| Error Tracker In Tests
|--------------------------------------------------------------------------
|
| The error tracker (Rustrak, which reads what Sentry SDKs send) leaves
| the process, so a test points the application at a stand-in that
| keeps every envelope. TrackedSpans reads the AI spans out of them the
| way the tracker stores them, and TrackerKind classifies one as the
| tracker does, so a test asserts what its Agents pages will show.
|
*/

package tests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
Tracker is a stand-in error tracker that keeps every envelope sent.
*/
type Tracker struct {
	dsn       string
	mu        sync.Mutex
	envelopes [][]byte
}

/*
NewTracker starts a stand-in tracker for the test.
*/
func NewTracker(tb testing.TB) *Tracker {
	tb.Helper()
	tr := &Tracker{}
	url := testkit.Serve(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			tr.mu.Lock()
			tr.envelopes = append(tr.envelopes, body)
			tr.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	tr.dsn = strings.Replace(url, "http://", "http://public@", 1) + "/1"

	return tr
}

/*
Option points the application at the tracker.
*/
func (tr *Tracker) Option() Option {
	return WithEnv("DIMMI_TELEMETRY__SENTRY_DSN=" + tr.dsn)
}

/*
TrackedSpan is one span as the tracker stores it: a transaction's root,
promoted from its trace context, or one of its spans.
*/
type TrackedSpan struct {
	Transaction string         `json:"-"`
	TraceID     string         `json:"trace_id"`
	SpanID      string         `json:"span_id"`
	ParentID    string         `json:"parent_span_id"`
	Op          string         `json:"op"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Start       string         `json:"start_timestamp"`
	Data        map[string]any `json:"data"`
}

/*
Attr returns one of the span's attributes as text, or "".
*/
func (s TrackedSpan) Attr(key string) string {
	v, _ := s.Data[key].(string)

	return v
}

/*
Kind is how the tracker classifies the span: "agent", "ai_client",
"tool", "handoff", or "" for a span that is not AI work.
*/
func (s TrackedSpan) Kind() string {
	return TrackerKind(s.Op, s.Data)
}

/*
TrackedSpans flushes the application's telemetry and returns every span
of every transaction the tracker received, roots first.
*/
func (tc *TestCase) TrackedSpans(tr *Tracker) []TrackedSpan {
	tc.T.Helper()
	require.True(tc.T, tc.App.Telemetry.Flush(tc.T.Context()), "flush the error tracker")

	tr.mu.Lock()
	defer tr.mu.Unlock()

	var spans []TrackedSpan
	for _, envelope := range tr.envelopes {
		spans = append(spans, transactionSpans(tc.T, envelope)...)
	}

	return spans
}

/*
transactionSpans reads the transactions of one envelope: a header line,
then an item header line and its payload line per item.
*/
func transactionSpans(tb testing.TB, envelope []byte) []TrackedSpan {
	tb.Helper()
	lines := bufio.NewScanner(bytes.NewReader(envelope))
	lines.Buffer(make([]byte, 0, 1<<20), 16<<20)
	lines.Scan()

	var spans []TrackedSpan
	for lines.Scan() {
		var item struct {
			Type string `json:"type"`
		}
		require.NoError(tb, json.Unmarshal(lines.Bytes(), &item))
		lines.Scan()
		if item.Type != "transaction" {
			continue
		}
		var tx struct {
			Transaction string                      `json:"transaction"`
			Start       string                      `json:"start_timestamp"`
			Spans       []TrackedSpan               `json:"spans"`
			Contexts    struct{ Trace TrackedSpan } `json:"contexts"`
		}
		require.NoError(tb, json.Unmarshal(lines.Bytes(), &tx))
		root := tx.Contexts.Trace
		root.Start = tx.Start
		for _, s := range append([]TrackedSpan{root}, tx.Spans...) {
			s.Transaction = tx.Transaction
			spans = append(spans, s)
		}
	}

	return spans
}

/*
TrackerKind classifies a span as the tracker does, a port of
apps/server/src/services/gen_ai.rs (itself Relay's normalize_ai):

  - is_ai_span: "True if gen_ai.operation.type or gen_ai.operation.name
    is present in data, or if op starts with gen_ai. or ai."
  - normalize_operation_type reads gen_ai.operation.name first, then
    the op, through infer_operation_type: "gen_ai.invoke_agent" and
    "invoke_agent" are "agent", "execute_tool" is "tool", "handoff" is
    "handoff", and anything else, such as "chat", is the default
    "ai_client".

Only "agent" spans count as agent runs and name the agents of a trace
(services/span.rs agent_summary and trace_agent_names), so a trace
whose AI spans have no "agent" span shows as "(unnamed agent)".
*/
func TrackerKind(op string, data map[string]any) string {
	name, hasName := data["gen_ai.operation.name"].(string)
	_, hasType := data["gen_ai.operation.type"]
	aiOp := strings.HasPrefix(op, "gen_ai.") || strings.HasPrefix(op, "ai.")
	if !hasName && !hasType && !aiOp {
		return ""
	}
	if !hasName {
		name = op
	}
	switch name {
	case "gen_ai.invoke_agent", "invoke_agent", "gen_ai.create_agent", "create_agent":
		return "agent"
	case "gen_ai.execute_tool", "execute_tool":
		return "tool"
	case "gen_ai.handoff", "handoff":
		return "handoff"
	}

	return "ai_client"
}
