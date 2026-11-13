/*
|--------------------------------------------------------------------------
| The API Quickstart
|--------------------------------------------------------------------------
|
| The longest page in the help center: a whole integration from the
| first key to a delivered parcel, the page developers keep open while
| they build. It is assembled from the reference pages' own operations
| so every step matches them.
|
*/

package stowline

import (
	"fmt"
	"strings"
)

func quickstartArticle() Article {
	var b strings.Builder
	b.WriteString(`This guide builds a complete shipping integration against the Stowline API: authenticate, verify the address, create a shipment, pick a rate, buy the label with an idempotency key, print it, follow the parcel with webhooks, handle returns and voids, and survive errors and rate limits. Every step links to its reference page for the full list of parameters.

It uses curl so you can follow along in a terminal, with the same calls in the Node.js and Python libraries at the end of each step. Use a test key while you build; nothing reaches a carrier and nothing is charged.

`)
	ops := map[string]Endpoint{}
	for _, e := range Endpoints() {
		ops[e.Title] = e
	}
	plan := []struct{ title, why string }{
		{"Verifying an address", "Bad addresses are the most common reason a label fails or a parcel comes back. Verify the address when the order arrives, and show the corrected version to whoever packs the order."},
		{"Creating a shipment", "A shipment holds both addresses, the parcel and, for international parcels, the customs declaration. Creating it returns rates from every connected carrier."},
		{"Customs declarations in the API", "Only for parcels that cross a border. Create the declaration once per kind of contents and reuse its id."},
		{"Buying a label", "Pick a rate and buy it. Always send an idempotency key, so a retry after a timeout returns the label you already bought instead of buying another."},
		{"Scheduling a pickup through the API", "Optional: book the carrier to collect, or drop the parcels off."},
		{"Creating an end-of-day manifest", "Optional: close the day's labels into one form the driver scans."},
		{"Registering a webhook endpoint", "Instead of polling every tracking number, register one endpoint and receive tracker.updated events."},
		{"Tracking a parcel", "Read the current status on demand, for example when a customer asks support."},
		{"Creating a return label", "Issue a return label that is only charged when it is used."},
		{"Voiding a label through the API", "Void labels you will not use and get the postage back."},
	}
	for i, step := range plan {
		e := ops[step.title]
		fmt.Fprintf(&b, "## Step %d: %s\n\n%s\n\n", i+1, strings.ToLower(step.title[:1])+step.title[1:], step.why)
		b.WriteString(code("http", e.Method+" https://api.stowline.test"+e.Path))
		if e.Request != "" {
			b.WriteString("\n" + code("json", e.Request))
		}
		b.WriteString("\nThe response:\n\n" + code("json", e.Response))
		if e.Notes != "" {
			b.WriteString("\n" + e.Notes + "\n")
		}
		b.WriteString("\nSee *" + e.Title + "* for every parameter.\n\n")
	}
	b.WriteString(`## Handling errors

Every error has the same shape: a stable code, a message for people and a request id. Branch on the code, log the request id, and show the message to your team.

` + code("json", `{"error": {"code": "ADDRESS_UNVERIFIED", "message": "The carrier could not match the recipient's address.", "field": "to_address.street1", "request_id": "req_7Hc2mQ9d"}}`) + `
Retry only 429 and 5xx responses, with exponential backoff and the same idempotency key. Everything else needs a change before it can succeed. The troubleshooting section has a page for every code.

## Going live

` + steps(
		"Create a live key with only the scopes your integration needs.",
		"Connect your carrier accounts in the dashboard; test mode used sample accounts.",
		"Register your production webhook endpoint and verify signatures.",
		"Buy one real label to yourself and void it, to check the whole path end to end.",
		"Watch Settings, Developers, Logs during the first day for unexpected errors.",
	) + `
## Checklist for a reliable integration

` + list(
		"Idempotency keys on every purchase, derived from your own order id so retries reuse them.",
		"Backoff on 429 and 5xx, honoring Retry-After.",
		"Webhook signature verification and replay protection.",
		"Out-of-order webhook handling based on occurred_at.",
		"Label files downloaded and stored if you need them for more than 90 days.",
		"Alerts when the balance runs low, using the account endpoint.",
		"A daily reconciliation that lists yesterday's shipments and compares them with your orders.",
	))
	return Article{
		Collection: "api", Title: "API quickstart: from first request to delivered parcel",
		Description: "Build a complete shipping integration step by step.", Body: b.String(),
	}
}
