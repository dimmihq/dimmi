/*
|--------------------------------------------------------------------------
| Troubleshooting Scenarios
|--------------------------------------------------------------------------
|
| The cost simulation's long threads: a customer with a fault works
| through it with the agent over many messages, which is where the
| conversation's memory and the cost of each further turn show.
|
*/

package stowline

import (
	"github.com/dimmihq/dimmi/app/ai/costsim"
)

/*
troubleshootingScenarios are the long troubleshooting threads.
*/
func troubleshootingScenarios() []costsim.Scenario {
	k := costsim.KindTroubleshooting
	return []costsim.Scenario{
		{
			Key: "shopify-missing-orders", Kind: k, Subject: "Shopify orders missing",
			Persona: "Shopify store owner, getting frustrated over a long back and forth.",
			Goal:    "Get missing Shopify orders into Stowline.",
			Messages: []string{
				"Orders from Shopify stopped coming in this morning.",
				"The last sync says 5 minutes ago and imported 0 orders.",
				"We have about 40 unfulfilled orders in Shopify right now.",
				"Where's the import filter?",
				"Filter shows Paid and Unfulfilled. That's right isn't it?",
				"Hmm, we installed an app yesterday that auto-fulfills digital items. Could that be it?",
				"It's set to fulfill all orders, not just digital ones. Oops.",
				"I turned it off. Do I need to do anything for the orders that were already auto-fulfilled?",
				"Where is 'include fulfilled orders'?",
				"Ok synced, 38 came in. 2 are still missing: #10422 and #10423.",
				"Those two were canceled and recreated by the customer I think.",
				"Yes found them as #10431 and #10432. All good now, thanks for your patience.",
			},
		},
		{
			Key: "woocommerce-401", Kind: k, Subject: "WooCommerce 401",
			Persona: "Freelance developer helping a WooCommerce shop.",
			Goal:    "Get WooCommerce connected after 401 errors.",
			Messages: []string{
				"Connecting WooCommerce fails with 401 Unauthorized. Keys are read/write, I've regenerated them twice.",
				"Permalinks are on Post name.",
				"We do have Wordfence.",
				"What exactly should be allowed in Wordfence?",
				"Allowlisted, still 401.",
				"Host is a managed WordPress host. They say they don't block anything.",
				"curl to /wp-json/wc/v3/orders with the keys works from my laptop.",
				"Could it be that the Authorization header is stripped? We're behind Cloudflare.",
				"Turned off the Cloudflare rule, now the connection test passes!",
			},
		},
		{
			Key: "batch-stuck", Kind: k, Subject: "Batch stuck",
			Persona: "Warehouse lead with a pickup deadline.", Goal: "Unstick a batch and print all labels.",
			Messages: []string{
				"Our batch of 340 has been stuck at 61% for 40 minutes, pickup is at 3pm.",
				"It's all FedEx labels.",
				"Is there a FedEx outage?",
				"Now it says 27 failed.",
				"The errors say FEDEX-1237.",
				"How do I fix the postal code mismatch for 27 orders without opening each one?",
				"Retried, 25 worked. Two still fail.",
				"The two are rural addresses in Montana.",
				"Ship to unverified address worked. Thanks.",
			},
		},
		{
			Key: "labels-cut-off-long", Kind: k, Subject: "Labels cut off",
			Persona: "Small business owner printing on a Zebra.", Goal: "Fix labels that print cut off and faded.",
			Messages: []string{
				"Labels are cut off at the bottom, barcode half missing.",
				"I'm printing from Chrome on a Mac.",
				"There's no 4x6 option in the paper size list.",
				"Found the printer preferences, added 4x6. Now it's not cut off but very faint.",
				"Where's the darkness setting on a Mac?",
				"Set it to 22. Better. But every second label is blank now.",
				"Calibrated, works. Should I use ZPL instead?",
				"How do I install the print helper?",
				"Installed. Thanks for sticking with me.",
			},
		},
		{
			Key: "adjustment-dispute-long", Kind: k, Subject: "Adjustments every week",
			Persona: "Annoyed store owner getting weekly carrier adjustments.",
			Goal:    "Understand and stop dimensional weight adjustments.",
			Messages: []string{
				"We got $83 in UPS adjustments this week again. This is getting ridiculous.",
				"They all say dimensions. Our boxes are 12x10x8.",
				"How does UPS calculate dimensional weight?",
				"So a 12x10x8 box is billed at 6 lb even if it weighs 2?",
				"But the adjustments say 14x12x10.",
				"Ugh, the packers switched to a bigger box last month.",
				"How do I update the saved package?",
				"Can I dispute the ones from this week?",
				"How long do disputes take?",
				"OK. Will do, thanks.",
			},
		},
		{
			Key: "sso-broken", Kind: k, Subject: "SSO broken",
			Persona: "IT admin at an Enterprise customer, calm but urgent.", Goal: "Restore SSO sign-in.",
			Messages: []string{
				"Nobody can sign in via SSO since this morning. Error: Signature validation failed. We use Okta.",
				"Yes, Okta rotated the cert today.",
				"I'm the owner but I don't remember my password.",
				"Reset worked. Where do I upload the new metadata?",
				"Uploaded. Test passed. Do I need to do anything else?",
				"Thanks.",
			},
		},
		{
			Key: "tracking-stuck-long", Kind: k, Subject: "Tracking stuck",
			Persona: "Store owner with an anxious customer.", Goal: "Find out what to do about a parcel with no scans.",
			Messages: []string{
				"Tracking for 1ZX4F1E40312345678 has said Label Created for six days.",
				"I handed it to the UPS driver myself last Tuesday.",
				"The customer is asking for a refund.",
				"Can you open a trace?",
				"How long does a trace take?",
				"Should I send a replacement now or wait?",
				"I bought Stowline insurance on it. Can I claim?",
				"OK I'll wait for the trace.",
			},
		},
		{
			Key: "returns-setup-long", Kind: k, Subject: "Setting up returns",
			Persona: "Apparel store owner, patient, setting returns up for the first time.",
			Goal:    "Set up the returns portal with store credit and a return window.",
			Messages: []string{
				"We want to start offering returns through Stowline. Where do I begin?",
				"We're on the Pro plan, so I think we have the portal.",
				"Can customers choose store credit instead of a refund?",
				"Can I give a small bonus for choosing store credit?",
				"How do I set a 30 day return window?",
				"Do customers pay for the return label or do we?",
				"Can we brand the portal with our logo and colors?",
				"Where do I find the link to put on our website?",
				"How will we know when a return comes in?",
				"Great, that's everything for now. Thank you.",
			},
		},
		{
			Key: "rules-setup-long", Kind: k, Subject: "Automation rules",
			Persona: "Operations manager moving from presets to rules, methodical.",
			Goal:    "Replace presets with automation rules that pick the cheapest service.",
			Messages: []string{
				"We still use presets. Should we move to automation rules?",
				"Can a rule pick the cheapest service that arrives within 3 days?",
				"How do I limit a rule to orders under 1 lb?",
				"Can a rule apply a saved package too?",
				"What happens if two rules match the same order?",
				"How do I test a rule before turning it on?",
				"Can I convert our existing presets into rules?",
				"Will the old presets keep working while we switch?",
				"Can a rule split an order into two parcels by SKU tag?",
				"Perfect, we'll set them up this afternoon. Thanks.",
			},
		},
		{
			Key: "international-long", Kind: k, Subject: "Shipping internationally",
			Persona: "Ceramics seller shipping abroad for the first time, careful.",
			Goal:    "Ship the first international orders with correct customs and duties.",
			Messages: []string{
				"We're starting to ship to the UK and the EU. What do we need?",
				"Where do I put the HS code for our mugs?",
				"Can the HS code fill in automatically for each product?",
				"Who pays the duties, us or the customer?",
				"Can we pay the duties up front so customers don't get a bill?",
				"Which carriers do you recommend for small parcels to Germany?",
				"Do I need to print a commercial invoice with DHL Express?",
				"How do returns work for international orders?",
				"What happens if a parcel is held in customs?",
				"Thanks, that covers it.",
			},
		},
		{
			Key: "api-onboarding-long", Kind: k, Subject: "Building on the API",
			Persona: "Backend developer integrating Stowline, precise.",
			Goal:    "Build an order-to-label integration on the API.",
			Messages: []string{
				"I'm integrating our order system with your API. How do I authenticate?",
				"Can I limit a key to only the endpoints we need?",
				"How do I create an order through the API?",
				"Will automation rules run on orders created through the API?",
				"How do I avoid buying a label twice if my request times out and I retry?",
				"What are the rate limits?",
				"How do I get notified when tracking changes?",
				"How do I verify the webhook signature?",
				"Can I resend an event my endpoint missed?",
				"Are amounts strings or numbers in the API?",
				"Great, I have what I need.",
			},
		},
		{
			Key: "warehouse-setup-long", Kind: k, Subject: "New warehouse",
			Persona: "Warehouse lead opening a second location, friendly.",
			Goal:    "Set up a second warehouse with printers, scales and pickups.",
			Messages: []string{
				"We're opening a second warehouse next month. How do we add it?",
				"Can orders route to the closest warehouse automatically?",
				"We'll use thermal printers there too. Which ones work best?",
				"Can a USB scale fill in the weight?",
				"How do we print labels without the print dialog popping up?",
				"Can we schedule daily UPS pickups from the new address?",
				"How do manifests work for USPS from that location?",
				"Can staff there only see that warehouse's orders?",
				"Thanks, very helpful.",
			},
		},
	}
}
