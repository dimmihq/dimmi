package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
fixedNow is the moment every test store believes it is.
*/
func fixedNow() time.Time { return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC) }

/*
adaRefund is a full refund of Ada's refundable annual invoice.
*/
func adaRefund() refundRequest {
	return refundRequest{Customer: "user_42", InvoiceID: "inv_1001", Amount: 144, Reason: "bought by mistake"}
}

/*
requireProblem asserts err is a problem with the given code.
*/
func requireProblem(t *testing.T, err error, code string) {
	t.Helper()

	var p *problemError
	require.ErrorAs(t, err, &p)
	assert.Equal(t, code, p.code)
}

func TestRefunds(t *testing.T) {
	t.Parallel()

	t.Run("an annual invoice paid within thirty days is refunded", func(t *testing.T) {
		t.Parallel()
		st := newStore(fixedNow)

		refund, err := st.refund(adaRefund(), "")
		require.NoError(t, err)
		assert.InDelta(t, 144.0, refund.Amount, 0.001)
		assert.Equal(t, "succeeded", refund.Status)

		list, err := st.invoiceList("user_42")
		require.NoError(t, err)
		assert.Equal(t, "refunded", list.Invoices[0].Status)
	})

	t.Run("a monthly plan is never refunded", func(t *testing.T) {
		t.Parallel()

		_, err := newStore(fixedNow).refund(refundRequest{
			Customer: "user_7", InvoiceID: "inv_1002", Amount: 36, Reason: "too expensive",
		}, "")
		requireProblem(t, err, "monthly_not_refundable")
	})

	t.Run("an annual invoice paid more than thirty days ago is refused", func(t *testing.T) {
		t.Parallel()

		_, err := newStore(fixedNow).refund(refundRequest{
			Customer: "user_99", InvoiceID: "inv_0950", Amount: 100, Reason: "team shrank",
		}, "")
		requireProblem(t, err, "refund_window_passed")
		assert.Contains(t, err.Error(), "45 days ago")
	})

	t.Run("a second refund never returns money already refunded", func(t *testing.T) {
		t.Parallel()
		st := newStore(fixedNow)

		_, err := st.refund(adaRefund(), "")
		require.NoError(t, err)

		_, err = st.refund(adaRefund(), "")
		requireProblem(t, err, "amount_exceeds_balance")
	})

	t.Run("another customer's invoice is not found", func(t *testing.T) {
		t.Parallel()

		req := adaRefund()
		req.Customer = "user_7"
		_, err := newStore(fixedNow).refund(req, "")
		requireProblem(t, err, "invoice_not_found")
	})

	t.Run("a request without a reason is invalid", func(t *testing.T) {
		t.Parallel()

		req := adaRefund()
		req.Reason = " "
		_, err := newStore(fixedNow).refund(req, "")
		requireProblem(t, err, "reason_required")
	})
}

func TestRefundIdempotency(t *testing.T) {
	t.Parallel()

	t.Run("a retry with the same key replays the refund without moving money again", func(t *testing.T) {
		t.Parallel()
		st := newStore(fixedNow)

		first, err := st.refund(adaRefund(), "key-1")
		require.NoError(t, err)
		assert.False(t, first.Replayed)

		again, err := st.refund(adaRefund(), "key-1")
		require.NoError(t, err)
		assert.Equal(t, first.ID, again.ID)
		assert.True(t, again.Replayed)

		list, err := st.invoiceList("user_42")
		require.NoError(t, err)
		assert.InDelta(t, 144.0, list.Invoices[0].AmountRefunded, 0.001)
	})

	t.Run("a refusal is replayed too", func(t *testing.T) {
		t.Parallel()
		st := newStore(fixedNow)
		req := refundRequest{Customer: "user_7", InvoiceID: "inv_1002", Amount: 36, Reason: "too expensive"}

		_, err := st.refund(req, "key-2")
		requireProblem(t, err, "monthly_not_refundable")

		_, err = st.refund(req, "key-2")
		requireProblem(t, err, "monthly_not_refundable")
	})

	t.Run("a key reused for a different request is refused", func(t *testing.T) {
		t.Parallel()
		st := newStore(fixedNow)

		_, err := st.refund(adaRefund(), "key-3")
		require.NoError(t, err)

		other := adaRefund()
		other.Amount = 10
		_, err = st.refund(other, "key-3")
		requireProblem(t, err, "idempotency_key_reused")
	})
}

/*
call sends one request to the REST API and decodes the JSON answer.
*/
func call(t *testing.T, api http.Handler, req *http.Request) (int, map[string]any) {
	t.Helper()

	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return rec.Code, body
}

/*
apiRequest builds a request carrying the demo bearer token.
*/
func apiRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer demo-token")

	return req
}

func TestRESTAPI(t *testing.T) {
	t.Parallel()

	newAPI := func() http.Handler {
		api := &restAPI{store: newStore(fixedNow), journal: newJournal(io.Discard, fixedNow), token: "demo-token"}

		return api.routes()
	}

	t.Run("a request without the bearer token is rejected", func(t *testing.T) {
		t.Parallel()

		req := apiRequest(t, http.MethodGet, "/api/customers/user_42/subscription", "")
		req.Header.Del("Authorization")
		status, _ := call(t, newAPI(), req)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("the subscription shows plan, interval and amount", func(t *testing.T) {
		t.Parallel()

		status, body := call(t, newAPI(), apiRequest(t, http.MethodGet, "/api/customers/user_42/subscription", ""))
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "pro", body["plan"])
		assert.Equal(t, "year", body["billing_interval"])
		assert.InDelta(t, 144.0, body["amount"], 0.001)
	})

	t.Run("an order of another customer is not found", func(t *testing.T) {
		t.Parallel()

		status, _ := call(t, newAPI(), apiRequest(t, http.MethodGet, "/api/orders/ord_5001?customer=user_7", ""))
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("the same idempotency key returns the same refund", func(t *testing.T) {
		t.Parallel()
		api := newAPI()
		body := `{"customer":"user_42","invoice_id":"inv_1001","amount":144,"reason":"bought by mistake"}`

		first := apiRequest(t, http.MethodPost, "/api/refunds", body)
		first.Header.Set("Idempotency-Key", "abc")
		status, created := call(t, api, first)
		require.Equal(t, http.StatusCreated, status)

		again := apiRequest(t, http.MethodPost, "/api/refunds", body)
		again.Header.Set("Idempotency-Key", "abc")
		status, replayed := call(t, api, again)
		require.Equal(t, http.StatusCreated, status)
		assert.Equal(t, created["id"], replayed["id"])
		assert.Equal(t, true, replayed["replayed"])
	})

	t.Run("a refund past the window is refused with a reason", func(t *testing.T) {
		t.Parallel()
		body := `{"customer":"user_99","invoice_id":"inv_0950","amount":100,"reason":"team shrank"}`

		status, answer := call(t, newAPI(), apiRequest(t, http.MethodPost, "/api/refunds", body))
		assert.Equal(t, http.StatusUnprocessableEntity, status)
		assert.Equal(t, map[string]any{
			"code":    "refund_window_passed",
			"message": "invoice inv_0950 was paid 45 days ago; annual plans can be refunded within 30 days of payment",
		}, answer["error"])
	})
}

func TestMCPServer(t *testing.T) {
	t.Parallel()

	t.Run("a request without the bearer token is rejected", func(t *testing.T) {
		t.Parallel()

		handler, err := newMCPHandler(newStore(fixedNow), newJournal(io.Discard, fixedNow), "demo-mcp-token")
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("issue_refund refunds once and replays a retry with the same key", func(t *testing.T) {
		t.Parallel()
		var log bytes.Buffer
		session := connect(t, &log)
		args := map[string]any{
			"customer_id": "user_42", "invoice_id": "inv_1001", "amount": 144,
			"reason": "bought by mistake", "idempotency_key": "mcp-1",
		}

		first, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "issue_refund", Arguments: args})
		require.NoError(t, err)
		require.False(t, first.IsError)

		again, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "issue_refund", Arguments: args})
		require.NoError(t, err)
		require.False(t, again.IsError)
		assert.Contains(t, text(t, again), "no new money moved")
		assert.Contains(t, log.String(), "idempotency_key=mcp-1")
	})

	t.Run("a refund the policy refuses is a tool error with the reason", func(t *testing.T) {
		t.Parallel()
		session := connect(t, io.Discard)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "issue_refund", Arguments: map[string]any{
			"customer_id": "user_7", "invoice_id": "inv_1002", "amount": 36, "reason": "too expensive",
		}})
		require.NoError(t, err)
		require.True(t, res.IsError)
		assert.Contains(t, text(t, res), "only annual plans can be refunded")
	})

	t.Run("read tools are read-only and refunds are destructive", func(t *testing.T) {
		t.Parallel()
		session := connect(t, io.Discard)

		list, err := session.ListTools(t.Context(), nil)
		require.NoError(t, err)

		hints := map[string]*mcp.ToolAnnotations{}
		for _, tool := range list.Tools {
			hints[tool.Name] = tool.Annotations
		}
		assert.True(t, hints["get_subscription"].ReadOnlyHint)
		assert.True(t, hints["list_invoices"].ReadOnlyHint)
		assert.False(t, hints["issue_refund"].ReadOnlyHint)
		assert.True(t, *hints["issue_refund"].DestructiveHint)
	})
}

/*
connect starts the Lovelace MCP server in memory and returns a connected
client session, journaling to log.
*/
func connect(t *testing.T, log io.Writer) *mcp.ClientSession {
	t.Helper()

	srv, err := newMCPServer(newStore(fixedNow), newJournal(log, fixedNow))
	require.NoError(t, err)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
			t.Log(err)
		}
	})

	return session
}

/*
text returns the first text block of a tool result.
*/
func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	require.NotEmpty(t, res.Content)
	block, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	return block.Text
}
