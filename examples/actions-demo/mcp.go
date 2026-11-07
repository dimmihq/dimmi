/*
|--------------------------------------------------------------------------
| MCP Server
|--------------------------------------------------------------------------
|
| The same Lovelace operations as the REST API, offered to agents as MCP
| tools over Streamable HTTP. Each tool says in its description what it
| does and what the policy allows, so a model can decide before calling.
| Read tools are marked read-only; refunds are marked destructive so a
| client knows to ask a person first.
|
| A tool answers with structured JSON and a one-sentence summary. A
| refusal comes back as a tool error carrying the policy's reason, which
| the agent can pass on to the customer.
|
*/

package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

/*
customerInput names the customer a tool acts on.
*/
type customerInput struct {
	CustomerID string `json:"customer_id" jsonschema:"the customer's external ID in Lovelace, for example user_42"`
}

func (in customerInput) customer() string { return in.CustomerID }

func (customerInput) idempotencyKey() string { return "" }

/*
orderInput names an order and the customer who should own it.
*/
type orderInput struct {
	CustomerID string `json:"customer_id" jsonschema:"the customer's external ID in Lovelace, for example user_42"`
	OrderID    string `json:"order_id"    jsonschema:"the order ID, for example ord_5001"`
}

func (in orderInput) customer() string { return in.CustomerID }

func (orderInput) idempotencyKey() string { return "" }

/*
refundInput asks for part or all of an invoice back.
*/
type refundInput struct {
	CustomerID     string  `json:"customer_id"               jsonschema:"the customer's external ID in Lovelace"`
	InvoiceID      string  `json:"invoice_id"                jsonschema:"the invoice to refund, from list_invoices"`
	Amount         float64 `json:"amount"                    jsonschema:"US dollars to refund; at most what is left"`
	Reason         string  `json:"reason"                    jsonschema:"why the customer wants the refund"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"a retry with the same key never refunds twice"`
}

func (in refundInput) customer() string { return in.CustomerID }

func (in refundInput) idempotencyKey() string { return in.IdempotencyKey }

/*
planInput names the plan to move a customer to.
*/
type planInput struct {
	CustomerID string `json:"customer_id" jsonschema:"the customer's external ID in Lovelace"`
	Plan       string `json:"plan"        jsonschema:"the plan to move to"`
}

func (in planInput) customer() string { return in.CustomerID }

func (planInput) idempotencyKey() string { return "" }

/*
scoped is a tool input that names its customer, and maybe the
idempotency key, for the journal.
*/
type scoped interface {
	customer() string
	idempotencyKey() string
}

/*
toolFunc runs a tool: given its input and idempotency key it returns the
structured result and its one-sentence summary.
*/
type toolFunc[In scoped, Out any] func(in In, key string) (Out, string, error)

/*
newMCPHandler returns the Streamable HTTP handler for the Lovelace tools,
guarded by Token.
*/
func newMCPHandler(st *store, j *journal, token string) (http.Handler, error) {
	srv, err := newMCPServer(st, j)
	if err != nil {
		return nil, err
	}

	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", stream)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authorized(r, token) {
			mux.ServeHTTP(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Bearer realm="lovelace-mcp"`)
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "send Authorization: Bearer <token>"))
		j.record(entry{
			channel: "MCP", action: r.Method + " " + r.URL.Path,
			outcome: "401 rejected: missing or wrong bearer token",
		})
	}), nil
}

/*
newMCPServer registers every Lovelace tool on a new MCP server.
*/
func newMCPServer(st *store, j *journal) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "lovelace", Title: "Lovelace billing", Version: "1.0.0"}, nil)

	planSchema, err := jsonschema.For[planInput](nil)
	if err != nil {
		return nil, fmt.Errorf("infer change_plan schema: %w", err)
	}
	planSchema.Properties["plan"].Enum = []any{planFree, planPro, planBusiness}

	t := tools{store: st}
	addTool(srv, j, readTool("get_subscription", describeGetSubscription), t.getSubscription)
	addTool(srv, j, readTool("list_invoices", describeListInvoices), t.listInvoices)
	addTool(srv, j, readTool("get_order", describeGetOrder), t.getOrder)
	addTool(srv, j, refundTool(), t.issueRefund)
	addTool(srv, j, planTool(planSchema), t.changePlan)
	addTool(srv, j, resetTool(), t.sendPasswordReset)

	return srv, nil
}

/*
addTool registers a tool whose every call is journaled. The idempotency
key comes from the input, or else from the Idempotency-Key header of the
HTTP request that carried the call.
*/
func addTool[In scoped, Out any](srv *mcp.Server, j *journal, tool *mcp.Tool, run toolFunc[In, Out]) {
	handle := func(_ context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		key := in.idempotencyKey()
		if key == "" && req.Extra != nil && req.Extra.Header != nil {
			key = req.Extra.Header.Get("Idempotency-Key")
		}

		call := entry{channel: "MCP", action: "tool " + tool.Name, customer: in.customer(), key: key}

		out, summary, err := run(in, key)
		if err != nil {
			call.outcome = "error: " + err.Error()
			j.record(call)

			var zero Out

			return nil, zero, err
		}

		call.outcome = summary
		j.record(call)

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, out, nil
	}

	mcp.AddTool(srv, tool, handle)
}

func readTool(name, description string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(false)},
	}
}

func refundTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "issue_refund",
		Description: "Refund part or all of a paid invoice to the customer's card. Policy: only invoices for " +
			"annual plans, only within 30 days of payment, and never more than the invoice's unrefunded amount; " +
			"monthly plans are never refunded. Money moves at once and cannot be taken back, so confirm with a " +
			"person first. Pass an idempotency_key so a retry can never refund twice.",
		Annotations: &mcp.ToolAnnotations{Title: "Issue refund", DestructiveHint: new(true)},
	}
}

func planTool(schema *jsonschema.Schema) *mcp.Tool {
	return &mcp.Tool{
		Name: "change_plan",
		Description: "Move a customer to another plan (free, pro or business) at once, keeping their seats " +
			"and billing interval. The new price applies from the next renewal. Moving to the current plan " +
			"changes nothing.",
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{Title: "Change plan", DestructiveHint: new(true), IdempotentHint: true},
	}
}

func resetTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "send_password_reset",
		Description: "Email the customer a link to reset their Lovelace password. " +
			"It does not change the password itself.",
		Annotations: &mcp.ToolAnnotations{Title: "Send password reset", DestructiveHint: new(false)},
	}
}
