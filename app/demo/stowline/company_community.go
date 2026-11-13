/*
|--------------------------------------------------------------------------
| Changelog And Feedback
|--------------------------------------------------------------------------
|
| What Stowline published on its changelog this year, and what its
| customers asked for on the feedback board.
|
*/

package stowline

/*
ChangelogEntries are Stowline's product updates: title, summary, body.
*/
func ChangelogEntries() [][3]string {
	return [][3]string{
		{"Faster batches", "Batches of 1,000 labels now finish in under ten minutes.", "We rebuilt how batches buy labels. Labels are now bought in parallel per carrier, so a batch of 1,000 UPS labels finishes in under ten minutes instead of half an hour.\n\nFailed labels are retried automatically for an hour when the carrier is slow, and **Retry failed** only buys what is still missing."},
		{"QR code returns for Royal Mail", "UK customers can return parcels at a Post Office counter without printing.", "The returns portal now offers QR code returns for Royal Mail. Your customer shows the code at any Post Office counter and the clerk prints the label.\n\nTurn it on under **Returns → Settings → Label options**."},
		{"Address suggestions while you type", "Fix addresses before they fail.", "The order editor now suggests corrected addresses from the carrier databases as you type, and flags a missing apartment number before you buy a label."},
		{"Carrier adjustments, explained", "See exactly what the carrier measured.", "Every adjustment under **Billing → Adjustments** now shows the carrier's measured weight and dimensions next to what was on the label, with a one-click dispute."},
		{"Etsy personalization on packing slips", "Long personalization notes print in full.", "The new long notes packing slip layout prints Etsy personalization and gift messages in full, wrapping onto a second page when needed."},
		{"API: idempotency keys on every purchase", "Retries can never buy twice.", "Every purchase endpoint now honors the `Idempotency-Key` header for 24 hours. The official libraries send one automatically.\n\n```bash\ncurl -X POST https://api.stowline.test/v1/shipments/shp_123/buy \\\n  -H \"Idempotency-Key: order-1042\"\n```"},
		{"DHL Express paperless trade", "No more printed invoices for most destinations.", "DHL Express commercial invoices are now sent electronically for every destination that accepts them. Stowline turns it on automatically."},
		{"Our AI assistant answers around the clock", "Get answers from the help center at any time.", "The messenger now answers questions from our help center at any hour, with links to the articles it used. When it can't help, it passes the conversation to our team with everything you've said."},
	}
}

/*
FeedbackRequests are the requests on Stowline's feedback board: title
and body.
*/
func FeedbackRequests() [][2]string {
	return [][2]string{
		{"Bulk edit weights on the orders page", "We ship hundreds of the same item. Please let us select orders and set the weight once."},
		{"Support Evri", "Most of our UK customers prefer Evri pickup shops. Please add it as a carrier."},
		{"Split an order into parcels by SKU automatically", "Our fragile items always ship separately. An automation rule action to split by SKU would save us hours."},
		{"Scheduled reports by email", "A weekly email with label spend by carrier would help our finance team."},
		{"Mobile app for scanning parcels", "A phone app to scan a packing slip barcode and print the label would speed up our warehouse."},
		{"Show delivery estimates on the tracking page in the customer's time zone", "Customers abroad get confused by our time zone."},
		{"Packing slip templates per store", "We run three stores with different branding."},
		{"Two-step approval for voids", "Warehouse staff sometimes void the wrong label."},
		{"Webhook for adjustments", "We want to reconcile adjustments automatically in our ERP."},
		{"Dark mode", "Our warehouse runs night shifts."},
	}
}
