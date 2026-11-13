/*
|--------------------------------------------------------------------------
| API Reference: Resources
|--------------------------------------------------------------------------
|
| Every object Stowline's API exposes, field by field, and the
| operations each one has. The reference pages are built from these
| facts, so a field reads the same on every page that shows it, as in a
| real reference, and each operation's page carries the whole object.
|
*/

package stowline

/*
Field is one field of an API object: its name, type, what it holds,
an example value as JSON, and whether a request may set it.
*/
type Field struct {
	Name, Type, About, Example string
	Writable                   bool
}

/*
Resource is one API object and its operations.
*/
type Resource struct {
	Key, Name, Plural, Path, Scope, About string
	Fields                                []Field
	Ops                                   []string
	Specials                              []Special
	Since                                 string
}

/*
Special is an operation beyond the usual five.
*/
type Special struct {
	Verb, Method, Suffix, Title, About string
}

func f(name, typ, about, example string) Field { return Field{name, typ, about, example, false} }
func w(name, typ, about, example string) Field { return Field{name, typ, about, example, true} }

/*
Resources returns every API object.
*/
func Resources() []Resource {
	crud := []string{"list", "create", "retrieve", "update", "delete"}
	return []Resource{
		{
			Key: "shipment", Name: "shipment", Plural: "shipments", Path: "/v1/shipments", Scope: "shipments", Since: "2023-01-15",
			About: "A shipment is one parcel, or one multi-parcel consignment, traveling from a ship-from address to a recipient. It holds the addresses, the parcel, the customs declaration, the rates carriers quoted and, once bought, the label.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed shp_.", `"shp_01JC8Z4M2Q"`),
				f("object", "string", "Always shipment.", `"shipment"`),
				f("status", "enum", "One of draft, rated, purchased, void_pending, voided, in_transit, delivered, returned, failed. New statuses may be added; treat unknown values as in progress.", `"purchased"`),
				w("reference", "string", "Your own reference, up to 64 characters, printed on labels that support it and searchable with the reference filter.", `"order 1042"`),
				w("from_address", "object or id", "The ship-from address, or the id of a saved warehouse address (wh_...). Required to rate.", `"wh_01HZX3"`),
				w("to_address", "address object", "The recipient. Verified when the shipment is rated; the verified form is returned in verified_to_address.", `{"name": "Ada Okafor", "street1": "417 Montgomery St", "city": "San Francisco", "state": "CA", "postal_code": "94104", "country": "US"}`),
				f("verified_to_address", "address object", "The recipient as the carrier database corrected it, or null when verification was skipped.", `null`),
				w("return_address", "object or id", "Where returned parcels go. Defaults to the warehouse's return address.", `null`),
				w("parcel", "parcel object", "Dimensions and weight. Weight is required; length, width and height are needed for dimensional weight pricing. Units are set by distance_unit and mass_unit.", `{"length": 10, "width": 8, "height": 4, "distance_unit": "in", "weight": 32, "mass_unit": "oz"}`),
				w("parcels", "array of parcel objects", "For multi-parcel shipments, up to 50 parcels sharing one master tracking number where the carrier supports it. Mutually exclusive with parcel.", `null`),
				w("customs", "customs id or object", "The customs declaration, required when the shipment crosses a border. See the customs declaration object.", `null`),
				w("options", "options object", "Label format, signature, insurance, Saturday delivery, delivery confirmation, dry ice, alcohol, hold for pickup, print custom fields and label date.", `{"label_format": "pdf_4x6", "signature": "none"}`),
				w("carrier_accounts", "array of ids", "Limit rating to these carrier connections. Empty means every active connection for the ship-from warehouse.", `[]`),
				f("rates", "array of rate objects", "Every quote carriers returned for the shipment, cheapest first. Rates older than 24 hours are fetched again before purchase.", `[{"id": "rate_01JC8Z5A", "carrier": "ups", "service": "UPS Ground", "amount": "11.42", "currency": "USD", "delivery_days": 3}]`),
				f("messages", "array of message objects", "Warnings and errors carriers returned while rating, such as a service that does not serve the destination.", `[]`),
				f("selected_rate", "rate object", "The rate the label was bought at, or null before purchase.", `{"id": "rate_01JC8Z5A", "carrier": "ups", "service": "UPS Ground", "amount": "11.42"}`),
				f("label", "label object", "The label: tracking number, file address, format and the date it was printed for. Label files are kept for 90 days.", `{"id": "lbl_01JC8Z5K", "tracking_number": "1Z999AA10123456784", "label_url": "https://files.stowline.test/labels/lbl_01JC8Z5K.pdf", "format": "pdf_4x6"}`),
				f("tracker", "tracker id", "The tracker created for the label, or null before purchase.", `"trk_01JC8Z6B"`),
				f("insurance", "insurance object", "Stowline insurance on the shipment, with the insured amount and premium.", `null`),
				f("refund", "refund object", "The postage refund of a voided label: status submitted, refunded or rejected, the amount and when the carrier decided.", `null`),
				f("batch", "batch id", "The batch the shipment was bought in, if any.", `null`),
				w("order", "order id", "The order the shipment ships, when it came from a connected store.", `"ord_01JCH"`),
				w("metadata", "object", "Up to 20 keys of your own, each value up to 500 characters. Returned unchanged and never shown to customers.", `{"erp_id": "SO-48211"}`),
				f("mode", "enum", "live or test, from the key that created it.", `"live"`),
				f("created_at", "timestamp", "When the shipment was created, in UTC, RFC 3339.", `"2026-10-02T14:21:07Z"`),
				f("updated_at", "timestamp", "When anything on the shipment last changed.", `"2026-10-02T14:21:41Z"`),
			},
			Ops: crud,
			Specials: []Special{
				{"buy", "POST", "/{id}/buy", "Buying a shipment's label", "Buys the label for one of the shipment's rates and returns the shipment with its label and tracker. Send an Idempotency-Key so a retried request never buys a second label."},
				{"void", "POST", "/{id}/void", "Voiding a shipment's label", "Voids the label and submits the postage refund. The refund lands on the Stowline balance once the carrier confirms the label was never scanned."},
				{"rerate", "POST", "/{id}/rerate", "Fetching fresh rates for a shipment", "Asks every carrier again for rates, for example after changing the parcel or when the rates are older than 24 hours."},
				{"label", "GET", "/{id}/label", "Downloading a label in another format", "Returns the label rendered in the format asked for, such as zpl_4x6 for a thermal printer, without buying a new one."},
			},
		},
		{
			Key: "rate", Name: "rate quote", Plural: "rates", Path: "/v1/rates", Scope: "rates", Since: "2023-01-15",
			About: "A rate quote prices a parcel between two postal codes without creating a shipment, for showing shipping prices at checkout. Quotes leave out surcharges that need the full address, such as residential delivery.",
			Fields: []Field{
				w("from_postal_code", "string", "Origin postal code.", `"94104"`),
				w("from_country", "string", "Two-letter origin country, defaulting to the ship-from warehouse's.", `"US"`),
				w("to_postal_code", "string", "Destination postal code.", `"10001"`),
				w("to_country", "string", "Two-letter destination country.", `"US"`),
				w("residential", "boolean", "Price as a residential delivery. Defaults to true, which is right for most online orders.", `true`),
				w("parcel", "parcel object", "Weight and dimensions.", `{"weight": 32, "mass_unit": "oz", "length": 10, "width": 8, "height": 4, "distance_unit": "in"}`),
				w("services", "array of strings", "Limit to these services, by their keys such as ups_ground.", `[]`),
				w("delivery_promise_days", "integer", "Only return services estimated to arrive within this many business days.", `null`),
				f("rates", "array of rate objects", "The quotes, cheapest first.", `[{"carrier": "usps", "service": "Ground Advantage", "amount": "8.71", "delivery_days": 4}]`),
				f("expires_at", "timestamp", "When the quotes stop being valid for display. Quotes are cached for an hour per route and parcel.", `"2026-10-02T15:21:07Z"`),
			},
			Ops: []string{"create"},
		},
		{
			Key: "address", Name: "address", Plural: "addresses", Path: "/v1/addresses", Scope: "shipments", Since: "2023-01-15",
			About: "An address is a saved place a parcel goes to or comes from. Saving an address verifies it against carrier databases and stores the corrected form beside what you sent.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed adr_.", `"adr_01JD2"`),
				w("name", "string", "The person's name, up to 35 characters for most carriers; longer names are cut on the label.", `"Ada Okafor"`),
				w("company", "string", "Company name, printed on the label above the street.", `"Pinecrest Ceramics"`),
				w("street1", "string", "Street and number.", `"417 Montgomery St"`),
				w("street2", "string", "Apartment, suite, unit or floor. Missing unit numbers are the most common reason verification fails.", `"Suite 400"`),
				w("city", "string", "City or town.", `"San Francisco"`),
				w("state", "string", "State, province or region code. Required for US, Canada, Australia and Mexico.", `"CA"`),
				w("postal_code", "string", "Postal code. US ZIP+4 is accepted and returned.", `"94104"`),
				w("country", "string", "Two-letter ISO country code. UK is not valid; use GB.", `"US"`),
				w("phone", "string", "Phone number in any format. Required by DHL Express, GLS ExpressParcel and most international services.", `"+1 415 555 0100"`),
				w("email", "string", "Email for carrier delivery notifications, which you may prefer to leave empty when Stowline sends tracking emails.", `"ada@pinecrest.test"`),
				w("residential", "boolean or null", "Whether the address is a home. Null asks Stowline to detect it.", `null`),
				f("verification", "verification object", "Whether the carrier database matched the address, the corrected form, and messages such as Street suffix added.", `{"verified": true, "messages": ["Street suffix added"]}`),
				f("created_at", "timestamp", "When the address was saved.", `"2026-09-14T09:02:11Z"`),
			},
			Ops:      []string{"list", "create", "retrieve", "delete"},
			Specials: []Special{{"verify", "POST", "/verify", "Verifying an address without saving it", "Checks an address against a carrier database and returns the corrected form, without saving it. Free, and limited to 10 requests a second."}},
		},
		{
			Key: "tracker", Name: "tracker", Plural: "trackers", Path: "/v1/trackers", Scope: "shipments", Since: "2023-03-01",
			About: "A tracker follows one tracking number through a carrier's network. Stowline creates one for every label it sells; you can create trackers for parcels sent elsewhere too, at one tracker each against your plan.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed trk_.", `"trk_01JC8Z6B"`),
				w("tracking_number", "string", "The carrier's tracking number.", `"1Z999AA10123456784"`),
				w("carrier", "string", "The carrier key. Required only for tracking numbers Stowline did not create.", `"ups"`),
				f("status", "enum", "pre_transit, in_transit, out_for_delivery, delivered, available_for_pickup, return_to_sender, failure or unknown.", `"in_transit"`),
				f("status_detail", "string", "A finer reason, such as address_correction or held_at_customs, when the carrier gives one.", `"arrived_at_facility"`),
				f("est_delivery_date", "date", "The carrier's estimate, when it gives one.", `"2026-10-05"`),
				f("signed_by", "string", "Who signed for the parcel, when signature was required.", `null`),
				f("weight", "number", "The weight the carrier measured, in ounces, which is what adjustments are based on.", `33.6`),
				f("events", "array of event objects", "Every scan, newest first, with status, the carrier's message, location and occurred_at.", `[{"status": "in_transit", "message": "Departed from facility", "location": "Oakland, CA", "occurred_at": "2026-10-03T02:14:00Z"}]`),
				f("public_url", "string", "The branded tracking page for this parcel.", `"https://track.stowline.app/pinecrest/1Z999AA10123456784"`),
				f("created_at", "timestamp", "When tracking started.", `"2026-10-02T14:21:41Z"`),
			},
			Ops: []string{"list", "create", "retrieve"},
		},
		{
			Key: "return", Name: "return", Plural: "returns", Path: "/v1/returns", Scope: "returns", Since: "2024-02-20",
			About: "A return is a parcel coming back from a customer: its reason, its prepaid label or QR code, and where it is in your returns process. Return labels are charged only when the carrier first scans them.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed ret_.", `"ret_01JCA1"`),
				w("shipment_id", "string", "The original outbound shipment.", `"shp_01JC8Z4M2Q"`),
				w("reason", "string", "One of the reasons configured for the returns portal, or free text through the API.", `"Wrong size"`),
				w("items", "array", "The items coming back, by SKU and quantity, with an optional photo URL for reasons that need one.", `[{"sku": "TEE-BLK-M", "quantity": 1}]`),
				w("qr_code", "boolean", "Issue a QR code for the counter instead of a printable label, where the carrier supports it.", `true`),
				w("resolution", "enum", "refund, exchange or store_credit.", `"exchange"`),
				f("status", "enum", "requested, approved, label_created, in_transit, received, refunded, rejected or canceled.", `"label_created"`),
				f("label", "label object", "The return label or QR code.", `{"tracking_number": "9202090153540000000001", "qr_code_url": "https://files.stowline.test/qr/ret_01JCA1.png"}`),
				f("charge", "enum", "on_scan for prepaid labels charged when scanned, or customer_paid when deducted from the refund.", `"on_scan"`),
				f("exchange_order", "order id", "The exchange order created in the store, for exchanges.", `null`),
				f("created_at", "timestamp", "When the return was requested.", `"2026-10-09T11:40:00Z"`),
			},
			Ops: []string{"list", "create", "retrieve", "update"},
			Specials: []Special{
				{"approve", "POST", "/{id}/approve", "Approving a return", "Approves a return that needs approval under your rules and issues its label."},
				{"receive", "POST", "/{id}/receive", "Marking a return received", "Marks the parcel received at your warehouse and, for connected stores, issues the refund or store credit the resolution names."},
			},
		},
		{
			Key: "pickup", Name: "pickup", Plural: "pickups", Path: "/v1/pickups", Scope: "shipments", Since: "2023-06-12",
			About: "A pickup asks a carrier to collect labels already bought from a warehouse in a time window.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed pku_.", `"pku_01JCB"`),
				w("carrier_account", "string", "The carrier connection that collects.", `"ca_ups_01"`),
				w("warehouse", "string", "Where to collect.", `"wh_01HZX3"`),
				w("ready_at", "timestamp", "When the parcels are ready, in the warehouse's time zone.", `"2026-10-03T13:00:00-07:00"`),
				w("close_at", "timestamp", "When the warehouse closes. Windows shorter than two hours are refused by most carriers.", `"2026-10-03T17:00:00-07:00"`),
				w("shipments", "array of ids", "The shipments to collect. Their count and total weight are sent to the carrier.", `["shp_01JC8Z4M2Q"]`),
				w("instructions", "string", "Where the driver should go, up to 60 characters.", `"Loading dock B"`),
				f("status", "enum", "scheduled, canceled, missed or completed.", `"scheduled"`),
				f("confirmation", "string", "The carrier's confirmation number.", `"2929602E9CP"`),
				f("fee", "decimal string", "What the carrier charges for the pickup.", `"0.00"`),
			},
			Ops:      []string{"list", "create", "retrieve"},
			Specials: []Special{{"cancel", "POST", "/{id}/cancel", "Canceling a pickup", "Cancels a scheduled pickup. Cancel before the driver is dispatched or the carrier may still charge the fee."}},
		},
		{
			Key: "manifest", Name: "manifest", Plural: "manifests", Path: "/v1/manifests", Scope: "shipments", Since: "2023-06-12",
			About: "A manifest, also called a SCAN form or end-of-day form, closes a day's labels for one carrier account into a single document the driver scans.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed man_.", `"man_01JCC"`),
				w("carrier_account", "string", "The carrier connection.", `"ca_usps_01"`),
				w("shipments", "array of ids", "Shipments to include. Defaults to every unmanifested label of the account.", `[]`),
				w("ship_date", "date", "The date the parcels are handed over. Defaults to today.", `"2026-10-03"`),
				f("status", "enum", "creating, closed or failed.", `"closed"`),
				f("form_url", "string", "The printable form.", `"https://files.stowline.test/manifests/man_01JCC.pdf"`),
				f("shipment_count", "integer", "How many labels are on the form.", `48`),
			},
			Ops: []string{"list", "create", "retrieve"},
		},
		{
			Key: "batch", Name: "batch", Plural: "batches", Path: "/v1/batches", Scope: "shipments", Since: "2023-09-04",
			About: "A batch buys labels for up to 1,000 shipments in the background and combines them into one printable file.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed bat_.", `"bat_01JCD"`),
				w("shipments", "array", "Shipment ids, each with the rate_id to buy or a service to pick.", `[{"id": "shp_01JC8Z4M2Q", "service": "UPS Ground"}]`),
				w("label_format", "string", "The format of the combined label file.", `"pdf_4x6"`),
				w("sort", "enum", "order_number, sku or service: the order labels print in.", `"sku"`),
				f("status", "enum", "creating, purchasing, completed or failed.", `"completed"`),
				f("counts", "object", "total, purchased and failed.", `{"total": 250, "purchased": 247, "failed": 3}`),
				f("failures", "array", "Each failed shipment with its error code and message.", `[{"shipment": "shp_01JCG", "code": "ADDRESS_UNVERIFIED"}]`),
				f("label_url", "string", "The combined label file, purchased labels only.", `"https://files.stowline.test/batches/bat_01JCD.pdf"`),
			},
			Ops:      []string{"list", "create", "retrieve"},
			Specials: []Special{{"retry", "POST", "/{id}/retry", "Retrying a batch's failed shipments", "Buys again the shipments that failed, after you fixed them. Purchased labels are never bought twice."}},
		},
		{
			Key: "webhook", Name: "webhook endpoint", Plural: "webhooks", Path: "/v1/webhooks", Scope: "webhooks", Since: "2023-03-01",
			About: "A webhook endpoint receives events, such as tracker.updated, at a URL of yours, signed with the endpoint's secret.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed whk_.", `"whk_01JCE"`),
				w("url", "string", "An HTTPS URL that answers 2xx within 10 seconds. Private and loopback addresses are refused.", `"https://hooks.example.com/stowline"`),
				w("events", "array of strings", "Events to send, or [\"*\"] for all. See the list of events.", `["tracker.updated", "refund.updated"]`),
				w("description", "string", "Your note on what the endpoint is for.", `"ERP sync"`),
				w("enabled", "boolean", "Disabled endpoints receive nothing; failing endpoints are disabled automatically after three days.", `true`),
				f("secret", "string", "The signing secret, shown only when the endpoint is created or the secret rotated.", `"whsec_4f0d1c..."`),
				f("failing_since", "timestamp", "When deliveries started failing, or null.", `null`),
				f("api_version", "string", "The API version events are rendered in.", `"2026-03-01"`),
			},
			Ops: crud,
			Specials: []Special{
				{"rotate", "POST", "/{id}/rotate_secret", "Rotating a webhook secret", "Issues a new signing secret. Both secrets are valid for 24 hours so you can deploy the new one without missing deliveries."},
				{"test", "POST", "/{id}/test", "Sending a test event", "Sends a signed ping event to the endpoint and returns the status and time it answered with."},
			},
		},
		{
			Key: "carrier_account", Name: "carrier account", Plural: "carrier accounts", Path: "/v1/carrier_accounts", Scope: "account", Since: "2023-01-15",
			About: "A carrier account is a connection to a carrier, your own account with your negotiated rates or one Stowline provides.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed ca_.", `"ca_ups_01"`),
				w("carrier", "string", "The carrier key, such as ups or dhl_express.", `"ups"`),
				w("description", "string", "Your name for the connection.", `"UPS main"`),
				w("credentials", "object", "The carrier's account details. Write only; never returned.", `{"account_number": "A1B2C3"}`),
				w("warehouses", "array of ids", "Limit the connection to these warehouses. Empty means all.", `["wh_01HZX3"]`),
				w("services", "array of strings", "Hide services from rating by listing the ones to keep. Empty means all.", `[]`),
				f("status", "enum", "active, pending (waiting on the carrier's check), error or disconnected.", `"active"`),
				f("billing", "enum", "carrier when the carrier bills you, stowline when labels are paid from your balance.", `"carrier"`),
				f("last_error", "string", "The last error the carrier returned for the connection.", `null`),
			},
			Ops: crud,
		},
		{
			Key: "order", Name: "order", Plural: "orders", Path: "/v1/orders", Scope: "orders", Since: "2024-05-07",
			About: "An order is something a customer bought that needs shipping, imported from a store or created through the API. Automation rules run on it when it is created.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed ord_.", `"ord_01JCH"`),
				w("order_number", "string", "Your reference, unique per store connection.", `"WEB-88213"`),
				w("to_address", "address object", "The recipient.", `{"name": "Ada Okafor", "street1": "417 Montgomery St", "city": "San Francisco", "state": "CA", "postal_code": "94104", "country": "US"}`),
				w("items", "array", "SKU, title, quantity, unit value, weight, HS code and origin country per item.", `[{"sku": "TEE-BLK-M", "quantity": 1, "value": "24.00", "weight": 8}]`),
				w("requested_service", "string", "What the customer chose at checkout, for automation rules.", `"Standard"`),
				w("tags", "array of strings", "Tags, from the store or your own.", `["gift"]`),
				w("notes", "string", "Gift messages and notes, printed on packing slips.", `null`),
				f("status", "enum", "awaiting_shipment, shipped, partially_shipped, canceled or on_hold.", `"awaiting_shipment"`),
				f("store", "string", "The store connection it came from.", `"st_shopify_01"`),
				f("rules_applied", "array of strings", "Automation rules that changed it.", `["Domestic under 1 lb"]`),
				f("shipments", "array of ids", "Shipments created for it.", `[]`),
			},
			Ops:      crud,
			Specials: []Special{{"hold", "POST", "/{id}/hold", "Putting an order on hold", "Keeps an order out of batches and automation until a date or until released."}},
		},
		{
			Key: "customs", Name: "customs declaration", Plural: "customs declarations", Path: "/v1/customs", Scope: "shipments", Since: "2023-04-18",
			About: "A customs declaration lists what is in an international parcel, its value, origin and tariff codes, and who pays duties.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed cst_.", `"cst_01JCF"`),
				w("contents_type", "enum", "merchandise, gift, documents, returned_goods or sample.", `"merchandise"`),
				w("incoterm", "enum", "DAP (the recipient pays duties, default) or DDP (you pay).", `"DAP"`),
				w("eel_pfc", "string", "US export exemption code, such as NOEEI 30.37(a) for exports under $2,500 per HS code.", `"NOEEI 30.37(a)"`),
				w("items", "array", "Each with description, quantity, value, weight, hs_code and origin_country.", `[{"description": "Cotton t-shirt", "quantity": 2, "value": "24.00", "weight": 8, "hs_code": "610910", "origin_country": "PT"}]`),
				w("tax_ids", "array", "IOSS, UK VAT or EORI numbers sent with the declaration.", `[{"type": "ioss", "value": "IM2760000742"}]`),
				w("signer", "string", "Who certifies the declaration.", `"Maya Lindgren"`),
				f("total_value", "decimal string", "The declared value of all items.", `"48.00"`),
			},
			Ops: []string{"create", "retrieve"},
		},
		{
			Key: "insurance", Name: "insurance policy", Plural: "insurance policies", Path: "/v1/insurance", Scope: "shipments", Since: "2024-01-09",
			About: "An insurance policy covers one shipment against loss, damage and theft, up to $5,000.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed ins_.", `"ins_01JCJ"`),
				w("shipment_id", "string", "The insured shipment.", `"shp_01JC8Z4M2Q"`),
				w("amount", "decimal string", "The insured value, up to 5000.00.", `"320.00"`),
				f("premium", "decimal string", "What the insurance costs, charged to the balance.", `"3.04"`),
				f("status", "enum", "active, claimed, paid, rejected or canceled.", `"active"`),
				f("claim", "claim object", "The claim filed on the policy, its documents and decision.", `null`),
			},
			Ops:      []string{"list", "create", "retrieve"},
			Specials: []Special{{"claim", "POST", "/{id}/claims", "Filing an insurance claim", "Files a claim with the invoice and, for damage, photos. Claims are decided within 10 business days."}},
		},
		{
			Key: "refund", Name: "refund", Plural: "refunds", Path: "/v1/refunds", Scope: "shipments", Since: "2023-01-15",
			About: "A refund is the postage coming back for a voided label, or a carrier's answer to a refund request for a label scanned by mistake.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed rfd_.", `"rfd_01JCL"`),
				f("shipment_id", "string", "The voided shipment.", `"shp_01JC8Z4M2Q"`),
				f("status", "enum", "submitted, refunded or rejected.", `"submitted"`),
				f("amount", "decimal string", "The postage refunded.", `"11.42"`),
				f("destination", "enum", "balance for labels billed through Stowline, carrier_account for labels on your own account.", `"balance"`),
				f("expected_by", "date", "When the carrier is expected to decide: 14 to 21 days for USPS, 7 for most others.", `"2026-10-23"`),
				f("rejection_reason", "string", "Why the carrier refused, such as label_scanned.", `null`),
			},
			Ops: []string{"list", "retrieve"},
		},
		{
			Key: "balance_transaction", Name: "balance transaction", Plural: "balance transactions", Path: "/v1/balance_transactions", Scope: "account", Since: "2023-08-21",
			About: "A balance transaction is one movement of your Stowline balance: a top-up, a label purchase, a refund, an adjustment or an insurance premium.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed btx_.", `"btx_01JCM"`),
				f("type", "enum", "top_up, label, void_refund, adjustment, adjustment_reversal, insurance, claim_payout, credit or payout.", `"label"`),
				f("amount", "decimal string", "Positive for money in, negative for money out.", `"-11.42"`),
				f("balance_after", "decimal string", "The balance after the transaction.", `"300.98"`),
				f("source", "object", "What caused it: a shipment, an invoice, a dispute.", `{"shipment": "shp_01JC8Z4M2Q"}`),
				f("created_at", "timestamp", "When it happened.", `"2026-10-02T14:21:41Z"`),
			},
			Ops: []string{"list", "retrieve"},
		},
		{
			Key: "adjustment", Name: "adjustment", Plural: "adjustments", Path: "/v1/adjustments", Scope: "account", Since: "2024-09-02",
			About: "An adjustment is an extra charge a carrier sent after handling a parcel, for weight, dimensions, residential delivery or an address correction.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed adj_.", `"adj_01JCN"`),
				f("shipment_id", "string", "The shipment adjusted.", `"shp_01JC8Z4M2Q"`),
				f("reason", "enum", "weight, dimensions, residential, address_correction or service_change.", `"dimensions"`),
				f("declared", "object", "What was on the label.", `{"weight": 32, "length": 12, "width": 10, "height": 8}`),
				f("measured", "object", "What the carrier measured.", `{"weight": 34, "length": 14, "width": 12, "height": 10}`),
				f("amount", "decimal string", "The extra charge.", `"6.18"`),
				f("dispute", "dispute object", "A dispute filed on it, its status and decision.", `null`),
			},
			Ops:      []string{"list", "retrieve"},
			Specials: []Special{{"dispute", "POST", "/{id}/disputes", "Disputing an adjustment", "Files a dispute with a photo of the parcel on a scale with a tape measure. Disputes must be filed within 30 days."}},
		},
		{
			Key: "warehouse", Name: "warehouse", Plural: "warehouses", Path: "/v1/warehouses", Scope: "account", Since: "2023-01-15",
			About: "A warehouse is a place parcels leave from, with its address, opening hours, return address and default packages.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed wh_.", `"wh_01HZX3"`),
				w("name", "string", "Your name for it.", `"Brooklyn"`),
				w("address", "address object", "Where parcels leave from. Must match the address on carrier accounts limited to it.", `{"street1": "55 Water St", "city": "Brooklyn", "state": "NY", "postal_code": "11201", "country": "US"}`),
				w("return_address", "address object", "Where returns go, when different.", `null`),
				w("hours", "array", "Opening hours per weekday, used for pickups.", `[{"day": "mon", "open": "08:00", "close": "17:00"}]`),
				w("timezone", "string", "IANA time zone.", `"America/New_York"`),
				w("default", "boolean", "Whether new orders ship from here.", `true`),
				f("archived", "boolean", "Archived warehouses keep their history but leave menus.", `false`),
			},
			Ops: crud,
		},
		{
			Key: "api_key", Name: "API key", Plural: "API keys", Path: "/v1/api_keys", Scope: "account", Since: "2025-03-11",
			About: "An API key authenticates your systems. Keys are scoped and are either live or test.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed key_.", `"key_01JCP"`),
				w("name", "string", "Where the key is used.", `"ERP production"`),
				w("mode", "enum", "live or test.", `"live"`),
				w("scopes", "array of strings", "shipments:read, shipments:write, rates:read, returns:write, webhooks:write, orders:write, account:read.", `["shipments:read", "shipments:write"]`),
				w("allowed_ips", "array of strings", "Enterprise only: CIDR ranges the key may be used from.", `[]`),
				f("secret", "string", "The key itself, returned only when created.", `"sk_live_51Hc..."`),
				f("last_used_at", "timestamp", "When the key last authenticated a request.", `"2026-10-02T14:21:07Z"`),
			},
			Ops:      []string{"list", "create", "delete"},
			Specials: []Special{{"roll", "POST", "/{id}/roll", "Rolling an API key", "Issues a new secret for the key and keeps the old one working for a grace period you choose, up to 7 days."}},
		},
		{
			Key: "event", Name: "event", Plural: "events", Path: "/v1/events", Scope: "webhooks", Since: "2023-03-01",
			About: "An event records something that happened in your account; webhooks deliver them, and the events list keeps the last 30 days for replaying missed deliveries.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed evt_.", `"evt_01JCQ"`),
				f("type", "string", "Such as tracker.updated, refund.updated, batch.updated, shipment.purchased, return.received, adjustment.created.", `"tracker.updated"`),
				f("occurred_at", "timestamp", "When it happened. Use this, not the delivery time, to order events.", `"2026-10-03T02:14:00Z"`),
				f("data", "object", "The object as it was after the change, rendered in the endpoint's API version.", `{"object": "tracker", "status": "in_transit"}`),
				f("previous", "object", "The fields that changed, as they were before.", `{"status": "pre_transit"}`),
				f("deliveries", "array", "Each endpoint it was sent to, the status it answered with and attempts made.", `[{"webhook": "whk_01JCE", "status": 200, "attempts": 1}]`),
			},
			Ops:      []string{"list", "retrieve"},
			Specials: []Special{{"resend", "POST", "/{id}/resend", "Resending an event", "Sends an event again to one endpoint, for example after fixing a bug that rejected it."}},
		},
		{
			Key: "report", Name: "report", Plural: "reports", Path: "/v1/reports", Scope: "account", Since: "2025-06-30",
			About: "A report is an export of shipments, labels, tracking events, returns, adjustments or balance history for a date range, prepared in the background.",
			Fields: []Field{
				f("id", "string", "Unique identifier, prefixed rpt_.", `"rpt_01JCR"`),
				w("type", "enum", "shipments, labels, tracking_events, returns, adjustments or balance.", `"labels"`),
				w("start_date", "date", "First day included.", `"2026-09-01"`),
				w("end_date", "date", "Last day included. Ranges are limited to 366 days.", `"2026-09-30"`),
				w("columns", "array", "Columns to include; empty means the default set.", `[]`),
				f("status", "enum", "queued, preparing, ready or failed.", `"ready"`),
				f("url", "string", "Where to download the CSV, valid for 7 days.", `"https://files.stowline.test/reports/rpt_01JCR.csv"`),
				f("row_count", "integer", "Rows in the file.", `4213`),
			},
			Ops: []string{"list", "create", "retrieve"},
		},
	}
}
