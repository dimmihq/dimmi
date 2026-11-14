/*
|--------------------------------------------------------------------------
| Cost Simulation Scenarios
|--------------------------------------------------------------------------
|
| The customers the cost simulation plays against the agent, written by
| hand for Stowline. They cover the spread that drives cost: a question
| one article answers, questions that need several searches, long
| troubleshooting threads, conversations that call the agent's actions,
| vague and off-topic customers, angry ones who want a person, questions
| the help center does not cover, customers writing in other languages,
| and long messages with pasted logs.
|
*/

package stowline

import (
	"strings"

	"github.com/dimmihq/dimmi/app/ai/costsim"
)

/*
Scenarios returns every scenario.
*/
func Scenarios() []costsim.Scenario {
	var out []costsim.Scenario
	out = append(out, oneAnswerScenarios()...)
	out = append(out, multiSearchScenarios()...)
	out = append(out, troubleshootingScenarios()...)
	out = append(out, actionScenarios()...)
	out = append(out, edgeScenarios()...)
	for i := range out {
		if out[i].Locale == "" {
			out[i].Locale = "en"
		}
	}
	return out
}

func oneAnswerScenarios() []costsim.Scenario {
	k := costsim.KindOneAnswer
	return []costsim.Scenario{
		{
			Key: "void-refund-timing", Kind: k, Subject: "Void refund",
			Persona: "Owner of a small candle shop, friendly, in a hurry.", Goal: "Find out when a voided USPS label is refunded.",
			Messages: []string{"How long does it take to get money back for a voided USPS label?", "Thanks!"},
		},
		{
			Key: "thermal-blank", Kind: k, Subject: "Blank labels",
			Persona: "Warehouse helper, not technical.", Goal: "Fix labels printing blank on a new thermal printer.",
			Messages: []string{"my new rollo printer prints blank labels, what am i doing wrong", "oh ok will try flipping the roll"},
		},
		{
			Key: "plan-allowance", Kind: k, Subject: "Plan question",
			Persona: "Operations manager comparing plans.", Goal: "Learn whether return labels count toward the monthly allowance.",
			Messages: []string{"Do return labels count toward our monthly label allowance on Growth?"},
		},
		{
			Key: "two-factor-team", Kind: k, Subject: "2FA for team",
			Persona: "Account owner tightening security.", Goal: "Require two-factor for the whole team.",
			Messages: []string{"Can I force everyone on my team to use two-factor authentication?", "Great, thank you."},
		},
		{
			Key: "csv-uk", Kind: k, Subject: "CSV import error",
			Persona: "Shop owner importing a spreadsheet.", Goal: "Fix a CSV import that rejects the country column.",
			Messages: []string{"CSV import says country not recognized for UK on every row?"},
		},
		{
			Key: "annual-billing", Kind: k, Subject: "Annual plan",
			Persona: "Finance lead.", Goal: "Understand annual billing.",
			Messages: []string{"If we switch to annual, do we get all 12 months of labels upfront?", "Makes sense."},
		},
		{
			Key: "webhook-signature", Kind: k, Subject: "Webhook signature",
			Persona: "Backend developer, terse.", Goal: "Verify webhook signatures.",
			Messages: []string{"Stowline-Signature never matches our HMAC. We use the secret from the dashboard. What's signed exactly?"},
		},
		{
			Key: "pickup-usps", Kind: k, Subject: "USPS pickup",
			Persona: "Etsy seller.", Goal: "Learn if USPS pickups are free and how late to book.",
			Messages: []string{"Is a USPS pickup free and by when do I need to book it?"},
		},
	}
}

func multiSearchScenarios() []costsim.Scenario {
	k := costsim.KindMultiSearch
	return []costsim.Scenario{
		{
			Key: "migrate-from-other-tool", Kind: k, Subject: "Moving over",
			Persona: "Ops lead moving from another shipping tool, organized, asks several things at once.",
			Goal:    "Learn how to bring saved boxes, rules, UPS rates and a Zebra printer over.",
			Messages: []string{
				"We're moving from another shipping tool. Can we import our saved boxes, do you have something like presets, can we keep our negotiated UPS rates, and will our Zebra ZP450 work?",
				"Can the rules pick the cheapest service that still arrives in 3 days?",
				"And do we need our own USPS account?",
				"Perfect, thanks.",
			},
		},
		{
			Key: "international-setup", Kind: k, Subject: "Shipping to the EU and UK",
			Persona: "Ceramics brand starting to ship abroad.", Goal: "Understand customs, HS codes, IOSS and who pays duties.",
			Messages: []string{
				"We're starting to ship to the EU and UK next month. What do we need to set up for customs? Do we need HS codes for everything?",
				"What's the difference between DDP and DAP and which carriers support DDP?",
				"We have an IOSS number, where does it go?",
			},
		},
		{
			Key: "returns-and-exchanges", Kind: k, Subject: "Returns portal",
			Persona: "Apparel store owner on Pro.", Goal: "Set up returns with exchanges and store credit.",
			Messages: []string{
				"How do I set up the returns portal so customers can exchange sizes, and can I offer store credit with a bonus?",
				"Are we charged for return labels people never use?",
				"Does it work with WooCommerce?",
			},
		},
		{
			Key: "api-integration-plan", Kind: k, Subject: "Building on the API",
			Persona: "Developer at a marketplace building an integration.",
			Goal:    "Plan an integration: auth, idempotency, rate limits, webhooks, pagination.",
			Messages: []string{
				"I'm building a Stowline integration. How should I authenticate, and how do I avoid buying a label twice if our request times out?",
				"What are the rate limits for rating at checkout?",
				"Do webhooks arrive in order? And how do I page through all shipments for a nightly reconciliation?",
			},
		},
		{
			Key: "billing-mixed", Kind: k, Subject: "Billing questions",
			Persona: "Bookkeeper for an online store.", Goal: "Understand balance top-ups, invoices and VAT.",
			Messages: []string{
				"Why are there so many small charges from Stowline on our card, and where do I find invoices with our VAT number on them?",
				"Can invoices be emailed to our accounts address automatically?",
			},
		},
		{
			Key: "dhl-dim-divisor", Kind: k, Subject: "DHL billing weight",
			Persona: "Ceramics brand comparing carriers for bulky parcels.", Goal: "Find DHL Express's dimensional divisor and how a 40 x 30 x 30 cm box is billed.",
			Messages: []string{"How does DHL Express work out the billable weight for a 40 x 30 x 30 cm box that weighs 3 kg? Which divisor do they use?", "And are there surcharges for remote postcodes?"},
		},
		{
			Key: "api-concurrent-update", Kind: k, Subject: "Concurrent updates",
			Persona: "Developer syncing an ERP.", Goal: "Learn how to avoid overwriting a colleague's change when updating an order through the API.",
			Messages: []string{"If two of our systems PATCH the same order at once, which change wins? Is there a way to refuse an update when someone changed it since we read it?", "Does that work for warehouses too?"},
		},
		{
			Key: "fedex-pieces", Kind: k, Subject: "Multi-piece shipments",
			Persona: "Furniture parts seller.", Goal: "Learn how many parcels a FedEx multi-parcel shipment can hold and how labels work.",
			Messages: []string{"We ship one order as many boxes. How many parcels can a FedEx multi-parcel shipment have, and do they share a tracking number?"},
		},
		{
			Key: "carrier-options", Kind: k, Subject: "Which carriers",
			Persona: "UK-based store.", Goal: "Choose between Royal Mail, DPD and DHL Express for different parcels.",
			Messages: []string{
				"We ship from Manchester. What are the weight limits on Royal Mail Tracked 48, does DPD collect daily, and what does DHL need for shipments to Switzerland?",
				"OK and how do pickups work with DPD?",
			},
		},
	}
}

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

func actionScenarios() []costsim.Scenario {
	k := costsim.KindActions
	return []costsim.Scenario{
		{
			Key: "where-is-parcel", Kind: k, Signed: true, Subject: "Where is my parcel",
			Persona: "Store owner checking a parcel.", Goal: "Get the status of parcel 1ZX4F1E40312345678.",
			Messages: []string{"Where is parcel 1ZX4F1E40312345678?", "And when will it arrive?"},
		},
		{
			Key: "two-parcels", Kind: k, Signed: true, Subject: "Two parcels",
			Persona: "Customer service rep at a store.", Goal: "Check two parcels for two customers.",
			Messages: []string{"Can you check 1ZX4F1E40312345678 and 9400111899223344556677 for me?", "Thanks, the second customer is worried, anything I should tell them?"},
		},
		{
			Key: "balance-check", Kind: k, Signed: true, Subject: "My balance",
			Persona: "Owner wondering about charges.", Goal: "See the account's balance, plan and usage.",
			Messages: []string{"How many labels have we used this month and what's our balance?", "Are we close to the limit?"},
		},
		{
			Key: "invoices", Kind: k, Signed: true, Subject: "Invoices",
			Persona: "Bookkeeper.", Goal: "See the last invoices.",
			Messages: []string{"Can you list our last invoices? I need the October one.", "Was it paid?"},
		},
		{
			Key: "void-a-label", Kind: k, Signed: true, Subject: "Void a label",
			Persona: "Warehouse lead who bought a label twice.", Goal: "Void label shp_01JC8Z4M2Q.",
			Messages: []string{"We bought a label twice by mistake, shipment shp_01JC8Z4M2Q. Can you void it?", "Yes please go ahead.", "When does the money come back?"},
		},
		{
			Key: "refund-to-card", Kind: k, Signed: true, Subject: "Balance to card",
			Persona: "Owner closing down a side store.", Goal: "Get $120 of balance refunded to the card.",
			Messages: []string{"Can I get $120 of our Stowline balance back on our card?", "Yes, $120 please.", "Thanks, how long will it take?"},
		},
		{
			Key: "void-then-status", Kind: k, Signed: true, Subject: "Voided label and parcel",
			Persona: "Busy shipper.", Goal: "Check a parcel, then check the balance after a void.",
			Messages: []string{"First, where's 9400111899223344556677? Then, I voided a label yesterday, has the balance changed?", "OK thanks."},
		},
	}
}

func edgeScenarios() []costsim.Scenario {
	logs := strings.Repeat(`2026-11-12T14:02:11Z ERROR request_id=req_7Hc2mQ9d POST /v1/shipments/shp_01JC8Z4M2Q/buy status=424 code=CARRIER_AUTH_FAILED carrier=ups account=A1B2C3 latency_ms=1840
2026-11-12T14:02:11Z WARN retrying in 2s attempt=2
`, 18)
	trace := strings.Repeat(`    at StowlineClient.request (node_modules/@stowline/node/dist/client.js:214:23)
    at async ShipmentsResource.buy (node_modules/@stowline/node/dist/resources/shipments.js:88:16)
    at async fulfilOrder (/srv/app/jobs/fulfill.js:41:5)
`, 12)
	return []costsim.Scenario{
		{
			Key: "vague-help", Kind: costsim.KindVague, Subject: "help",
			Persona: "Customer who writes very little.", Goal: "Something is wrong with shipping, they are not sure what.",
			Messages: []string{"hi", "it doesnt work", "the shipping thing", "labels"},
		},
		{
			Key: "vague-pricing", Kind: costsim.KindVague, Subject: "Question",
			Persona: "Visitor browsing the site.", Goal: "Vaguely wonders if Stowline is expensive.",
			Messages: []string{"is this expensive", "for a small shop"},
		},
		{
			Key: "off-topic-recipe", Kind: costsim.KindVague, Subject: "Random",
			Persona: "Someone who clicked the messenger by accident.", Goal: "Asks something unrelated.",
			Messages: []string{"what's a good banana bread recipe", "lol wrong chat sorry"},
		},
		{
			Key: "angry-double-charge", Kind: costsim.KindWantsHuman, Subject: "CHARGED TWICE",
			Persona: "Angry owner who was charged twice.", Goal: "Talk to a person and get money back.",
			Messages: []string{"You charged me TWICE for the same invoice. I want a human NOW.", "I don't want an article. Get me a person."},
		},
		{
			Key: "angry-lost-parcels", Kind: costsim.KindWantsHuman, Subject: "Lost parcels",
			Persona: "Furious store owner, threatening to cancel.", Goal: "Escalate lost parcels to a manager.",
			Messages: []string{"Three parcels lost this week and nobody helps. I'm canceling unless I talk to a manager today.", "I said a manager."},
		},
		{
			Key: "wants-phone-call", Kind: costsim.KindWantsHuman, Subject: "Call me",
			Persona: "Older owner who prefers the phone.", Goal: "Get a phone call.",
			Messages: []string{"Can someone call me? I'd rather explain on the phone."},
		},
		{
			Key: "uncovered-evri", Kind: costsim.KindUncovered, Subject: "Evri",
			Persona: "UK store.", Goal: "Learn whether Evri is supported.",
			Messages: []string{"Do you support Evri? Half our customers want Evri drop-off.", "Any plans?"},
		},
		{
			Key: "uncovered-crypto", Kind: costsim.KindUncovered, Subject: "Payment",
			Persona: "Customer who wants to pay in crypto.", Goal: "Pay the plan in USDC.",
			Messages: []string{"Can I pay my Stowline plan in USDC?"},
		},
		{
			Key: "uncovered-warehouse-robot", Kind: costsim.KindUncovered, Subject: "Robots",
			Persona: "Large warehouse ops manager.", Goal: "Integrate with an autonomous picking robot vendor.",
			Messages: []string{"Do you integrate with Locus picking robots so labels print at the pack station when a robot arrives?"},
		},
		{
			Key: "german-void", Kind: costsim.KindNonEnglish, Locale: "de", Subject: "Erstattung",
			Persona: "German shop owner writing in German.", Goal: "Learn when a voided DHL label is refunded.",
			Messages: []string{"Hallo, ich habe vor einer Woche ein DHL-Etikett storniert, aber das Geld ist noch nicht zurück. Wie lange dauert das?", "Danke, und kann ich das Geld auf meine Karte bekommen?"},
		},
		{
			Key: "french-print", Kind: costsim.KindNonEnglish, Locale: "fr", Subject: "Impression",
			Persona: "French shop owner writing in French.", Goal: "Fix a cut-off barcode.",
			Messages: []string{"Bonjour, l'étiquette ne s'imprime pas correctement, le code-barres est coupé. J'utilise une imprimante thermique.", "Merci, c'était l'échelle."},
		},
		{
			Key: "spanish-plans", Kind: costsim.KindNonEnglish, Locale: "es", Subject: "Planes",
			Persona: "Spanish shop owner writing in Spanish.", Goal: "Compare Growth and Pro.",
			Messages: []string{"Hola, enviamos unos 2.000 pedidos al mes. ¿Qué plan nos conviene?", "¿Y el portal de devoluciones está incluido?"},
		},
		{
			Key: "portuguese-card", Kind: costsim.KindNonEnglish, Locale: "pt", Subject: "Cartão recusado",
			Persona: "Brazilian shop owner writing in Portuguese.", Goal: "Restore label buying after a declined card.",
			Messages: []string{"Olá, nosso cartão foi recusado e não conseguimos comprar etiquetas. O que fazemos?"},
		},
		{
			Key: "logs-carrier-auth", Kind: costsim.KindLongLogs, Subject: "Errors in our logs",
			Persona: "Developer pasting raw logs.", Goal: "Understand why label purchases fail.",
			Messages: []string{"Our label job is failing since 14:00. Logs:\n\n" + logs + "\nWhat's going on?", "We changed the UPS password this morning. Is that it?"},
		},
		{
			Key: "logs-stacktrace", Kind: costsim.KindLongLogs, Subject: "Node SDK error",
			Persona: "Developer pasting a stack trace.", Goal: "Fix a Node SDK error when buying labels.",
			Messages: []string{"Getting this from the Node library when buying:\n\nStowline.ApiError: RATE_EXPIRED The rate has expired\n" + trace + "\nWe rate in the morning and buy in the afternoon. How should we do it?"},
		},
		{
			Key: "long-story", Kind: costsim.KindLongLogs, Subject: "Long story",
			Persona: "Owner who writes everything in one long message.", Goal: "Get help with several problems described at once.",
			Messages: []string{strings.Repeat("So last week we started using the batch feature for our morning orders and it was great at first, but then on Wednesday half the labels came out with the wrong service because I think one of the automation rules was overriding the other one, and then the customer emails went out with tracking numbers for labels we later voided, and our customers got confused. ", 4) + "What should we change so this doesn't happen again?", "Thanks, which rule wins when two set the service?"},
		},
	}
}
