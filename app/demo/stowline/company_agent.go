/*
|--------------------------------------------------------------------------
| The Agent's Setup
|--------------------------------------------------------------------------
|
| How Stowline's support lead set the AI agent up: the guidance it
| follows, the actions it can take on Stowline's API and the procedures
| it follows for the requests the team handles the same way every time.
|
*/

package stowline

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/support/actionresult"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
APIBaseURL is where the seeded actions send their requests. The host
does not exist; the cost simulation answers it in process.
*/
const APIBaseURL = "https://api.stowline.test/v1"

/*
AgentGuidance is what the support lead wrote for the agent.
*/
const AgentGuidance = `You answer customers of Stowline, shipping software for online stores.

- Search the help center before answering anything about Stowline. Quote the exact menu path, such as Settings, Carriers.
- Customers are store owners and warehouse staff, often in the middle of a busy shipping day. The first sentence of every reply to a problem is the first thing the customer should do. No apology, sympathy, cause or restating the problem before it. Say why it happened after the steps, in one line.
- Never promise a refund, a credit or the outcome of a carrier investigation. Explain the process and hand money questions to the Billing team.
- When a customer gives a tracking number, look the parcel up before answering.
- If the customer is an Enterprise account and labels cannot be bought at all, hand over at once.
- Someone who has not signed up yet and asks about plans or prices goes to Rio straight away. Never quote or work out a price for them yourself.`

/*
AgentActions are the actions the agent can take on Stowline's API.
*/
func AgentActions(conn ids.UUID) []agentactions.DefinitionInput {
	schema := func(v any) *json.RawMessage {
		b, _ := json.Marshal(v)
		raw := json.RawMessage(b)
		return &raw
	}
	fields := func(pairs ...string) *[]actionresult.ResponseField {
		var out []actionresult.ResponseField
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, actionresult.ResponseField{Name: pairs[i], Path: pairs[i+1]})
		}
		return &out
	}
	channels := &[]string{"widget", "email", "api", "slack"}
	return []agentactions.DefinitionInput{
		{
			ConnectionID: &conn, Name: new("get_shipment_status"), Title: new("Look up a parcel"),
			Description: new("Look up where a parcel is from its tracking number: status, estimated delivery and the latest carrier scan."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/trackers/{input.tracking_number}"),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"tracking_number"},
				"properties": map[string]any{"tracking_number": map[string]any{"type": "string", "minLength": 8, "maxLength": 40}},
			}),
			ResponseFields: fields("status", "status", "carrier", "carrier", "estimated_delivery", "est_delivery_date",
				"latest_scan", "events.0.message", "latest_scan_at", "events.0.occurred_at",
				"latest_location", "events.0.location"),
			Channels: channels, Agents: &[]string{"dimmi", "operator"},
		},
		{
			ConnectionID: &conn, Name: new("get_account_overview"), Title: new("Check the customer's account"),
			Description: new("Read the signed-in customer's plan, labels used this month, label allowance and Stowline balance."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/accounts/{contact.external_id}"),
			InputSchema:    schema(map[string]any{"type": "object", "properties": map[string]any{}}),
			ResponseFields: fields("plan", "plan", "labels_used", "labels_used", "labels_included", "labels_included", "balance", "balance"),
			Channels:       channels, Agents: &[]string{"dimmi", "operator"},
		},
		{
			ConnectionID: &conn, Name: new("list_recent_invoices"), Title: new("List recent invoices"),
			Description: new("List the signed-in customer's last invoices and balance top-ups with their amounts and status."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/accounts/{contact.external_id}/invoices"),
			InputSchema:    schema(map[string]any{"type": "object", "properties": map[string]any{}}),
			ResponseFields: fields("invoices", "data"),
			Channels:       channels,
		},
		{
			ConnectionID: &conn, Name: new("void_label"), Title: new("Void a label"),
			Description: new("Void one of the signed-in customer's unused labels by its shipment id, so the postage is refunded to their balance."),
			SideEffect:  new("write"), Confirmation: new("customer"), Method: new("POST"),
			PathTemplate: new("/shipments/{input.shipment_id}/void"),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"shipment_id"},
				"properties": map[string]any{"shipment_id": map[string]any{"type": "string", "pattern": "^shp_[A-Za-z0-9]{6,30}$"}},
			}),
			ResponseFields: fields("status", "status", "refund_status", "refund.status"),
			Channels:       channels,
		},
		{
			ConnectionID: &conn, Name: new("request_balance_refund"), Title: new("Refund balance to card"),
			Description: new("Ask for part of the signed-in customer's Stowline balance to be refunded to their card. A teammate approves it first."),
			SideEffect:  new("write"), Approval: new("teammate"), Method: new("POST"),
			PathTemplate: new("/accounts/{contact.external_id}/balance_refunds"),
			BodyTemplate: schema(map[string]any{"amount": "{input.amount}"}),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"amount"},
				"properties": map[string]any{"amount": map[string]any{"type": "number", "minimum": 1, "maximum": 2000}},
			}),
			ResponseFields: fields("refund_id", "id", "status", "status"),
			Channels:       channels,
		},
		{
			ConnectionID: &conn, Name: new("get_order"), Title: new("Look up an order"),
			Description: new("Look up an order by its number: the store it came from, its items, whether it has shipped and the tracking number."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/orders/{input.order_number}"),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"order_number"},
				"properties": map[string]any{"order_number": map[string]any{"type": "string", "pattern": "^ORD-[0-9]{4,8}$"}},
			}),
			ResponseFields: fields("store", "store", "status", "status", "items", "items", "tracking_number", "tracking_number"),
			Channels:       channels, Agents: &[]string{"dimmi", "operator"},
		},
		{
			ConnectionID: &conn, Name: new("change_plan"), Title: new("Change the plan"),
			Description: new("Move the signed-in customer's account to another plan from the next billing period. An admin approves it first."),
			SideEffect:  new("write"), Approval: new("admin"), Method: new("POST"),
			PathTemplate: new("/accounts/{contact.external_id}/plan"),
			BodyTemplate: schema(map[string]any{"plan": "{input.plan}"}),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"plan"},
				"properties": map[string]any{"plan": map[string]any{"type": "string", "enum": []string{"starter", "growth", "pro"}}},
			}),
			ResponseFields: fields("plan", "plan", "effective_from", "effective_from"),
			Channels:       channels, Agents: &[]string{"dimmi", "operator"},
		},
		{
			ConnectionID: &conn, Name: new("unlock_account"), Title: new("Unlock an account"),
			Description: new("Unlock a Stowline sign-in locked after too many failed attempts, by the account owner's email, and send them a reset link."),
			SideEffect:  new("write"), Method: new("POST"), PathTemplate: new("/logins/unlock"),
			BodyTemplate: schema(map[string]any{"email": "{input.email}"}),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"email"},
				"properties": map[string]any{"email": map[string]any{"type": "string", "format": "email", "maxLength": 200}},
			}),
			ResponseFields: fields("status", "status"),
			Channels:       channels, Agents: &[]string{"operator"},
		},
	}
}

/*
TestResponses are what each action answered when the team last tested
it, as the agent reads it: the fields the action keeps, by their names.
A preview or a simulation of a procedure answers the action with it, so
the agent has a real plan, balance or parcel to talk about.
*/
func TestResponses() map[string]map[string]any {
	return map[string]map[string]any{
		"get_shipment_status": {
			"status": "in_transit", "carrier": "ups", "estimated_delivery": "2026-11-15",
			"latest_scan": "Departed from facility", "latest_scan_at": "2026-11-12T23:41:00Z",
			"latest_location": "Louisville, KY",
		},
		"get_account_overview": {
			"plan": "growth", "labels_used": 1184, "labels_included": 1500, "balance": "212.40",
		},
		"list_recent_invoices": {"invoices": invoices()["data"]},
		"void_label":           {"status": "void_pending", "refund_status": "submitted"},
		"request_balance_refund": {
			"refund_id": "bref_01JCK2", "status": "pending_review",
		},
		"get_order": {
			"store": "Shopify", "status": "shipped", "tracking_number": "1ZX4F1E40312345678",
			"items": []map[string]any{{"sku": "MUG-BLU", "quantity": 2}, {"sku": "TEA-50", "quantity": 1}},
		},
		"change_plan":    {"plan": "pro", "effective_from": "2026-12-01"},
		"unlock_account": {"status": "unlocked"},
	}
}

/*
NamedProcedure is one procedure: its definition as it stands, the topic
whose conversations follow it, and how its first step read before the
team reworded it.
*/
type NamedProcedure struct {
	Name       string
	Topic      string
	FirstDraft string
	Definition procedure.Definition
}

/*
chip is an action as a step's words name it, the way typing @ in the
editor writes it.
*/
func chip(action string) string {
	return "[[action:" + action + "]]"
}

/*
Procedures are the requests the team handles the same way every time,
taking the published actions by their public ids.
*/
func Procedures(actions map[string]string) []NamedProcedure {
	return []NamedProcedure{
		{"Parcel not moving", "missing_tracking", "Ask the customer for their tracking number.", procedure.Definition{
			Description: "The customer says one parcel's tracking has not changed, or asks where a parcel is. " +
				"Not for a batch, an import or a print job that is stuck.",
			Steps: []procedure.Step{
				{ID: "ask", Kind: procedure.KindInstruction, Text: "Ask for the tracking number if the customer has not given it."},
				{ID: "lookup", Kind: procedure.KindInstruction, Text: "Look the parcel up with " + chip(actions["get_shipment_status"]) + "."},
				{ID: "wait", Kind: procedure.KindCondition, Text: "If the last scan is less than 5 business days old for a domestic parcel, or 10 for an international one, explain that gaps between scans are normal and when to check again."},
				{ID: "trace", Kind: procedure.KindInstruction, Text: "Otherwise tell the customer we will open a trace with the carrier, and hand the conversation to the team."},
			},
			Handover: []string{"The parcel is marked delivered but the customer says it never arrived.", "The customer wants to file an insurance claim."},
		}},
		{"Refund for a voided label", "void_refund", "Tell the customer voided labels are refunded to the Stowline balance within a few weeks.", procedure.Definition{
			Description: "The customer voided a label and asks where the money is.",
			Steps: []procedure.Step{
				{ID: "explain", Kind: procedure.KindInstruction, Text: "Explain that voided labels are refunded to the Stowline balance once the carrier confirms the label was never scanned: 14 to 21 days for USPS, within 7 days for most others."},
				{ID: "check", Kind: procedure.KindInstruction, Text: "Check the customer's balance with " + chip(actions["get_account_overview"]) + "."},
				{ID: "card", Kind: procedure.KindCondition, Text: "If the customer wants the money on their card instead, offer a balance refund for top-ups from the last 120 days."},
				{ID: "refund", Kind: procedure.KindInstruction, Text: "Request the balance refund with " + chip(actions["request_balance_refund"]) + " for the amount the customer agrees to."},
			},
			Handover: []string{"The void is more than 30 days old and still not refunded."},
		}},
		{"Customer wants to cancel", "cancel_account", "Ask why they are leaving.", procedure.Definition{
			Description: "The customer asks to cancel their plan, close their account or downgrade to Free.",
			Steps: []procedure.Step{
				{ID: "why", Kind: procedure.KindInstruction, Text: "Say you are sorry to see them go, and ask what made them decide to cancel."},
				{ID: "account", Kind: procedure.KindInstruction, Text: "Look up their plan and Stowline balance with " + chip(actions["get_account_overview"]) + "."},
				{ID: "explain", Kind: procedure.KindInstruction, Text: "Explain what canceling does, using their plan and balance: the account moves to Free at the end of the billing period, their labels, tracking pages and history stay, and their balance can be refunded to their card. Then ask if they still want to cancel."},
				{
					ID: "confirm", Kind: procedure.KindCondition, Text: "The customer still wants to cancel",
					ThenSteps: []procedure.Step{
						{ID: "handover", Kind: procedure.KindInstruction, Text: "Tell them the Billing team will cancel the plan and confirm by email, then hand the conversation to the Billing team with their reason and their balance."},
					},
					ElseSteps: []procedure.Step{
						{ID: "stay", Kind: procedure.KindInstruction, Text: "Thank them for staying, and offer to help with what made them want to leave."},
					},
				},
			},
			Handover: []string{
				"The customer asks for a refund of this month's plan fee.",
				"The customer is upset or mentions a chargeback.",
			},
		}},
	}
}
