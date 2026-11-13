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
- Customers are store owners and warehouse staff, often in the middle of a busy shipping day. Lead with the fix, then explain.
- Never promise a refund, a credit or the outcome of a carrier investigation. Explain the process and hand money questions to the Billing team.
- When a customer gives a tracking number, look the parcel up before answering.
- If the customer is an Enterprise account and labels cannot be bought at all, hand over at once.`

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
	channels := &[]string{"widget", "email", "api"}
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
				"latest_scan", "events.0.message", "latest_location", "events.0.location"),
			Channels: channels,
		},
		{
			ConnectionID: &conn, Name: new("get_account_overview"), Title: new("Check the customer's account"),
			Description: new("Read the signed-in customer's plan, labels used this month, label allowance and Stowline balance."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/accounts/{contact.external_id}"),
			InputSchema:    schema(map[string]any{"type": "object", "properties": map[string]any{}}),
			ResponseFields: fields("plan", "plan", "labels_used", "labels_used", "labels_included", "labels_included", "balance", "balance"),
			Channels:       channels,
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
	}
}

/*
NamedProcedure is one procedure and its definition.
*/
type NamedProcedure struct {
	Name       string
	Definition procedure.Definition
}

/*
Procedures are the requests the team handles the same way every time,
taking the published actions by their public ids.
*/
func Procedures(actions map[string]string) []NamedProcedure {
	return []NamedProcedure{
		{"Parcel not moving", procedure.Definition{
			Description: "The customer says a parcel's tracking has not changed, or asks where a parcel is.",
			Steps: []procedure.Step{
				{ID: "ask", Kind: procedure.KindInstruction, Text: "Ask for the tracking number if the customer has not given it."},
				{ID: "lookup", Kind: procedure.KindAction, Text: "Look the parcel up.", ActionID: actions["get_shipment_status"]},
				{ID: "wait", Kind: procedure.KindCondition, Text: "If the last scan is less than 5 business days old for a domestic parcel, or 10 for an international one, explain that gaps between scans are normal and when to check again."},
				{ID: "trace", Kind: procedure.KindInstruction, Text: "Otherwise tell the customer we will open a trace with the carrier, and hand the conversation to the team."},
			},
			Handover: []string{"The parcel is marked delivered but the customer says it never arrived.", "The customer wants to file an insurance claim."},
		}},
		{"Refund for a voided label", procedure.Definition{
			Description: "The customer voided a label and asks where the money is.",
			Steps: []procedure.Step{
				{ID: "explain", Kind: procedure.KindInstruction, Text: "Explain that voided labels are refunded to the Stowline balance once the carrier confirms the label was never scanned: 14 to 21 days for USPS, within 7 days for most others."},
				{ID: "check", Kind: procedure.KindAction, Text: "Check the customer's balance.", ActionID: actions["get_account_overview"]},
				{ID: "card", Kind: procedure.KindCondition, Text: "If the customer wants the money on their card instead, offer a balance refund for top-ups from the last 120 days."},
				{ID: "refund", Kind: procedure.KindAction, Text: "Request the balance refund for the amount the customer agrees to.", ActionID: actions["request_balance_refund"]},
			},
			Handover: []string{"The void is more than 30 days old and still not refunded."},
		}},
		{"Customer wants to cancel", procedure.Definition{
			Description: "The customer asks to cancel their plan or close their account.",
			Steps: []procedure.Step{
				{ID: "why", Kind: procedure.KindInstruction, Text: "Ask, once and kindly, what made them decide to leave."},
				{ID: "how", Kind: procedure.KindInstruction, Text: "Explain that canceling moves the account to Free at the end of the period and keeps its history, and that the balance can be refunded to the card."},
				{ID: "hand", Kind: procedure.KindInstruction, Text: "Hand the conversation to the Billing team with the reason."},
			},
			Handover: []string{"Always, after explaining."},
		}},
	}
}
