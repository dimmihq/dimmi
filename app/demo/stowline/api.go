/*
|--------------------------------------------------------------------------
| Stowline's API, In Process
|--------------------------------------------------------------------------
|
| The seeded actions call Stowline's API, which does not exist. During a
| cost simulation the actions' requests are answered here instead, with
| the responses the real API would give, so the agent reads realistic
| results and its next call is the size it would really be. Anything
| else is refused, as an unknown route would be.
|
*/

package stowline

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

/*
StubAPI answers the seeded actions' API on the client, and refuses any
other request, as an unknown route would.
*/
func StubAPI(client *httpclient.Client) {
	ok := func(status int, v any) httpclient.Response {
		raw, _ := json.Marshal(v)
		return httpclient.Response{Status: status, Body: raw}
	}
	base := APIBaseURL
	httpclienttest.New(client).PreventStrayRequests().
		Stub("GET "+base+"/trackers/94*", ok(200, tracker("9400111899223344556677"))).
		Stub("GET "+base+"/trackers/*", ok(200, tracker("1ZX4F1E40312345678"))).
		Stub("GET "+base+"/accounts/*/invoices", ok(200, invoices())).
		Stub("GET "+base+"/accounts/*", ok(200, map[string]any{
			"plan": "growth", "labels_used": 1184,
			"labels_included": 1500, "balance": "212.40", "currency": "USD",
		})).
		Stub("POST "+base+"/shipments/*/void", ok(200, map[string]any{
			"status": "void_pending",
			"refund": map[string]any{"status": "submitted", "amount": "11.42", "expected_by": "2026-11-27"},
		})).
		Stub("POST "+base+"/accounts/*/balance_refunds", ok(201, map[string]any{
			"id":     "bref_01JCK2",
			"status": "pending_review",
		}))
}

/*
invoices are the account's last invoices as the API lists them.
*/
func invoices() map[string]any {
	return map[string]any{"data": []map[string]any{
		{
			"number": "INV-2026-1104", "period": "November 2026", "amount": "49.00", "status": "paid",
			"paid_at": "2026-11-03",
		},
		{
			"number": "INV-2026-1004", "period": "October 2026", "amount": "63.25", "status": "paid",
			"paid_at": "2026-10-03", "lines": []string{"Growth plan $49.00", "285 labels over the allowance $14.25"},
		},
		{
			"number": "INV-2026-0904", "period": "September 2026", "amount": "49.00", "status": "paid",
			"paid_at": "2026-09-03",
		},
	}}
}

/*
tracker is a parcel's tracking as the API reports it: a USPS label not
yet scanned, or a UPS parcel on its way.
*/
func tracker(number string) map[string]any {
	if number[:2] == "94" {
		return map[string]any{
			"tracking_number": number, "carrier": "usps", "status": "pre_transit",
			"est_delivery_date": nil, "events": []map[string]any{
				{
					"status": "pre_transit", "message": "Shipping label created, USPS awaiting item",
					"location": "Austin, TX", "occurred_at": "2026-11-06T16:02:00Z",
				},
			},
		}
	}
	return map[string]any{
		"tracking_number": number, "carrier": "ups", "status": "in_transit",
		"est_delivery_date": "2026-11-15", "events": []map[string]any{
			{
				"status": "in_transit", "message": "Departed from facility", "location": "Louisville, KY",
				"occurred_at": "2026-11-12T23:41:00Z",
			},
			{
				"status": "in_transit", "message": "Arrived at facility", "location": "Louisville, KY",
				"occurred_at": "2026-11-12T14:05:00Z",
			},
			{
				"status": "in_transit", "message": "Picked up", "location": "Brooklyn, NY",
				"occurred_at": "2026-11-11T19:20:00Z",
			},
		},
	}
}

/*
TeammateRequests are what Stowline's team asks Operator, the team's own
agent, in a working week.
*/
func TeammateRequests() []string {
	return []string{
		"How many conversations are open right now, and how many have nobody assigned?",
		"Summarize what customers wrote in about billing this week. What are the top three problems?",
		"Find open conversations from the last 7 days about voided labels and tag them voids.",
		"Read our help center articles about dimensional weight and draft a shorter, clearer version of the DHL Express section.",
		"Change the workflow that raises priority on handover so it also tags the conversation urgent.",
		"Which customers at enterprise companies wrote in more than twice this month, what about, and is there a help article that would have answered them?",
		"Something seems off with returns lately. Can you look into it?",
		"Draft a reply to the newest open conversation tagged tracking, using what the help center says about tracking gaps.",
		"How did customers rate us over the last 30 days, and which teammates got the low ratings?",
		"List the open conversations tagged urgent with how long each has waited, oldest first.",
		"Which teammate has the most open conversations?",
		"Find conversations that mention Evri. Do we have a feedback request for it?",
		"Which help center articles are about printing? List their titles.",
		"Tag every open conversation that mentions HS codes with international.",
		"What saved replies do we have for billing questions?",
		"Summarize the last conversation from an Enterprise customer.",
		"How many conversations did the AI resolve on its own last month?",
		"Assign the oldest unassigned billing conversation to the Billing team.",
		"Write a short internal note template for escalating a lost parcel to a carrier trace.",
		"What changed in our changelog in the last two months?",
		"List conversations in the Developers inbox waiting longer than a day.",
	}
}
