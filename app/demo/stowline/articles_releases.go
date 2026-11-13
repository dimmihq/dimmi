/*
|--------------------------------------------------------------------------
| Release Notes
|--------------------------------------------------------------------------
|
| Two years of changes, oldest first: title, what changed, who it
| affects and how to use it. Several change the same feature twice, so
| older notes describe screens and names that later notes replace, as a
| real product's history does.
|
*/

package stowline

/*
releaseNotes are the changes, oldest first.
*/
func releaseNotes() [][4]string {
	return [][4]string{
		{"Shipping presets", "Presets save a package, service and options under a name you pick when buying a label.", "Everyone buying labels by hand.", "Settings, Shipping, Presets. Presets were later replaced by automation rules."},
		{"USPS Ground Advantage", "Ground Advantage replaces First-Class Package and Parcel Select Ground for USPS labels.", "Stores shipping with USPS.", "Nothing to do; rules that named the old services were moved to Ground Advantage."},
		{"Packing slip logos", "Packing slips can carry your logo.", "Stores printing packing slips.", "Settings, Shipping, Packing slips, Logo."},
		{"Batch size raised to 500", "Batches can now hold 500 orders instead of 250.", "High-volume stores.", "Select up to 500 orders and choose Create batch."},
		{"Two-factor authentication", "Accounts can require a code from an authenticator app at sign-in.", "Every teammate.", "Profile, Security."},
		{"Etsy integration", "Etsy orders import every 15 minutes and tracking goes back automatically.", "Etsy sellers.", "Settings, Integrations, Etsy."},
		{"Address verification for Canada", "Canadian addresses are verified against Canada Post's database.", "Stores shipping to Canada.", "On for everyone."},
		{"Scan forms for USPS", "End-of-day SCAN forms close the day's USPS labels into one barcode.", "USPS shippers handing over many parcels.", "Labels, Scan forms. Later renamed Manifests."},
		{"Webhook signatures", "Every webhook delivery is signed with HMAC-SHA256.", "Developers.", "Read the Stowline-Signature header; the secret is on the webhook's page."},
		{"Returns portal beta", "Customers can start returns on a page you brand.", "Pro accounts in the beta.", "Returns, Settings."},
		{"Insurance through Stowline", "Insure parcels up to $5,000 at $0.95 per $100.", "Everyone.", "Add insurance on the order or in a preset."},
		{"Carrier adjustments page", "Adjustments carriers send after delivery are listed with the order they belong to.", "Accounts paying postage from their balance.", "Billing, Adjustments."},
		{"Automation rules", "Rules apply a service, package and options to new orders automatically. They replace presets, which keep working until removed.", "Everyone buying more than a few labels a day.", "Settings, Automation. Existing presets can be converted from the rules page."},
		{"Cheapest rate in rules", "A rule can pick the cheapest service that arrives within a number of days.", "Stores comparing carriers.", "In a rule, choose Cheapest rate as the service."},
		{"DHL Express", "DHL Express accounts can be connected.", "International shippers.", "Settings, Carriers, Add carrier."},
		{"Print helper", "A desktop app prints labels the moment they are bought, without a print dialog.", "Warehouses with thermal printers.", "Settings, Printing, Print helper."},
		{"Rate quotes API", "POST /v1/rates prices a parcel without creating a shipment.", "Developers building checkout rates.", "See Getting rates without a shipment."},
		{"Branded tracking page", "Tracking pages carry your logo and colors and can promote products.", "Growth and above.", "Tracking, Page."},
		{"Batch size raised to 1,000", "Batches can now hold 1,000 orders.", "High-volume stores.", "No change needed."},
		{"WooCommerce integration rebuilt", "WooCommerce connects with a REST key instead of the plugin.", "WooCommerce stores.", "Reconnect from Settings, Integrations; the plugin stops syncing at the end of the month."},
		{"Saturday delivery", "Saturday delivery can be added where UPS and FedEx offer it.", "Stores with weekend customers.", "Option on the order or in a rule."},
		{"Returns portal for everyone on Pro", "The returns portal leaves beta.", "Pro and Enterprise.", "Returns, Settings."},
		{"API keys with scopes", "Keys can be limited to the operations a system needs.", "Developers.", "Settings, API keys. Older keys keep full access until rolled."},
		{"Manifests", "Scan forms are now called manifests and cover every carrier that supports them.", "Everyone handing over parcels.", "Labels, Manifests. The old Scan forms page redirects here."},
		{"Canada Post contract rates", "Canada Post connections accept a contract number for contract rates.", "Canadian stores.", "Edit the Canada Post connection and add the contract number."},
		{"Single sign-on", "Enterprise accounts can sign in through SAML.", "Enterprise.", "Settings, Security, Single sign-on."},
		{"Residential detection", "Stowline detects whether an address is a home before rating, so residential surcharges are priced up front.", "Stores getting residential adjustments.", "On by default; set per order to override."},
		{"Faster label PDFs", "Label files open in under a second.", "Everyone.", "Nothing to do."},
		{"QR code returns for USPS", "Customers can show a QR code at a post office instead of printing.", "Pro accounts.", "Returns, Settings, Label options."},
		{"Idempotency keys", "POST requests accept an Idempotency-Key so retries never buy twice.", "Developers.", "Send the header; the official libraries do it for you."},
		{"Cursor pagination", "List operations page with cursors instead of offsets.", "Developers.", "Pass next_cursor as cursor. Offsets keep working on older API versions."},
		{"Amazon tracking upload", "Tracking for Amazon orders uploads with the label date as the ship date.", "Amazon sellers.", "Automatic."},
		{"Purolator", "Purolator accounts can be connected.", "Canadian stores.", "Settings, Carriers."},
		{"Store credit in returns", "Customers can choose store credit, with an optional bonus.", "Shopify and BigCommerce on Pro.", "Returns, Settings, Resolution options."},
		{"Exchanges in returns", "Customers can exchange for another size or color.", "Shopify and BigCommerce on Pro.", "Returns, Settings, Resolution options."},
		{"Audit log", "Enterprise accounts see who changed what, kept for two years.", "Enterprise admins.", "Settings, Security, Audit log."},
		{"Pickup booking for more carriers", "Pickups can be booked for USPS, Canada Post and Australia Post.", "Stores not using drop-off.", "Labels, Schedule pickup."},
		{"Weight from scales", "USB scales fill the weight on the order.", "Warehouses.", "Settings, Printing, Scale."},
		{"Balance auto top-up threshold", "You choose when the balance tops up and by how much.", "Accounts paying postage through Stowline.", "Billing, Balance."},
		{"HS codes on products", "Products store an HS code and origin that fill customs automatically.", "International shippers.", "Products, edit a product."},
		{"Adjustment disputes", "Adjustments can be disputed from the dashboard with a photo.", "Accounts receiving adjustments.", "Billing, Adjustments, Dispute."},
		{"Australia Post eParcel", "eParcel contract accounts can be connected.", "Australian stores.", "Settings, Carriers."},
		{"IP allowlists", "Enterprise accounts can limit sign-ins and API keys to listed addresses.", "Enterprise.", "Settings, Security, IP allowlist."},
		{"Webhook secret rotation", "Webhook secrets can be rotated with a 24-hour overlap.", "Developers.", "Webhook page, Rotate secret."},
		{"NetSuite SuiteApp", "NetSuite connects through a SuiteApp and fulfills sales orders.", "Brands on NetSuite.", "Settings, Integrations, NetSuite."},
		{"Long notes packing slips", "A packing slip layout prints gift messages and personalization in full.", "Etsy and gift stores.", "Settings, Packing slips, Layout."},
		{"Custom roles", "Enterprise accounts can build roles from individual permissions.", "Enterprise admins.", "Settings, Team, Roles."},
		{"Batches retry on carrier errors", "Batches retry failed labels for an hour when the carrier is slow.", "Everyone using batches.", "Automatic."},
		{"Royal Mail Click & Drop", "Royal Mail connects with a Click & Drop API key.", "UK stores.", "Settings, Carriers."},
		{"GLS", "GLS accounts can be connected.", "European stores.", "Settings, Carriers."},
		{"Rules can split orders", "A rule action can split an order into parcels by SKU tag.", "Stores shipping fragile items apart.", "Rule action Split by tag."},
		{"Paperless trade for DHL Express", "Commercial invoices go electronically wherever accepted.", "DHL Express shippers.", "Automatic."},
		{"Address suggestions while typing", "The order editor suggests corrected addresses as you type.", "Everyone editing orders.", "Automatic."},
		{"Reports API", "POST /v1/reports prepares CSV exports in the background.", "Developers and finance teams.", "See Creating a report."},
		{"Faster batches", "Batches buy labels in parallel per carrier; 1,000 UPS labels in under ten minutes.", "High-volume stores.", "Automatic."},
		{"QR code returns for Royal Mail", "UK customers can return parcels at a Post Office counter without printing.", "UK stores on Pro.", "Returns, Settings, Label options."},
		{"Adjustment details", "Each adjustment shows what the carrier measured next to what was on the label.", "Accounts receiving adjustments.", "Billing, Adjustments."},
		{"Etsy personalization on slips", "Personalization prints in full on the long notes layout.", "Etsy sellers.", "Settings, Packing slips."},
		{"Idempotency mismatches refused", "Repeating an idempotency key with a different body is refused instead of returning the first result.", "Developers.", "Generate one key per distinct request."},
		{"Decimal string amounts", "Every amount in the API is a decimal string.", "Developers on API version 2026-03-01.", "Parse amounts as decimals, never floats."},
		{"AI assistant in the messenger", "The messenger answers from the help center at any hour and hands to the team when it cannot.", "Everyone writing to support.", "Open the messenger on any page."},
		{"Evri requests collected", "Evri is not supported yet; requests are collected on the feedback board.", "UK stores.", "Vote on the feedback board."},
		{"Balance refunds to card", "Balance from recent top-ups can be refunded to the card on request.", "Accounts closing or downsizing.", "Ask support from the messenger."},
		{"Webhook test events", "Send a signed test event to an endpoint from its page or the API.", "Developers.", "Webhook page, Send test event."},
		{"Rolling API keys", "API keys can be rolled with a grace period of up to 7 days.", "Developers.", "Settings, API keys, Roll."},
		{"Warehouses in the API", "Warehouses can be managed through /v1/warehouses.", "Brands with many locations.", "See The warehouse object."},
		{"Orders API", "Orders can be created through /v1/orders, and automation rules run on them.", "Stores on platforms Stowline does not connect to.", "See Creating an order through the API."},
		{"Hold orders", "Orders can be held until a date or until released.", "Pre-order stores.", "Order page, Hold, or POST /v1/orders/{id}/hold."},
		{"Peak season checklist", "A checklist page walks through cut-offs, balances and printers before Black Friday.", "Everyone.", "Help center, Peak season."},
		{"Event resend", "Missed webhook events can be resent from the events list.", "Developers.", "See Resending an event."},
		{"Stronger read consistency", "Send Stowline-Consistency: strong to read your own write.", "Developers.", "Add the header to reads that must see a just-made change."},
		{"Batches sort by SKU", "Batch labels can print sorted by SKU for picking.", "Warehouses.", "Batch, Print all, Sort."},
	}
}
