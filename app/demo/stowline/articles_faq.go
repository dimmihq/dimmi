/*
|--------------------------------------------------------------------------
| Common Questions
|--------------------------------------------------------------------------
|
| Real help centers end most articles with the questions customers ask
| next. Each section has its own hand-written pool, and every article
| takes a few of them, picked by its place in the section, so siblings
| do not end the same way.
|
*/

package stowline

import (
	"fmt"
	"strings"
)

/*
qa is one common question and its answer.
*/
type qa struct{ q, a string }

/*
questionPools are each section's common questions.
*/
func questionPools() map[string][]qa {
	return map[string][]qa{
		"getting-started": {
			{"Can I try Stowline before connecting my store?", "Yes. Create an order by hand on the Orders page, or switch your account to test mode under Settings, Developers, and buy as many sample labels as you like. Sample labels are free and clearly marked so they cannot be shipped."},
			{"Do I need a thermal printer?", "No. Every label can be printed on letter or A4 paper and taped to the parcel. Most stores buy a thermal printer once they ship more than about twenty parcels a day, because it is faster and needs no ink."},
			{"How long does it take for new orders to appear?", "Orders from connected stores appear within the sync interval of the integration, five minutes for Shopify and BigCommerce and up to fifteen for marketplaces. Click Sync now on the integration to import straight away."},
			{"Can I use Stowline in more than one currency?", "Rates are always shown in the currency the carrier bills in. Customs values use the order's currency, and your Stowline balance is held in the currency of your billing country."},
			{"Is there a mobile app?", "There is no separate app, but the dashboard works on phones and tablets, and the print helper can print to a Bluetooth thermal printer from a tablet in the warehouse."},
			{"Where do I find my account id?", "It is shown under Settings, Account, and starts with acct_. Support may ask for it when you write in from an address that is not on the account."},
		},
		"shipping-labels": {
			{"Can I change the service after buying a label?", "No. A label is tied to its service. Void it and buy a new one with the right service; the void refunds the first label as described in Voiding a label and getting your money back."},
			{"Why is the label price different from the rate I saw on import?", "Rates are fetched again when you buy, because carriers change prices and surcharges. A rate older than 24 hours is always fetched again, and you are asked to confirm if the price changed."},
			{"Can I reprint a label?", "Yes, as often as you need, from the order or the Labels page. Reprinting is free. Do not photocopy a label and use it twice; carriers bill each use and may suspend accounts that reuse labels."},
			{"How do I ship one order in several boxes?", "Open the order, click Split into parcels and enter each box's weight and size. Carriers that support multi-parcel shipments link the labels under one master tracking number; others get one label per box."},
			{"Can I add a return address that is not my warehouse?", "Yes. Each warehouse has its own return address setting, and an automation rule can set a different return address per order."},
			{"What happens to labels I never use?", "If you do not void them, most carriers charge for them anyway. USPS refunds unused labels only when they are voided within 30 days."},
		},
		"carriers": {
			{"Can I use Stowline's rates and my own account for the same carrier?", "For USPS, yes: Stowline's rates are always available beside any USPS contract you connect. For other carriers you use either your own account or Stowline's, and the rate list shows which one each rate comes from."},
			{"Why do I see fewer services than the carrier offers?", "Rates only show services the carrier offers for that exact parcel and route. A service missing for one order may appear for another. Services can also be hidden under the carrier connection's settings."},
			{"Does connecting my account change my negotiated rates?", "No. The carrier prices labels bought through Stowline exactly as labels bought in its own portal. If rates look different, check that the connection uses the account number your discount is attached to."},
			{"Who do I contact about a lost parcel?", "Start with us from the label's page. We open the trace with the carrier and keep you updated. For parcels on your own carrier account you can also contact the carrier directly."},
			{"Can I connect a carrier that is not listed?", "Not yet. Tell us which carrier you need from the messenger; we add carriers based on how many stores ask for them."},
			{"How are carrier holiday schedules handled?", "Delivery estimates use the carrier's own calendar, so a national holiday pushes the estimate automatically. Pickups cannot be booked on days the carrier does not collect."},
		},
		"integrations": {
			{"Will Stowline change my store's orders?", "Only to mark them fulfilled and add the tracking number, and only if the integration is set to send tracking back. It never changes items, prices or customer details."},
			{"Can I connect a staging or development store?", "Yes. Development stores connect like real ones. Use test mode so labels for test orders are never charged."},
			{"What happens to orders when I disconnect a store?", "Imported orders stay in Stowline with their labels. New orders stop importing, and tracking stops being sent back."},
			{"Does Stowline import canceled orders?", "No. If an order is canceled in the store after it was imported, Stowline marks it canceled at the next sync and warns you if a label was already bought for it."},
			{"Can different stores ship from different warehouses?", "Yes. Each store connection has a default warehouse, and automation rules can override it per order."},
			{"Is there a limit on how many orders can be imported?", "No hard limit. Very large stores, above 20,000 orders a day, should talk to us about a dedicated sync schedule."},
		},
		"tracking-returns": {
			{"Can customers track parcels without an email?", "Yes. The tracking page has a search box for the order number and postal code, so customers can look up their parcel without the email."},
			{"Can I turn off tracking emails for some orders?", "Yes. Add the tag no-tracking-email to the order, by hand or with an automation rule, and Stowline sends no tracking emails for it."},
			{"Who pays for a return label nobody uses?", "Nobody. Return labels are charged only when the carrier first scans them."},
			{"Can I accept returns from other countries?", "Yes, with UPS, FedEx and DHL Express. The return label includes a customs declaration marked as returned goods so duties are not charged again in most countries."},
			{"How do I see why customers return things?", "The Returns page groups returns by reason and product, and the reasons can be exported as CSV for any date range."},
			{"Does the tracking page work on my own domain?", "Yes, on Growth and above. Add a CNAME record for the subdomain you want, and Stowline issues the certificate automatically."},
		},
		"billing": {
			{"Can I get a refund for a label I did not use?", "Void the label within the carrier's void window and the postage comes back to your Stowline balance. Plan fees are not refunded for partial months."},
			{"Why was I charged twice?", "Usually it is a plan invoice and a balance top-up on the same day, or two top-ups during a busy day. Billing, Balance history shows every charge with what it was for."},
			{"Can I pay by bank transfer?", "Enterprise accounts can pay invoices by bank transfer. Other plans need a card for automatic balance top-ups."},
			{"Do you charge for voided labels?", "No. Voided labels do not count toward your allowance and their postage is refunded."},
			{"Can I change the email address invoices go to?", "Yes, under Billing, Billing contacts. Invoices go to every billing contact and to the account owner."},
			{"What currency am I billed in?", "Plans are billed in US dollars, euros or pounds depending on your billing country. Postage is billed in the carrier's currency."},
		},
		"account-security": {
			{"Can I have more than one owner?", "No, every account has exactly one owner. Give other people the Admin role; admins can do everything except close the account and change the owner."},
			{"How do I change my email address?", "Go to Profile, Email, enter the new address and confirm it from the email we send. Your sign-in address changes once confirmed."},
			{"Where is my data stored?", "In the United States, with a copy in a second US region for disaster recovery. Enterprise accounts can ask for data residency in the EU."},
			{"Is Stowline SOC 2 compliant?", "Yes, Stowline holds a SOC 2 Type II report, available to customers under NDA from the Trust page."},
			{"Can I see who bought a label?", "Every label shows who bought it, or which API key or automation bought it. Enterprise accounts also have the full audit log."},
			{"How long is a session valid?", "Sessions last 30 days on the same browser. Owners can shorten that for everyone under Settings, Security."},
		},
		"api": {
			{"Is there an SDK?", "Official libraries exist for Node.js, Python, PHP, Ruby and Go. They handle authentication, retries with backoff, idempotency keys and pagination for you."},
			{"Is there a sandbox?", "Use a test key. Test mode is a full copy of the API that never reaches carriers and never charges you, and test data is kept separate from live data."},
			{"How do I find a request in the logs?", "Every response carries a Request-Id header. Search for it under Settings, Developers, Logs, where requests from the last 30 days are kept."},
			{"Do you send webhooks in order?", "Not guaranteed. Deliveries can arrive out of order, especially after retries. Use the occurred_at field of each event to decide which is newest."},
			{"Can I get rates in my checkout?", "Yes, with the rates endpoint. Cache quotes for the same route and parcel for up to an hour to stay within rate limits during busy periods."},
			{"Which TLS versions do you accept?", "TLS 1.2 and 1.3. Requests over plain HTTP are refused, not redirected."},
		},
		"troubleshooting": {
			{"Where can I see the full error?", "Open the order and click the red error badge. It shows our explanation, the carrier's own code and message, and the request id to send us."},
			{"Is there a status page?", "Yes, at status.stowline.test. It shows incidents for Stowline itself and for each carrier's systems as we see them."},
			{"Will retrying make things worse?", "Not for labels. Every purchase carries an idempotency key, so retrying a failed purchase never buys two labels."},
			{"Why does the same order work for my colleague?", "Check that you are both using the same warehouse and carrier connection. Connections can be limited to some warehouses, and a rule may pick a different service for each of you."},
			{"Can support fix it for me?", "Yes. Send us the order number and we will look at the request the carrier answered. Most problems can be fixed on the order without any change to your settings."},
			{"How fast does support answer?", "Within four business hours on Growth, two on Pro and one on Enterprise, from 6 a.m. to 8 p.m. Eastern on weekdays. The AI assistant in the messenger answers at any time."},
		},
	}
}

/*
withQuestions ends each article with common questions from its
section, four for most articles and six for the longer technical ones.
*/
func withQuestions(out []Article) []Article {
	pools := questionPools()
	seen := map[string]int{}
	for i := range out {
		a := &out[i]
		pool := pools[a.Collection]
		if len(pool) == 0 || a.Visibility == "agent_only" {
			continue
		}
		n := 4
		if a.Collection == "api" || a.Collection == "troubleshooting" {
			n = 5
		}
		start := seen[a.Collection]
		seen[a.Collection]++
		var b strings.Builder
		b.WriteString("\n\n## Common questions\n\n")
		for k := range n {
			q := pool[(start*3+k)%len(pool)]
			fmt.Fprintf(&b, "**%s**\n\n%s\n\n", q.q, q.a)
		}
		a.Body = strings.TrimSpace(a.Body) + b.String()
	}
	return out
}
