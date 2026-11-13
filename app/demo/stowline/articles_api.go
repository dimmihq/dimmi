/*
|--------------------------------------------------------------------------
| Article Families: API Reference and Error Codes
|--------------------------------------------------------------------------
|
| One reference page per API operation, with its parameters, a request,
| a response and its errors, the longest and most technical pages of
| the help center; four guides every integration needs; and one page per
| error code saying what it means, why it happens and how to fix it.
|
*/

package stowline

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

/*
Endpoints returns the API operations the reference documents.
*/
func Endpoints() []Endpoint {
	addr := `{"name": "Ada Okafor", "street1": "417 Montgomery St", "city": "San Francisco", "state": "CA", "postal_code": "94104", "country": "US"}`
	parcel := `{"length": 10, "width": 8, "height": 4, "distance_unit": "in", "weight": 32, "mass_unit": "oz"}`
	shipment := `{
  "id": "shp_01JC8Z4M2Q",
  "status": "purchased",
  "to_address": ` + addr + `,
  "parcel": ` + parcel + `,
  "selected_rate": {"id": "rate_01JC8Z5A", "carrier": "ups", "service": "UPS Ground", "amount": "11.42", "currency": "USD", "delivery_days": 3},
  "label": {"id": "lbl_01JC8Z5K", "tracking_number": "1Z999AA10123456784", "label_url": "https://files.stowline.test/labels/lbl_01JC8Z5K.pdf", "format": "pdf_4x6"},
  "created_at": "2026-10-02T14:21:07Z"
}`
	return []Endpoint{
		{
			Method: "POST", Path: "/v1/shipments", Title: "Creating a shipment",
			Summary: "Create a shipment from addresses and a parcel and get its rates in one call.",
			Params: [][3]string{
				{"from_address", "object or id", "Where the parcel ships from. Pass a warehouse id or a full address."},
				{"to_address", "object", "The recipient. Must include a name, street, city, postal code and country."},
				{"parcel", "object", "Dimensions and weight. Weight is required; dimensions are needed for dimensional weight pricing."},
				{"customs", "object", "Required for international shipments. See *Customs declarations in the API*."},
				{"options", "object", "Signature, insurance, Saturday delivery, label format and reference fields."},
				{"carrier_accounts", "array of ids", "Limit rating to these carrier connections. Defaults to all."},
			},
			Request:  `{"from_address": "wh_01HZX3", "to_address": ` + addr + `, "parcel": ` + parcel + `, "options": {"label_format": "pdf_4x6", "reference": "order 1042"}}`,
			Response: strings.Replace(shipment, `"status": "purchased"`, `"status": "rated"`, 1),
			Errors:   []string{"ADDRESS_UNVERIFIED", "PARCEL_INVALID", "CUSTOMS_REQUIRED", "CARRIER_UNAVAILABLE"},
			Notes:    "A shipment is free to create. Nothing is charged until a label is bought. Unbought shipments are deleted after 30 days.",
		},
		{
			Method: "POST", Path: "/v1/shipments/{id}/buy", Title: "Buying a label",
			Summary: "Buy the label for one of a shipment's rates.",
			Params: [][3]string{
				{"rate_id", "string", "The rate to buy, from the shipment's rates."},
				{"insurance", "string", "Optional amount to insure, such as \"150.00\"."},
			},
			Request:  `{"rate_id": "rate_01JC8Z5A", "insurance": "150.00"}`,
			Response: shipment,
			Errors:   []string{"RATE_EXPIRED", "INSUFFICIENT_BALANCE", "CARRIER_AUTH_FAILED", "LABEL_LIMIT_REACHED"},
			Notes:    "Send an Idempotency-Key header. If the request times out, repeating it with the same key returns the label already bought instead of buying a second one.",
		},
		{
			Method: "GET", Path: "/v1/shipments/{id}", Title: "Retrieving a shipment",
			Summary: "Get a shipment with its rates, label and tracking.",
			Params:  [][3]string{{"id", "string", "The shipment's id, starting with shp_."}, {"expand", "array", "Pass tracker to include the full tracking history."}},
			Request: "", Response: shipment, Errors: []string{"NOT_FOUND"},
			Notes: "Shipments are kept for two years. Label files are kept for 90 days; download them if you need them longer.",
		},
		{
			Method: "GET", Path: "/v1/shipments", Title: "Listing shipments",
			Summary: "List shipments, newest first, filtered by date, status or reference.",
			Params: [][3]string{
				{"created_after, created_before", "timestamp", "Filter by creation time, in RFC 3339."},
				{"status", "string", "rated, purchased, voided or delivered."},
				{"reference", "string", "Exact match on the reference you set."},
				{"limit", "integer", "Up to 100, default 20."},
				{"cursor", "string", "From the previous page's next_cursor."},
			},
			Request: "", Response: `{"data": [` + shipment + `], "next_cursor": "c_eyJpZCI6InNocF8wMUpDOFo0TTJRIn0"}`,
			Errors: []string{"INVALID_CURSOR"},
			Notes:  "Use cursors, not offsets. A cursor stays valid for 24 hours. See *Pagination in the API*.",
		},
		{
			Method: "POST", Path: "/v1/shipments/{id}/void", Title: "Voiding a label through the API",
			Summary: "Void a label you will not use and get the postage back.",
			Params:  [][3]string{{"id", "string", "The shipment whose label to void."}},
			Request: "", Response: `{"id": "shp_01JC8Z4M2Q", "status": "void_pending", "refund": {"status": "submitted", "amount": "11.42"}}`,
			Errors: []string{"LABEL_ALREADY_VOIDED", "LABEL_SCANNED", "VOID_WINDOW_CLOSED"},
			Notes:  "Refund status moves from submitted to refunded when the carrier confirms. For USPS this takes 14 to 21 days. Listen for the refund.updated webhook.",
		},
		{
			Method: "POST", Path: "/v1/rates", Title: "Getting rates without a shipment",
			Summary: "Quote rates for a parcel, for example to show shipping prices at checkout.",
			Params: [][3]string{
				{"from_postal_code, to_postal_code", "string", "Origin and destination postal codes."},
				{"to_country", "string", "Two-letter code."},
				{"parcel", "object", "Weight and dimensions."},
				{"services", "array", "Limit to these services."},
			},
			Request:  `{"from_postal_code": "94104", "to_postal_code": "10001", "to_country": "US", "parcel": ` + parcel + `}`,
			Response: `{"rates": [{"carrier": "usps", "service": "Ground Advantage", "amount": "8.71", "delivery_days": 4}, {"carrier": "ups", "service": "UPS Ground", "amount": "11.42", "delivery_days": 3}]}`,
			Errors:   []string{"PARCEL_INVALID", "RATE_LIMITED"},
			Notes:    "Quotes are estimates without the full address; residential and remote area surcharges are only known once you create a shipment. Cache quotes for the same route and parcel for up to an hour.",
		},
		{
			Method: "POST", Path: "/v1/addresses/verify", Title: "Verifying an address",
			Summary:  "Check an address against carrier databases and get a corrected version.",
			Params:   [][3]string{{"address", "object", "The address to check."}, {"carrier", "string", "Verify with this carrier's database. Defaults to USPS for US addresses."}},
			Request:  `{"address": {"street1": "417 montgomery", "city": "san francisco", "state": "CA", "postal_code": "94104", "country": "US"}}`,
			Response: `{"verified": true, "residential": false, "address": {"street1": "417 MONTGOMERY ST", "city": "SAN FRANCISCO", "state": "CA", "postal_code": "94104-1129", "country": "US"}, "messages": ["Street suffix added"]}`,
			Errors:   []string{"ADDRESS_UNVERIFIED"},
			Notes:    "Verification is free and does not count toward your label allowance. It is rate limited to 10 requests a second.",
		},
		{
			Method: "POST", Path: "/v1/returns", Title: "Creating a return label",
			Summary:  "Create a prepaid return label for an earlier shipment.",
			Params:   [][3]string{{"shipment_id", "string", "The original shipment."}, {"reason", "string", "Free text shown in the returns list."}, {"qr_code", "boolean", "Return a QR code instead of a printable label where the carrier supports it."}},
			Request:  `{"shipment_id": "shp_01JC8Z4M2Q", "reason": "Wrong size", "qr_code": true}`,
			Response: `{"id": "ret_01JCA1", "status": "label_created", "label": {"tracking_number": "9202090153540000000001", "qr_code_url": "https://files.stowline.test/qr/ret_01JCA1.png"}, "charge": "on_scan"}`,
			Errors:   []string{"RETURN_NOT_ALLOWED", "CARRIER_UNAVAILABLE"},
			Notes:    "Return labels are charged only when the carrier first scans them.",
		},
		{
			Method: "GET", Path: "/v1/trackers/{tracking_number}", Title: "Tracking a parcel",
			Summary:  "Get the current status and full history of a parcel.",
			Params:   [][3]string{{"tracking_number", "string", "The carrier's tracking number."}, {"carrier", "string", "Needed only for tracking numbers Stowline did not create."}},
			Request:  "",
			Response: `{"tracking_number": "1Z999AA10123456784", "carrier": "ups", "status": "in_transit", "est_delivery_date": "2026-10-05", "events": [{"status": "in_transit", "message": "Departed from facility", "location": "Oakland, CA", "occurred_at": "2026-10-03T02:14:00Z"}]}`,
			Errors:   []string{"TRACKER_NOT_FOUND"},
			Notes:    "Tracking numbers not created through Stowline can be tracked too and count as one tracker each against your plan.",
		},
		{
			Method: "POST", Path: "/v1/pickups", Title: "Scheduling a pickup through the API",
			Summary:  "Book a carrier pickup for labels already bought.",
			Params:   [][3]string{{"carrier_account", "string", "The carrier connection."}, {"warehouse", "string", "Where to collect."}, {"ready_at, close_at", "timestamp", "The pickup window."}, {"shipments", "array", "Shipment ids to collect."}},
			Request:  `{"carrier_account": "ca_ups_01", "warehouse": "wh_01HZX3", "ready_at": "2026-10-03T13:00:00-07:00", "close_at": "2026-10-03T17:00:00-07:00", "shipments": ["shp_01JC8Z4M2Q"]}`,
			Response: `{"id": "pku_01JCB", "status": "scheduled", "confirmation": "2929602E9CP", "fee": "0.00"}`,
			Errors:   []string{"PICKUP_WINDOW_INVALID", "CARRIER_UNAVAILABLE"},
			Notes:    "Each carrier has its own booking cut-off; see the carrier's pickup article.",
		},
		{
			Method: "POST", Path: "/v1/manifests", Title: "Creating an end-of-day manifest",
			Summary:  "Close labels into a manifest for the driver to scan.",
			Params:   [][3]string{{"carrier_account", "string", "The carrier connection."}, {"shipments", "array", "Optional. Defaults to every unmanifested label for the account."}},
			Request:  `{"carrier_account": "ca_usps_01"}`,
			Response: `{"id": "man_01JCC", "status": "closed", "form_url": "https://files.stowline.test/manifests/man_01JCC.pdf", "shipment_count": 48}`,
			Errors:   []string{"NOTHING_TO_MANIFEST"},
			Notes:    "Manifests close automatically at 6:00 p.m. warehouse time; create one through the API only when you need it earlier.",
		},
		{
			Method: "POST", Path: "/v1/batches", Title: "Buying labels in a batch through the API",
			Summary:  "Buy up to 1,000 labels in one asynchronous request.",
			Params:   [][3]string{{"shipments", "array", "Shipment ids, each with a rate_id or a service to pick."}, {"label_format", "string", "Format of the combined label file."}},
			Request:  `{"shipments": [{"id": "shp_01JC8Z4M2Q", "service": "UPS Ground"}], "label_format": "pdf_4x6"}`,
			Response: `{"id": "bat_01JCD", "status": "creating", "counts": {"total": 1, "purchased": 0, "failed": 0}}`,
			Errors:   []string{"BATCH_TOO_LARGE"},
			Notes:    "Poll the batch or listen for batch.updated. Failed shipments carry their own error and can be retried.",
		},
		{
			Method: "POST", Path: "/v1/webhooks", Title: "Registering a webhook endpoint",
			Summary:  "Receive events such as tracker.updated at your own URL.",
			Params:   [][3]string{{"url", "string", "An HTTPS URL that answers 2xx within 10 seconds."}, {"events", "array", "Events to send. Use [\"*\"] for all."}},
			Request:  `{"url": "https://hooks.example.com/stowline", "events": ["tracker.updated", "refund.updated"]}`,
			Response: `{"id": "whk_01JCE", "url": "https://hooks.example.com/stowline", "secret": "whsec_4f0d1c...", "events": ["tracker.updated", "refund.updated"]}`,
			Errors:   []string{"WEBHOOK_URL_INVALID"},
			Notes:    "The secret is shown once. Use it to check each delivery's Stowline-Signature header; see *Verifying webhook signatures*.",
		},
		{
			Method: "GET", Path: "/v1/account", Title: "Reading your account and balance",
			Summary: "Get the account's plan, label usage and balance.",
			Params:  nil, Request: "",
			Response: `{"id": "acct_01HZ", "plan": "pro", "labels_used": 4213, "labels_included": 7500, "balance": "312.40", "currency": "USD"}`,
			Errors:   nil, Notes: "Useful for alerting before the balance runs out. Requires the account:read scope.",
		},
		{
			Method: "POST", Path: "/v1/customs", Title: "Customs declarations in the API",
			Summary: "Attach the items and declarations an international shipment needs.",
			Params: [][3]string{
				{"contents_type", "string", "merchandise, gift, documents, returned_goods or sample."},
				{"items", "array", "Each with description, quantity, value, weight, hs_code and origin_country."},
				{"incoterm", "string", "DAP (default) or DDP."},
				{"eel_pfc", "string", "US export exemption code, such as NOEEI 30.37(a), for exports under $2,500."},
			},
			Request:  `{"contents_type": "merchandise", "incoterm": "DAP", "eel_pfc": "NOEEI 30.37(a)", "items": [{"description": "Cotton t-shirt", "quantity": 2, "value": "24.00", "weight": 8, "hs_code": "610910", "origin_country": "PT"}]}`,
			Response: `{"id": "cst_01JCF", "items": 1, "total_value": "48.00"}`,
			Errors:   []string{"CUSTOMS_REQUIRED", "HS_CODE_INVALID"},
			Notes:    "Pass the returned id as customs on the shipment. One declaration can be reused for many shipments with the same contents.",
		},
		{
			Method: "GET", Path: "/v1/webhooks", Title: "Listing webhook endpoints",
			Summary: "List your webhook endpoints and their delivery health.",
			Params:  nil, Request: "",
			Response: `{"data": [{"id": "whk_01JCE", "url": "https://hooks.example.com/stowline", "events": ["tracker.updated"], "status": "enabled", "failing_since": null}]}`,
			Errors:   nil, Notes: "An endpoint failing for three days is disabled and shows status disabled with the time it started failing.",
		},
		{
			Method: "DELETE", Path: "/v1/webhooks/{id}", Title: "Deleting a webhook endpoint",
			Summary: "Stop sending events to an endpoint.",
			Params:  [][3]string{{"id", "string", "The endpoint's id, starting with whk_."}}, Request: "",
			Response: `{"id": "whk_01JCE", "deleted": true}`, Errors: []string{"NOT_FOUND"},
			Notes: "Deliveries already in flight may still arrive for a few seconds after deletion.",
		},
		{
			Method: "GET", Path: "/v1/carrier_accounts", Title: "Listing carrier accounts",
			Summary: "List the carrier connections on your account and their status.",
			Params:  nil, Request: "",
			Response: `{"data": [{"id": "ca_ups_01", "carrier": "ups", "description": "UPS main", "status": "active", "warehouses": ["wh_01HZX3"]}, {"id": "ca_usps_stowline", "carrier": "usps", "description": "Stowline USPS rates", "status": "active", "warehouses": []}]}`,
			Errors:   nil, Notes: "Pass these ids as carrier_accounts when creating a shipment to limit rating to some accounts.",
		},
		{
			Method: "GET", Path: "/v1/batches/{id}", Title: "Checking a batch",
			Summary: "Get a batch's progress and the error of each failed shipment.",
			Params:  [][3]string{{"id", "string", "The batch's id, starting with bat_."}}, Request: "",
			Response: `{"id": "bat_01JCD", "status": "completed", "counts": {"total": 250, "purchased": 247, "failed": 3}, "label_url": "https://files.stowline.test/batches/bat_01JCD.pdf", "failures": [{"shipment": "shp_01JCG", "code": "ADDRESS_UNVERIFIED"}]}`,
			Errors:   []string{"NOT_FOUND"}, Notes: "The combined label file only includes purchased labels, in the order the shipments were sent.",
		},
		{
			Method: "POST", Path: "/v1/orders", Title: "Creating an order through the API",
			Summary: "Send an order from a system Stowline does not connect to, so it appears on the Orders page.",
			Params: [][3]string{
				{"order_number", "string", "Your reference; unique per account."},
				{"to_address", "object", "The recipient."},
				{"items", "array", "SKU, quantity, value and weight per item."},
				{"requested_service", "string", "What the customer chose at checkout, for automation rules."},
			},
			Request:  `{"order_number": "WEB-88213", "to_address": ` + addr + `, "items": [{"sku": "TEE-BLK-M", "quantity": 1, "value": "24.00", "weight": 8}], "requested_service": "Standard"}`,
			Response: `{"id": "ord_01JCH", "order_number": "WEB-88213", "status": "awaiting_shipment", "rules_applied": ["Domestic under 1 lb"]}`,
			Errors:   []string{"DUPLICATE_ORDER", "ADDRESS_UNVERIFIED"},
			Notes:    "Automation rules run on orders created through the API exactly as on imported ones.",
		},
		{
			Method: "POST", Path: "/v1/insurance", Title: "Insuring a shipment after the label is bought",
			Summary:  "Add Stowline insurance to a label you already bought.",
			Params:   [][3]string{{"shipment_id", "string", "The shipment to insure."}, {"amount", "string", "The value to insure, up to 5000.00."}},
			Request:  `{"shipment_id": "shp_01JC8Z4M2Q", "amount": "320.00"}`,
			Response: `{"id": "ins_01JCJ", "status": "active", "premium": "3.04", "amount": "320.00"}`,
			Errors:   []string{"INSUFFICIENT_BALANCE"}, Notes: "Insurance can be added until the parcel's first scan. The premium is charged to your balance.",
		},
	}
}

func endpointArticle(e Endpoint) Article {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", e.Summary)
	b.WriteString(code("http", e.Method+" https://api.stowline.test"+e.Path))
	b.WriteString("\nAuthenticate with your API key as a bearer token. See *Authenticating with the API*.\n\n")
	if len(e.Params) > 0 {
		b.WriteString("## Parameters\n\n")
		rows := make([][]string, 0, len(e.Params))
		for _, p := range e.Params {
			rows = append(rows, []string{"`" + p[0] + "`", p[1], p[2]})
		}
		b.WriteString(table([]string{"Name", "Type", "Description"}, rows))
		b.WriteString("\n")
	}
	if e.Request != "" {
		b.WriteString("## Example request\n\n")
		b.WriteString(code("bash", fmt.Sprintf("curl -X %s https://api.stowline.test%s \\\n  -H \"Authorization: Bearer $STOWLINE_API_KEY\" \\\n  -H \"Content-Type: application/json\" \\\n  -H \"Idempotency-Key: 6f1c2e0a-order-1042\" \\\n  -d '%s'",
			e.Method, strings.ReplaceAll(e.Path, "{id}", "shp_01JC8Z4M2Q"), e.Request)))
		b.WriteString("\n")
	} else {
		b.WriteString("## Example request\n\n")
		b.WriteString(code("bash", fmt.Sprintf("curl https://api.stowline.test%s \\\n  -H \"Authorization: Bearer $STOWLINE_API_KEY\"",
			strings.NewReplacer("{id}", "shp_01JC8Z4M2Q", "{tracking_number}", "1Z999AA10123456784").Replace(e.Path))))
		b.WriteString("\n")
	}
	b.WriteString(sdkSamples(e))
	b.WriteString("## Example response\n\n")
	b.WriteString(code("json", e.Response))
	b.WriteString("\n")
	b.WriteString(responseFields(e.Response))
	if len(e.Errors) > 0 {
		b.WriteString("## Errors\n\nBesides the errors every request can return (`UNAUTHORIZED`, `FORBIDDEN`, `RATE_LIMITED`, `INTERNAL`), this operation can return:\n\n")
		for _, code := range e.Errors {
			if ec, ok := errorCodeByName(code); ok {
				fmt.Fprintf(&b, "- `%s` (%d): %s\n", ec.Code, ec.Status, ec.Meaning)
			} else {
				fmt.Fprintf(&b, "- `%s`: see the error's message for details.\n", code)
			}
		}
		b.WriteString("\n")
	}
	if e.Notes != "" {
		b.WriteString("## Notes\n\n" + e.Notes + "\n\n")
	}
	b.WriteString("## Test mode\n\nWith a test key (`sk_test_...`) this operation behaves exactly the same but nothing is sent to a carrier and nothing is charged. Test labels carry the word SAMPLE across them.\n")
	return Article{Collection: "api", Title: e.Title, Description: e.Summary, Body: b.String()}
}

func apiGuideArticles() []Article {
	return []Article{
		{
			Collection: "api", Title: "Authenticating with the API", Description: "API keys, bearer tokens and test mode.",
			Body: `Every API request is authenticated with an API key sent as a bearer token:

` + code("bash", `curl https://api.stowline.test/v1/account -H "Authorization: Bearer sk_live_51Hc..."`) + `
Create keys under **Settings → API keys**. Live keys start with *sk_live_* and buy real labels; test keys start with *sk_test_* and never reach a carrier.

## Errors

A missing or revoked key returns 401 with the code *UNAUTHORIZED*. A valid key without the scope an operation needs returns 403 with *FORBIDDEN* and names the missing scope in the message.

## Keeping keys safe

Never put a live key in a browser, a mobile app or a public repository. Keys in client code can be read by anyone and used to buy labels on your balance. Call the API from your server, and rotate keys at least once a year.

## API versions

The API is versioned by date. Your account is pinned to the version current when you made your first request, and you can send a *Stowline-Version* header to use another one. Breaking changes only ship in a new version; additive changes such as new fields and events ship to every version.`,
		},
		{
			Collection: "api", Title: "Rate limits and retries", Description: "How many requests you can make and how to handle 429.",
			Body: `The API allows 100 requests a second per account in live mode and 25 in test mode. Rating and address verification have their own limits of 20 and 10 a second, because each one calls carriers.

## When you hit the limit

Requests over the limit get a 429 response with the code *RATE_LIMITED* and a *Retry-After* header in seconds. Wait that long, then retry. Every response also carries:

` + table([]string{"Header", "Meaning"}, [][]string{
				{"X-RateLimit-Limit", "Requests allowed per second."},
				{"X-RateLimit-Remaining", "Requests left in the current second."},
				{"X-RateLimit-Reset", "When the window resets, as a Unix time."},
			}) + `
## Retrying safely

Retry 429, 500, 502, 503 and 504 responses with exponential backoff and jitter, for example after 1, 2, 4 and 8 seconds. Do not retry 4xx errors other than 429; they will fail the same way.

Requests that buy something must carry an **Idempotency-Key** header, so that a retry after a timeout never buys a second label. Keys are kept for 24 hours.

## Bulk work

For hundreds of labels, use batches instead of many single requests; a batch counts as one request. For exports, list shipments with a high limit instead of fetching them one by one.`,
		},
		{
			Collection: "api", Title: "Pagination in the API", Description: "Use cursors to page through long lists.",
			Body: `List operations return at most 100 items per page, newest first, with a cursor for the next page.

` + code("json", `{"data": [...], "next_cursor": "c_eyJpZCI6InNocF8wMUpD"}`) + `
Pass *next_cursor* as the *cursor* parameter to get the next page. When *next_cursor* is null, you have reached the end.

## Why cursors

Offsets skip or repeat items when new shipments are created while you page. A cursor marks the last item you saw, so every item is returned exactly once.

## Example in Python

` + code("python", `import requests

def all_shipments(key, **filters):
    cursor = None
    while True:
        params = {"limit": 100, **filters}
        if cursor:
            params["cursor"] = cursor
        page = requests.get("https://api.stowline.test/v1/shipments",
                            headers={"Authorization": f"Bearer {key}"}, params=params).json()
        yield from page["data"]
        cursor = page["next_cursor"]
        if not cursor:
            return`) + `
Cursors stay valid for 24 hours. A cursor that is too old returns *INVALID_CURSOR*; start again from the first page with a *created_after* filter.`,
		},
		{
			Collection: "api", Title: "Verifying webhook signatures", Description: "Check that a webhook really came from Stowline.",
			Body: `Every webhook delivery is signed with your endpoint's secret, so you can reject requests that did not come from Stowline.

## The header

` + code("text", `Stowline-Signature: t=1791234567,v1=5257a869e7ecebeda32affa62cdca3fa51cad7e77a0e56ff536d0ce8e108d8bd`) + `
*t* is the time the delivery was signed and *v1* is an HMAC-SHA256 of the string *t* + "." + the raw request body, keyed with your secret.

## Checking it

` + code("go", `func verify(secret, header string, body []byte) bool {
	var t, sig string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "t":
			t = v
		case "v1":
			sig = v
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t + "." + string(body)))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(sig))
}`) + `
Use the raw body exactly as received. Frameworks that parse JSON before you see it change spacing and break the signature.

## Replays

Reject deliveries whose *t* is more than five minutes old. Stowline retries failed deliveries for up to three days with backoff; each retry is signed again with a new time.

## Retries and failures

Answer with any 2xx status within 10 seconds. Anything else counts as a failure. After three days of failures the endpoint is disabled and the account owner gets an email.`,
		},
	}
}

/*
ErrorCodes returns the error codes the troubleshooting section explains.
*/
func ErrorCodes() []ErrorCode {
	return []ErrorCode{
		{
			"ADDRESS_UNVERIFIED", "The address could not be verified", "The carrier could not match the recipient's address to a deliverable address.",
			[]string{"A missing apartment, suite or unit number.", "A postal code that belongs to a neighboring town.", "A new-build street the carrier's database does not know yet."},
			[]string{"Use **Validate address** on the order and accept the suggestion.", "Confirm the address with the recipient.", "Turn on **Ship to unverified address** for the order if you are sure."},
			422,
		},
		{
			"RATE_EXPIRED", "The rate has expired", "The rate you tried to buy is more than 24 hours old.",
			[]string{"An order was rated yesterday and bought today.", "An integration cached rates for too long."},
			[]string{"Click **Get rates** again and buy the new rate.", "Through the API, create the shipment again or fetch fresh rates before buying."},
			409,
		},
		{
			"INSUFFICIENT_BALANCE", "Your balance is too low", "The label costs more than your Stowline balance and the automatic top-up failed.",
			[]string{"The card on file expired or was declined.", "Automatic top-ups are switched off."},
			[]string{"Update the card under **Billing → Payment method**.", "Top up manually under **Billing → Balance**.", "Raise the refill amount so batches do not run the balance down."},
			402,
		},
		{
			"CARRIER_AUTH_FAILED", "The carrier refused the account details", "The carrier no longer accepts the credentials saved on your connection.",
			[]string{"The carrier account was closed or suspended.", "The password or API key was changed in the carrier's portal.", "API access was switched off on the carrier's side."},
			[]string{"Reconnect the carrier under **Settings → Carriers**.", "Check with the carrier that the account is active and enabled for API access."},
			424,
		},
		{
			"LABEL_ALREADY_VOIDED", "This label is already voided", "The label was voided earlier, so it cannot be voided again.",
			[]string{"Someone on your team already voided it.", "An integration voided it automatically when the order was canceled."},
			[]string{"Check the label's history to see who voided it.", "Nothing else is needed; the refund is already on its way."},
			409,
		},
		{
			"LABEL_SCANNED", "The label has been scanned", "The carrier has scanned the label, so it can no longer be voided.",
			[]string{"The parcel was handed over or picked up.", "A carrier scanned a label by mistake while processing others."},
			[]string{"If the parcel was never sent, open a refund request from the label's page within 30 days."},
			409,
		},
		{
			"VOID_WINDOW_CLOSED", "It is too late to void this label", "The carrier's void window for this label has passed.",
			[]string{"USPS labels can be voided for 30 days; others until the first scan or the end of the day."},
			[]string{"Open a refund request from the label's page; the carrier decides whether to refund it."},
			409,
		},
		{
			"CUSTOMS_REQUIRED", "Customs information is missing", "The shipment crosses a border but has no customs declaration.",
			[]string{"The products have no description, value or HS code.", "The order was created by API without a customs object."},
			[]string{"Add the customs details on the order's **International** section.", "Save HS codes and origin on your products so they fill in automatically."},
			422,
		},
		{
			"HS_CODE_INVALID", "The HS code is not valid", "An item's tariff code is not a valid six-digit Harmonized System code.",
			[]string{"The code has fewer than six digits.", "A national ten-digit code was entered with dots or spaces."},
			[]string{"Use the first six digits of the code, with no dots or spaces.", "Look up the code in your country's tariff search tool."},
			422,
		},
		{
			"PARCEL_INVALID", "The parcel details are not valid", "The weight or dimensions are missing, zero or over the carrier's limits.",
			[]string{"The weight was left empty.", "Dimensions were entered in the wrong unit.", "The parcel is heavier than the service allows."},
			[]string{"Weigh the packed parcel and enter its weight.", "Check the units on the order.", "Pick a service with a higher weight limit."},
			422,
		},
		{
			"CARRIER_UNAVAILABLE", "The carrier is not responding", "The carrier's systems did not answer in time.",
			[]string{"An outage or slowdown at the carrier."},
			[]string{"Try again in a few minutes.", "Batches retry failed labels automatically for an hour.", "Check our status page for carrier incidents."},
			503,
		},
		{
			"LABEL_LIMIT_REACHED", "You have used this month's labels", "Your plan's monthly label allowance is used up and your plan does not allow overage.",
			[]string{"The Free plan stops at 50 labels a month."},
			[]string{"Upgrade under **Billing → Plan** to keep shipping straight away.", "Wait for the allowance to reset on your billing date."},
			402,
		},
		{
			"RETURN_NOT_ALLOWED", "This item cannot be returned", "Your return rules do not allow a return for this order or item.",
			[]string{"The return window has passed.", "The item is tagged as final sale."},
			[]string{"Change the rules under **Returns → Settings**, or create the return label by hand from the order."},
			422,
		},
		{
			"TRACKER_NOT_FOUND", "We cannot find that tracking number", "The carrier does not know the tracking number yet, or it is mistyped.",
			[]string{"The label was bought minutes ago and the carrier has not registered it.", "The tracking number belongs to another carrier."},
			[]string{"Wait an hour and try again.", "Pass the carrier explicitly."},
			404,
		},
		{
			"PICKUP_WINDOW_INVALID", "The pickup window is not available", "The carrier cannot collect in the window you asked for.",
			[]string{"The booking cut-off for that day has passed.", "The window is outside the warehouse's opening hours."},
			[]string{"Pick the next available day.", "Widen the window to at least two hours."},
			422,
		},
		{
			"NOTHING_TO_MANIFEST", "There are no labels to put on a manifest", "Every label for that carrier account is already on a manifest or voided.",
			[]string{"The manifest was already closed automatically."},
			[]string{"Print the existing manifest from **Labels → Manifests**."},
			409,
		},
		{
			"BATCH_TOO_LARGE", "The batch is too large", "A batch can hold at most 1,000 shipments, or 25 on the Free plan.",
			[]string{"Too many orders were selected."},
			[]string{"Split the orders into several batches."},
			422,
		},
		{
			"WEBHOOK_URL_INVALID", "The webhook URL is not accepted", "The URL is not HTTPS, does not resolve, or points to a private address.",
			[]string{"An http:// URL.", "A localhost or internal address."},
			[]string{"Use a public HTTPS URL. For local development, use a tunneling tool."},
			422,
		},
		{
			"INVALID_CURSOR", "The page cursor is not valid", "The cursor is malformed or more than 24 hours old.",
			[]string{"A cursor was stored and reused the next day."},
			[]string{"Start again from the first page, filtered by created_after."},
			400,
		},
		{
			"RATE_LIMITED", "Too many requests", "The account sent more requests than its rate limit allows.",
			[]string{"A loop without backoff.", "Many single requests where a batch would do."},
			[]string{"Wait for the number of seconds in Retry-After, then retry.", "Use batches for bulk work."},
			429,
		},
		{
			"DUPLICATE_ORDER", "This order already exists", "An order with the same number from the same store is already in Stowline.",
			[]string{"The store sent the order twice.", "The order was imported by CSV and by the integration."},
			[]string{"Open the existing order; nothing was charged twice.", "Remove the CSV row if the store is connected."},
			409,
		},
		{
			"INTEGRATION_TOKEN_EXPIRED", "The store connection has expired", "The store or marketplace no longer accepts Stowline's access token.",
			[]string{"eBay tokens expire after 18 months.", "Someone removed the Stowline app from the store."},
			[]string{"Reconnect the store under **Settings → Integrations**."},
			424,
		},
		{
			"SIGNATURE_UNAVAILABLE", "Signature confirmation is not available", "The service or destination does not offer the signature option you chose.",
			[]string{"Signature on a letter or flat service.", "Adult signature outside the United States."},
			[]string{"Pick another service, or remove the option."},
			422,
		},
		{
			"ACCOUNT_SUSPENDED", "The account is suspended", "Label buying is paused on the account.",
			[]string{"An unpaid invoice past its grace period.", "A carrier asked us to pause the account during an investigation."},
			[]string{"Pay any open invoice under **Billing**.", "Write to support; we will explain what is needed."},
			403,
		},
		{
			"PRINT_HELPER_OFFLINE", "The print helper is offline", "Labels could not be sent to your printer because the print helper is not running.",
			[]string{"The computer running the helper is asleep or off.", "The helper was signed out after an update."},
			[]string{"Open the helper on the computer and sign in again.", "Print from the browser in the meantime."},
			503,
		},
	}
}

/*
errorCodeByName finds an error code's page facts.
*/
func errorCodeByName(name string) (ErrorCode, bool) {
	for _, e := range ErrorCodes() {
		if e.Code == name {
			return e, true
		}
	}
	return ErrorCode{}, false
}

func errorArticle(e ErrorCode) Article {
	body := fmt.Sprintf(`You may see this error as **%[1]s** in the dashboard or as the code `+"`%[2]s`"+` with HTTP status %[3]d in the API.

## What it means

%[4]s

## Why it happens

%[5]s
## How to fix it

%[6]s
## In the API

The response body names the error and, where it applies, the field:

%[7]s
Handle this code in your integration rather than matching the message text; messages are written for people and can change, codes never do.

%[8]s

## Still stuck

Send us the order number or the request id from the *Request-Id* response header, and we can see exactly what happened.`,
		e.Title, e.Code, e.Status, e.Meaning, list(e.Causes...), steps(e.Fixes...),
		code("json", fmt.Sprintf(`{"error": {"code": %q, "message": %q, "request_id": "req_7Hc2mQ9d"}}`,
			e.Code, e.Meaning)),
		handling(e))
	return Article{
		Collection: "troubleshooting", Title: "Error " + e.Code + ": " + e.Title,
		Slug: "error-" + strings.ToLower(strings.ReplaceAll(e.Code, "_", "-")), Description: e.Meaning, Body: body,
	}
}

/*
sdkSamples shows the operation in each official library.
*/
func sdkSamples(e Endpoint) string {
	method := sdkMethod(e)
	args := "{ /* fields as above */ }"
	pyArgs := "**fields"
	if e.Request == "" {
		args, pyArgs = `"shp_01JC8Z4M2Q"`, `"shp_01JC8Z4M2Q"`
	}
	var b strings.Builder
	b.WriteString("## With the official libraries\n\nThe libraries add the idempotency key, retries and error types for you.\n\n")
	b.WriteString("**Node.js**\n\n")
	b.WriteString(code("javascript", fmt.Sprintf(`import Stowline from "@stowline/node";

const stowline = new Stowline(process.env.STOWLINE_API_KEY);

try {
  const result = await stowline.%s(%s);
  console.log(result.id);
} catch (err) {
  if (err instanceof Stowline.ApiError) {
    console.error(err.code, err.message, err.requestId);
  }
  throw err;
}`, method, args)))
	b.WriteString("\n**Python**\n\n")
	b.WriteString(code("python", fmt.Sprintf(`import os
import stowline

client = stowline.Client(os.environ["STOWLINE_API_KEY"])

try:
    result = client.%s(%s)
    print(result.id)
except stowline.ApiError as err:
    print(err.code, err.message, err.request_id)
    raise`, snake(method), pyArgs)))
	b.WriteString("\n**Go**\n\n")
	b.WriteString(code("go", fmt.Sprintf(`client := stowline.New(os.Getenv("STOWLINE_API_KEY"))

result, err := client.%s(ctx, params)
var apiErr *stowline.Error
if errors.As(err, &apiErr) {
	log.Printf("%%s: %%s (request %%s)", apiErr.Code, apiErr.Message, apiErr.RequestID)
}`, goMethod(method))))
	b.WriteString("\n**PHP**\n\n")
	b.WriteString(code("php", fmt.Sprintf(`$stowline = new \Stowline\Client(getenv('STOWLINE_API_KEY'));

try {
    $result = $stowline->%s(%s);
} catch (\Stowline\ApiException $e) {
    error_log($e->getCode() . ': ' . $e->getMessage() . ' (' . $e->getRequestId() . ')');
    throw $e;
}`, strings.ReplaceAll(method, ".", "->"), pyArgs)))
	b.WriteString("\n**Ruby**\n\n")
	b.WriteString(code("ruby", fmt.Sprintf(`client = Stowline::Client.new(api_key: ENV.fetch("STOWLINE_API_KEY"))

begin
  result = client.%s(%s)
rescue Stowline::ApiError => e
  warn "#{e.code}: #{e.message} (#{e.request_id})"
  raise
end`, snake(method), pyArgs)))
	b.WriteString("\n")
	b.WriteString(practices(e))
	return b.String()
}

/*
practices is the advice for calling an operation of the kind well.
*/
func practices(e Endpoint) string {
	advice := map[string][]string{
		"POST": {
			"Send an `Idempotency-Key` header derived from your own record, such as the order id, so a retry after a timeout or a crash never does the work twice. Keys are kept for 24 hours.",
			"Retry only 429 and 5xx responses, with exponential backoff and jitter, honoring `Retry-After`. Any other 4xx will fail the same way until the request changes.",
			"Store the id of what was created in your own database in the same transaction as the record it belongs to, so you can always find it again.",
			"Log the `Request-Id` header of every failed request. Support can see exactly what happened from it.",
		},
		"DELETE": {
			"Deleting twice is safe: the second request answers 404, which you can treat as done.",
			"Remove the id from your own records only after the response confirms the deletion.",
		},
	}
	items, ok := advice[e.Method]
	if !ok {
		items = []string{
			"Cache responses that do not change often, and use webhooks instead of polling for changes.",
			"Ask only for what you need: use filters and `expand` rather than fetching every object and filtering in your code.",
			"Respect the rate limit headers; a nightly job that reads everything should page with the largest `limit` and pause when `X-RateLimit-Remaining` is low.",
		}
	}
	return "## Best practices\n\n" + list(items...) + "\n"
}

/*
sdkMethod names the operation the way the Node.js library does.
*/
func sdkMethod(e Endpoint) string {
	parts := strings.Split(strings.Trim(e.Path, "/"), "/")
	resource := parts[1]
	verb := map[string]string{"GET": "retrieve", "POST": "create", "DELETE": "del"}[e.Method]
	if verb == "retrieve" && !strings.Contains(e.Path, "{") {
		verb = "list"
	}
	if last := parts[len(parts)-1]; !strings.HasPrefix(last, "{") && len(parts) > 2 {
		verb = last
	}
	return camel(resource) + "." + verb
}

/*
camel turns snake_case into camelCase.
*/
func camel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

/*
snake turns camelCase into snake_case.
*/
func snake(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

/*
goMethod names the operation the way the Go library does.
*/
func goMethod(m string) string {
	var b strings.Builder
	for part := range strings.SplitSeq(m, ".") {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

/*
responseFields lists the top-level fields of a response with what each
holds.
*/
func responseFields(resp string) string {
	known := map[string]string{
		"id":                "The object's id. Ids are prefixed by type, such as shp_ for shipments.",
		"status":            "Where the object is in its life. New statuses may be added; treat unknown ones as in progress.",
		"to_address":        "The recipient as verified, which may differ from what you sent.",
		"parcel":            "The parcel as rated, in the units you sent.",
		"selected_rate":     "The rate the label was bought at.",
		"label":             "The label: tracking number, file address and format. Files are kept for 90 days.",
		"created_at":        "When the object was created, in UTC.",
		"data":              "The page of results.",
		"next_cursor":       "Pass as cursor to get the next page; null on the last page.",
		"refund":            "The postage refund and its status.",
		"rates":             "Each available service with its price and delivery estimate.",
		"verified":          "Whether the carrier database matched the address.",
		"residential":       "Whether the address is a home, which affects surcharges.",
		"address":           "The corrected address to use.",
		"messages":          "What was changed or could not be checked.",
		"charge":            "When the label is charged.",
		"tracking_number":   "The carrier's tracking number.",
		"carrier":           "The carrier's key, such as ups or usps.",
		"est_delivery_date": "The carrier's estimate, when it gives one.",
		"events":            "Tracking events, newest first.",
		"confirmation":      "The carrier's confirmation number.",
		"fee":               "What the carrier charges for this, if anything.",
		"form_url":          "Where to download the printable form.",
		"shipment_count":    "How many labels are on it.",
		"counts":            "How many shipments were bought and how many failed.",
		"url":               "Where events are sent.",
		"secret":            "The signing secret, shown only once.",
		"plan":              "The account's plan key.",
		"labels_used":       "Labels bought since the billing date.",
		"labels_included":   "The plan's monthly allowance.",
		"balance":           "The Stowline balance, as a decimal string.",
		"currency":          "The currency of amounts in this response.",
		"items":             "How many items the declaration holds.",
		"total_value":       "The declared value of all items.",
		"deleted":           "True once deleted.",
		"label_url":         "The combined label file.",
		"failures":          "Each failed shipment with its error code.",
		"order_number":      "Your reference.",
		"rules_applied":     "Automation rules that changed the order.",
		"premium":           "What the insurance costs.",
		"amount":            "The insured value.",
	}
	var top map[string]any
	if json.Unmarshal([]byte(resp), &top) != nil {
		return ""
	}
	keys := slices.SortedFunc(maps.Keys(top), func(a, b string) int {
		return strings.Index(resp, `"`+a+`"`) - strings.Index(resp, `"`+b+`"`)
	})
	var rows [][]string
	for _, k := range keys {
		if desc := known[k]; desc != "" {
			rows = append(rows, []string{"`" + k + "`", desc})
		}
	}
	if len(rows) == 0 {
		return ""
	}
	return "## Response fields\n\n" + table([]string{"Field", "Meaning"}, rows) +
		"\nAmounts are decimal strings, never floats, so they are exact. New fields can be added at any time; " +
		"ignore fields you do not know.\n\n"
}

/*
handling shows how an integration reacts to the error in code.
*/
func handling(e ErrorCode) string {
	retry := "Do not retry automatically; the request will fail the same way until something changes."
	if e.Status == 429 || e.Status >= 500 {
		retry = "This error is temporary. Retry with exponential backoff, and keep the same idempotency key."
	}
	return "## Handling it in code\n\n" + retry + "\n\n" + code("javascript", fmt.Sprintf(`try {
  await stowline.shipments.buy(shipmentId, { rateId });
} catch (err) {
  if (err.code === "%s") {
    // %s
    await flagOrderForReview(orderId, err.message);
  } else {
    throw err;
  }
}`, e.Code, e.Fixes[0])) + "\n" + code("python", fmt.Sprintf(`try:
    client.shipments.buy(shipment_id, rate_id=rate_id)
except stowline.ApiError as err:
    if err.code != "%s":
        raise
    flag_order_for_review(order_id, err.message)`, e.Code))
}
