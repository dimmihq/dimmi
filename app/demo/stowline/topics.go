/*
|--------------------------------------------------------------------------
| Conversation Topics
|--------------------------------------------------------------------------
|
| What Stowline's customers write in about, each topic written by hand
| the way support threads really read: a few ways to open, the team's
| replies in order, the customer's follow-ups, the note a teammate
| leaves for the next one, and how the AI agent answers when it can.
| Placeholders in braces are filled per conversation: {first} is the
| customer's first name, {me} the teammate's, and {order}, {tracking},
| {carrier}, {platform}, {amount}, {plan}, {code}, {shipment} and
| {invoice} the details of their case.
|
*/

package stowline

/*
Topic is one kind of conversation.
*/
type Topic struct {
	Key, Tag, Team, Inbox string
	Weight                int
	Priority              int
	AIResolvable          bool
	Subjects              []string
	Openings              []string
	Replies               []string
	FollowUps             []string
	Closings              []string
	Notes                 []string
	AIAnswers             []string
	Article               string
}

/*
Topics returns every topic with how often it comes up.
*/
func Topics() []Topic {
	return []Topic{
		{
			Key: "void_refund", Tag: "voids", Team: "Billing", Inbox: "Billing", Weight: 9, AIResolvable: true,
			Article:  "Voiding a label and getting your money back",
			Subjects: []string{"Refund for voided label", "Voided {carrier} labels not refunded", "Where is my refund?"},
			Openings: []string{
				"Hi, I voided a {carrier} label on order {order} last week and I still don't see the money back. How long does this take?",
				"We voided about 30 labels after a printer jam on Monday and the balance hasn't gone up. Can you check? One of them is {tracking}.",
				"Hello! I accidentally bought two labels for {order}. I voided one right away but I was charged {amount} for both.",
			},
			Replies: []string{
				"Hi {first}, thanks for the details. I can see the void on {tracking}. {carrier} confirms voids in batches, so the refund is pending on their side right now.",
				"For {carrier} labels the refund lands on your Stowline balance, not your card, once the carrier confirms the label was never scanned. For USPS that is 14 to 21 days after the void.",
				"I checked again this morning: the refund of {amount} is now on your balance. You can see it under Billing, Balance history.",
				"If you'd like the balance paid back to your card instead, I can do that for top-ups from the last 120 days. Just say the word.",
			},
			FollowUps: []string{
				"OK, but it's been 16 days already. Is that normal?",
				"I see the void but not the refund in the balance history.",
				"Can you send it to my card instead? We don't buy USPS labels very often.",
				"Got it. And the other labels from the same batch?",
			},
			Closings: []string{"Perfect, I see it now. Thanks!", "Great, thank you for checking."},
			Notes:    []string{"Void confirmed by carrier, refund batch runs Thursday.", "Customer asked for a card refund; balance top-ups are within 120 days, OK per policy."},
			AIAnswers: []string{
				"Voided {carrier} labels are refunded to your Stowline balance, not your card, once {carrier} confirms the label was never scanned. For USPS that takes 14 to 21 days after the void; for most other carriers it is within 7 days. You can follow it under Billing, Balance history.",
			},
		},
		{
			Key: "missing_tracking", Tag: "tracking", Team: "Support", Inbox: "Support", Weight: 10, AIResolvable: true,
			Article:  "My tracking has not updated",
			Subjects: []string{"Tracking stuck on label created", "Parcel not moving", "No tracking updates for {tracking}"},
			Openings: []string{
				"My customer is asking about {tracking}. It has said Label Created for four days. Did the carrier even get it?",
				"Order {order} hasn't moved since Tuesday. The customer is getting nervous. What can I do?",
				"hi, tracking for {tracking} stopped updating in Memphis 6 days ago. is it lost??",
			},
			Replies: []string{
				"Hi {first}, I looked at {tracking}. The first scan was missed at handover, which happens more than it should. The parcel is usually scanned at the next sorting center.",
				"International and ground parcels can go a few days without a scan while they are on a truck. After 5 business days without a scan I can open a trace with {carrier}.",
				"I've opened a trace with {carrier}, reference TR-{shipment}. They usually reply within 3 business days and I'll update you here.",
				"Good news: {carrier} found it and it was scanned this morning. It's back on track for delivery tomorrow.",
			},
			FollowUps: []string{
				"It's now been 6 business days. Can you open the trace?",
				"The customer wants a refund. If it's lost, can I claim insurance?",
				"Any news from {carrier}?",
				"Should I just send a replacement?",
			},
			Closings: []string{"Delivered! Thanks for chasing it.", "Thank you, I'll let the customer know."},
			Notes:    []string{"Trace opened with carrier, check back Thursday.", "Parcel was misrouted to the wrong hub, carrier confirmed."},
			AIAnswers: []string{
				"\"Label created\" means {carrier} hasn't scanned the parcel yet. That's normal until it's handed over, and a missed first scan usually appears at the next sorting center. If there's no scan after 5 business days for a domestic parcel, open the label and click Report a problem and we'll open a trace with {carrier}.",
			},
		},
		{
			Key: "address_error", Tag: "address-validation", Team: "Support", Inbox: "Support", Weight: 8, AIResolvable: true,
			Article:  "Error ADDRESS_UNVERIFIED: The address could not be verified",
			Subjects: []string{"Address keeps failing", "ADDRESS_UNVERIFIED on a valid address", "Can't buy label - address error"},
			Openings: []string{
				"I keep getting ADDRESS_UNVERIFIED for order {order}. The address is correct, I checked it on Google Maps.",
				"{carrier} says the address can't be found but the customer has ordered from us 5 times before at the same address.",
				"Getting \"{code}\" when buying a label. What does it mean?",
			},
			Replies: []string{
				"Hi {first}, the carrier checks addresses against its own database, which can be behind for new buildings. In this case the apartment number is missing, which is the most common cause.",
				"If you're sure the address is right, open the order and turn on Ship to unverified address. The label will buy and the carrier will deliver it as written.",
				"I tested the address with {carrier}'s validator and their suggestion moves it to the next postal code. Could you confirm with the customer which one is right?",
			},
			FollowUps: []string{
				"There's no apartment number, it's a house.",
				"OK the customer says it's unit 4B. I'll add it.",
				"Is there a way to stop this happening on every order from this town?",
			},
			Closings: []string{"That worked, thanks!", "Label bought. Thank you!"},
			Notes:    []string{"New development, not in carrier database yet. Suggested ship to unverified."},
			AIAnswers: []string{
				"ADDRESS_UNVERIFIED means {carrier} couldn't match the address to a deliverable one in its database, often because of a missing apartment or unit number or a postal code from the next town. Use Validate address on the order to see the suggestion. If you're sure the address is right, turn on Ship to unverified address for that order and buy the label again.",
			},
		},
		{
			Key: "plan_question", Tag: "plans", Team: "Billing", Inbox: "Billing", Weight: 7, AIResolvable: true,
			Article:  "Plans and pricing explained",
			Subjects: []string{"Which plan do I need?", "Question about the label allowance", "Upgrade from {plan}"},
			Openings: []string{
				"We ship about 1,200 orders a month. Is Growth enough or do we need Pro?",
				"Do return labels count toward the monthly allowance?",
				"If I upgrade from {plan} today, am I charged the full month?",
			},
			Replies: []string{
				"Hi {first}! Growth includes 1,500 labels a month, so at 1,200 you have room. Pro starts making sense when you need the returns portal or go above 1,500 regularly.",
				"Return labels only count when they're used. Voided labels never count.",
				"Upgrades are prorated to the day, so you only pay the difference for the rest of this billing period.",
			},
			FollowUps: []string{"And if we go over 1,500 one month?", "Is there a discount for paying yearly?", "Can I downgrade later if it's too much?"},
			Closings:  []string{"Clear, thank you!", "Great, we'll upgrade then."},
			AIAnswers: []string{
				"Growth includes 1,500 labels a month for $49, and labels above that are $0.05 each. Pro includes 7,500 for $149 and adds the returns portal. Only used labels count, voided ones never do, and upgrades are prorated to the day. Annual billing gives two months free.",
			},
		},
		{
			Key: "failed_payment", Tag: "payments", Team: "Billing", Inbox: "Billing", Weight: 5, Priority: 2,
			Article:  "What happens when a payment fails",
			Subjects: []string{"Card declined, can't buy labels", "Payment failed", "Account limited after failed payment"},
			Openings: []string{
				"Our card was declined and now we can't buy any labels. We have 200 orders to ship today!",
				"We got an email saying payment failed for invoice {invoice}. I updated the card, what now?",
				"Why is my account on the Free plan limits? We pay for {plan}.",
			},
			Replies: []string{
				"Hi {first}, sorry about the stress. The top-up for your balance was declined by your bank. Once a new card is saved under Billing, Payment method, click Top up now and label buying comes back within a minute.",
				"I can see the new card. I've retried invoice {invoice} and it went through, so the account is back on {plan}.",
				"Your bank flagged several small top-ups in one day as unusual. Raising the refill amount under Billing, Balance means fewer, larger charges.",
			},
			FollowUps: []string{"Done, it says paid now. Can you check?", "Still can't buy labels.", "Why did the bank block it?"},
			Closings:  []string{"Working again, thanks for the fast help!", "All good now."},
			Notes:     []string{"Bank declined as suspected fraud, customer called bank.", "Manually retried invoice after card update."},
		},
		{
			Key: "shopify_sync", Tag: "integrations", Team: "Integrations", Inbox: "Support", Weight: 9, AIResolvable: true,
			Article:  "{platform} orders are not importing",
			Subjects: []string{"{platform} orders missing", "Orders not syncing from {platform}", "Sync stopped working"},
			Openings: []string{
				"Orders from {platform} stopped coming in this morning. Last one we have is {order}.",
				"Some {platform} orders show up and some don't. Order {order} for example is missing.",
				"We reconnected {platform} yesterday and now we have duplicates of every order.",
			},
			Replies: []string{
				"Hi {first}, thanks for the order number. The sync log shows {order} was skipped because it was already fulfilled in {platform} when we imported it.",
				"This usually happens when automatic fulfillment is on in {platform}. If you turn it off, or include fulfilled orders in the import filter, they'll come through on the next sync.",
				"I've re-run the import for the last 48 hours and the missing orders are now on your Orders page.",
				"The duplicates came from the old connection still being active. I've archived the copies from the old connection; no labels were bought twice.",
			},
			FollowUps: []string{"Where do I find that setting?", "OK changed it. Do I need to do anything for the old orders?", "We still have two orders missing: {order} and the one after it."},
			Closings:  []string{"They're all there now. Thank you!", "Perfect, thanks for fixing the duplicates."},
			Notes:     []string{"Auto-fulfillment enabled by a third-party app the customer installed.", "Reimported 48h window for the customer."},
			AIAnswers: []string{
				"The most common reason {platform} orders don't import is that they don't match the import filter: orders that are unpaid, on hold or already fulfilled are skipped. Check Settings, Integrations, {platform} to see when the last sync ran and why orders were skipped, and click Sync now to import straight away.",
			},
		},
		{
			Key: "woocommerce_401", Tag: "integrations", Team: "Integrations", Inbox: "Support", Weight: 4,
			Article:  "Connecting WooCommerce",
			Subjects: []string{"WooCommerce connection test fails with 401", "Can't connect WooCommerce", "REST API error when connecting"},
			Openings: []string{
				"Trying to connect our WooCommerce store and the test keeps failing with 401 Unauthorized. Keys are read/write.",
				"WooCommerce connection worked for months and today it says unauthorized.",
			},
			Replies: []string{
				"Hi {first}, a 401 with correct keys almost always means something in front of WordPress blocks the REST API, usually a security plugin or the host's firewall.",
				"Could you check whether Wordfence or a similar plugin is installed? The /wp-json/wc/v3 routes need to be allowed.",
				"Thanks. Another cause is permalinks set to Plain, which breaks the REST routes. Settings, Permalinks, anything but Plain fixes it.",
			},
			FollowUps: []string{"We use Wordfence. What exactly do I allow?", "Permalinks are on Post name already.", "Our host says they don't block anything."},
			Closings:  []string{"It connected after allowlisting! Thanks.", "Fixed, it was the host firewall after all."},
			Notes:     []string{"Host had ModSecurity rule blocking Authorization header."},
		},
		{
			Key: "api_rate_limit", Tag: "api", Team: "Integrations", Inbox: "Developers", Weight: 4, AIResolvable: true,
			Article:  "Rate limits and retries",
			Subjects: []string{"429 errors from the API", "Rate limit on /v1/rates", "API throttling during peak"},
			Openings: []string{
				"We're getting a lot of 429 RATE_LIMITED on /v1/rates during checkout peaks. What is the limit exactly?",
				"Our batch job buys labels one by one and gets throttled after a few hundred. Is there a bulk option?",
			},
			Replies: []string{
				"Hi {first}, the rates endpoint has its own limit of 20 requests a second because every call goes to the carriers. The general limit is 100 a second.",
				"For checkout, caching quotes for the same route and parcel for up to an hour usually cuts calls by 80 to 90 percent.",
				"For bulk purchases, use POST /v1/batches. A batch of up to 1,000 counts as one request and runs in the background.",
			},
			FollowUps: []string{"Can you raise the limit for our account?", "Does the batch endpoint support idempotency keys?", "How do I know when the batch is done?"},
			Closings:  []string{"Batches it is, thanks.", "Caching fixed it, thank you."},
			AIAnswers: []string{
				"The API allows 100 requests a second per account, with a separate limit of 20 a second on rating and 10 on address verification. Requests over the limit get 429 with a Retry-After header. For checkout quotes, cache by route and parcel for up to an hour; for bulk label buying, use POST /v1/batches, which takes up to 1,000 shipments as one request.",
			},
		},
		{
			Key: "webhook_signature", Tag: "api", Team: "Integrations", Inbox: "Developers", Weight: 3, AIResolvable: true,
			Article:  "Verifying webhook signatures",
			Subjects: []string{"Webhook signature never matches", "Stowline-Signature verification failing"},
			Openings: []string{
				"Our webhook signature check fails for every delivery. We're using the secret from the dashboard and HMAC SHA256.",
				"How do I verify the Stowline-Signature header in Node? Express gives me a parsed body.",
			},
			Replies: []string{
				"Hi {first}, the signature is computed over the timestamp, a dot and the raw body. If the framework parses the JSON first, the bytes change and the HMAC can't match.",
				"In Express, use express.raw({type: 'application/json'}) on the webhook route so you get the exact bytes.",
			},
			FollowUps: []string{"That was it, we were hashing the parsed body. Do you sign retries again?", "Do you send the old secret during rotation?"},
			Closings:  []string{"Works now, thanks!"},
			AIAnswers: []string{
				"The Stowline-Signature header has a timestamp t and a signature v1. Compute HMAC-SHA256 of t + \".\" + the raw request body with your endpoint's secret and compare it with v1. The usual reason it never matches is that the framework parsed the JSON before you hashed it; use the raw body exactly as received.",
			},
		},
		{
			Key: "adjustment_dispute", Tag: "adjustments", Team: "Billing", Inbox: "Billing", Weight: 6,
			Article:  "Carrier adjustments and disputes",
			Subjects: []string{"Unexpected adjustment charge", "Dispute {carrier} dimensional weight charge", "Why was I charged {amount} extra?"},
			Openings: []string{
				"We got charged {amount} in adjustments from {carrier} for last week. The boxes were weighed. This is ridiculous.",
				"Can you explain adjustment ADJ-{shipment}? The carrier says 18x14x10 but our box is 12x10x8.",
			},
			Replies: []string{
				"Hi {first}, I understand, adjustments are frustrating. This one is a dimensional weight adjustment: {carrier} measured the box larger than the package on the label.",
				"You can dispute within 30 days. A photo of the box with a tape measure helps a lot. I can file it for you if you send the photo here.",
				"I've filed the dispute with {carrier}. They decide most disputes within 15 business days and if they agree, the amount goes back on your balance.",
				"{carrier} accepted the dispute and {amount} is back on your balance.",
			},
			FollowUps: []string{"Here's the photo. Can you file it?", "This happens every week. How do we stop it?", "Any update on the dispute?"},
			Closings:  []string{"Thanks for pushing it through.", "OK, we'll update the box sizes."},
			Notes:     []string{"Customer's saved package is 12x10x8 but they ship in a 14x12x10 box. Explained."},
		},
		{
			Key: "print_problem", Tag: "printing", Team: "Support", Inbox: "Support", Weight: 7, AIResolvable: true,
			Article:  "Printing labels on a thermal printer",
			Subjects: []string{"Labels printing blank", "Label cut off on thermal printer", "Printer prints every other label blank"},
			Openings: []string{
				"New Rollo printer, labels come out completely blank. Tried 3 times.",
				"Our labels are cut off at the bottom, the barcode is half missing.",
				"Every second label comes out blank on our Zebra.",
			},
			Replies: []string{
				"Hi {first}, blank labels almost always mean the roll is in upside down: thermal labels only darken on one side. Scratch the surface with a fingernail; the side that turns dark should face the print head.",
				"Cut-off labels come from the print scale. In the print dialog set paper to 4x6 and scale to 100% or Actual size, not Fit to page.",
				"Blank labels in between mean the printer hasn't calibrated to the gaps. Hold the feed button until it feeds two labels.",
			},
			FollowUps: []string{"Flipped it and it works. But now they're very faint.", "I don't have a 4x6 option in the paper size list.", "Still blank after calibrating."},
			Closings:  []string{"All good now, thanks!", "Printing perfectly, thank you."},
			AIAnswers: []string{
				"Blank labels on a thermal printer usually mean the roll is upside down, because thermal labels only darken on one side. If labels are cut off, print at 4x6 paper size with scale at 100% or Actual size, not Fit to page. If every other label is blank, hold the feed button until it feeds two labels so the printer calibrates to the gaps.",
			},
		},
		{
			Key: "customs", Tag: "international", Team: "Support", Inbox: "Support", Weight: 5, AIResolvable: true,
			Article:  "Customs forms and international shipping",
			Subjects: []string{"HS codes required?", "Customs form for UK orders", "DHL refusing label - commodity code"},
			Openings: []string{
				"DHL refuses every label to the UK with \"commodity code missing\". We never needed this before.",
				"Do I need to fill customs for every international order? We have 400 products.",
				"Customer in Germany was charged VAT again on delivery. We have IOSS.",
			},
			Replies: []string{
				"Hi {first}, DHL Express now requires a six-digit HS code on every item of a dutiable shipment. If you save the code on each product it fills in automatically.",
				"For 400 products the fastest way is the product CSV export: add the hs_code and origin_country columns and import it back.",
				"For the VAT charge, check that your IOSS number is saved under Settings, Customs. It's only sent on declarations created after it's saved.",
			},
			FollowUps: []string{"Where do I find HS codes for candles?", "Is the IOSS number sent for Etsy orders too?", "The import says row 12 has an invalid code."},
			Closings:  []string{"Imported, all labels buy now. Thanks!"},
			AIAnswers: []string{
				"Every international parcel needs a customs declaration with a clear description, quantity, value, country of origin and a six-digit HS code for each item; DHL Express refuses dutiable shipments without HS codes. Save the HS code and origin on your products under Products and they fill in automatically on every order.",
			},
		},
		{
			Key: "returns_portal", Tag: "returns", Team: "Support", Inbox: "Support", Weight: 5, AIResolvable: true,
			Article:  "Setting up the returns portal",
			Subjects: []string{"Returns portal setup", "Customer can't start a return", "Return label charged but not used?"},
			Openings: []string{
				"A customer says the returns portal tells them the order can't be returned, but it's within 30 days.",
				"Are we charged for return labels that customers never use?",
				"How do I add store credit as an option in the returns portal?",
			},
			Replies: []string{
				"Hi {first}, the portal counts the window from delivery, and this order has a product tagged final-sale, which your rules exclude.",
				"Return labels are only charged when the carrier first scans them, so unused ones cost nothing.",
				"Store credit is under Returns, Settings, Resolution options. It works with Shopify and BigCommerce and can add a bonus.",
			},
			FollowUps: []string{"Can I override it for this one customer?", "Does store credit work with WooCommerce?"},
			Closings:  []string{"Thanks, sorted.", "Great, thanks for explaining."},
			AIAnswers: []string{
				"The returns portal lets customers start a return within your rules: the window counted from delivery, products excluded by tag or SKU, and reasons. Return labels are only charged when the carrier first scans them, so unused ones cost nothing. You can create a return by hand from any order to make an exception.",
			},
		},
		{
			Key: "carrier_connect", Tag: "carriers", Team: "Support", Inbox: "Support", Weight: 6, AIResolvable: true,
			Article:  "Connecting your {carrier} account",
			Subjects: []string{"Can't connect {carrier} account", "{carrier} account pending", "{carrier} rates not showing"},
			Openings: []string{
				"I connected our {carrier} account but no rates show up when I click Get rates.",
				"{carrier} says \"account pending\" on every label for two days now.",
				"How do I connect a second {carrier} account for our other warehouse?",
			},
			Replies: []string{
				"Hi {first}, the connection is active but it's limited to your Brooklyn warehouse, and this order ships from New Jersey. Under the connection's Warehouses setting you can add the other one.",
				"{carrier} checks every new account before the first label. It normally finishes within a business day; I've asked them to look at yours.",
				"Connect it again from Settings, Carriers, Add carrier, and limit each connection to its warehouse.",
			},
			FollowUps: []string{"Added the warehouse and now I see rates. Are these our negotiated rates?", "Still pending today."},
			Closings:  []string{"Works now, thanks!"},
			AIAnswers: []string{
				"To connect {carrier}, go to Settings, Carriers, Add carrier, enter the account details and accept {carrier}'s agreement. If rates don't show, check that the connection isn't limited to another warehouse. Some carriers check new accounts before the first label, which can take up to a business day.",
			},
		},
		{
			Key: "batch_failures", Tag: "batches", Team: "Support", Inbox: "Support", Weight: 4,
			Article:  "Batch shipping: buy hundreds of labels at once",
			Subjects: []string{"Batch stuck at 60%", "Half the batch failed", "Batch labels missing from PDF"},
			Openings: []string{
				"Our morning batch of 340 orders has been at 61% for 40 minutes.",
				"Batch {shipment} says 27 failed. What do I do with them?",
			},
			Replies: []string{
				"Hi {first}, {carrier} had a slowdown this morning and the batch is retrying the failed requests. Nothing will be bought twice.",
				"The 27 failures are address errors. If you fix them in the batch and click Retry failed, only those are bought.",
			},
			FollowUps: []string{"It's done now but the PDF only has 313 labels.", "Can I print the successful ones now and the rest later?"},
			Closings:  []string{"Got them all, thanks."},
			Notes:     []string{"Carrier latency incident 8:10-9:05, batches recovered automatically."},
		},
		{
			Key: "sso_setup", Tag: "sso", Team: "Enterprise", Inbox: "Enterprise", Weight: 2, Priority: 2,
			Article:  "Single sign-on with SAML",
			Subjects: []string{"SAML: signature validation failed", "Okta SSO setup", "SSO broke after certificate rotation"},
			Openings: []string{
				"Nobody on our team can sign in since this morning. Error: Signature validation failed. We use Okta.",
				"Setting up SAML with Entra ID. What do we put as the NameID?",
			},
			Replies: []string{
				"Hi {first}, that error right after a morning change usually means the identity provider rotated its signing certificate. Uploading the new metadata under Settings, Security, Single sign-on fixes it.",
				"As the owner you can still sign in with your password as a break-glass account to upload it.",
				"NameID should be the user's email address. If that isn't possible, send an attribute called email.",
			},
			FollowUps: []string{"Uploaded the new metadata, testing now.", "Can we keep both certificates during the switch?"},
			Closings:  []string{"Everyone is back in. Thanks for the quick turnaround."},
			Notes:     []string{"Enterprise account, escalated per SLA."},
		},
		{
			Key: "api_auth", Tag: "api", Team: "Integrations", Inbox: "Developers", Weight: 3, AIResolvable: true,
			Article:  "Authenticating with the API",
			Subjects: []string{"401 with a new API key", "FORBIDDEN on shipments endpoint", "Test vs live keys"},
			Openings: []string{
				"New API key returns 403 FORBIDDEN when we call POST /v1/shipments/{id}/buy.",
				"Our test key works but the live key says UNAUTHORIZED.",
			},
			Replies: []string{
				"Hi {first}, a 403 means the key is valid but lacks a scope. Buying needs shipments:write; the message names the missing scope.",
				"A live key returning 401 has usually been revoked or has a stray space when pasted. Keys are shown once, so if in doubt create a new one.",
			},
			FollowUps: []string{"Can I add a scope to an existing key?"},
			Closings:  []string{"Created a new key with the right scopes, works now."},
			AIAnswers: []string{
				"A 401 UNAUTHORIZED means the key is missing, mistyped or revoked; a 403 FORBIDDEN means the key is valid but lacks the scope the operation needs, and the message names it. Buying labels needs shipments:write. Scopes are set when a key is created, so create a new key with the scopes you need and revoke the old one.",
			},
		},
		{
			Key: "cancel_account", Tag: "cancellation", Team: "Billing", Inbox: "Billing", Weight: 3, Priority: 1,
			Article:  "Changing or canceling your plan",
			Subjects: []string{"Cancel my subscription", "Please close our account", "Downgrade to free"},
			Openings: []string{
				"We're closing the shop at the end of the month. How do I cancel and get our balance back?",
				"Please cancel our {plan} plan. We moved to another tool.",
			},
			Replies: []string{
				"Hi {first}, sorry to see you go. Canceling from Billing, Plan moves you to Free at the end of the period, and the history stays. Your balance of {amount} can be refunded to your card.",
				"I've processed the balance refund; it shows on your card in 5 to 10 days.",
				"If anything about Stowline made you switch, I'd really like to hear it. It goes straight to the product team.",
			},
			FollowUps: []string{"Can you refund this month's plan too? We barely used it."},
			Closings:  []string{"Thanks for making it easy."},
			Notes:     []string{"Churn reason: moved to 3PL with own shipping software."},
		},
		{
			Key: "lost_claim", Tag: "claims", Team: "Support", Inbox: "Support", Weight: 4,
			Article:  "Insurance: what it covers and how to file a claim",
			Subjects: []string{"Insurance claim for lost parcel", "Claim status {shipment}", "Parcel damaged, insured"},
			Openings: []string{
				"Parcel {tracking} has been stuck for 3 weeks. It was insured for {amount}. How do I claim?",
				"Customer received a smashed box, the ceramics are broken. We had Stowline insurance.",
			},
			Replies: []string{
				"Hi {first}, I'm sorry. Open the label and click File a claim. You'll need the invoice showing what the items cost you, and for damage, photos of the item and the box.",
				"I can see the claim. It's with the claims team; they decide within 10 business days.",
				"The claim was approved and {amount} has been paid to your balance.",
			},
			FollowUps: []string{"Submitted. Do I need to send the box back?", "It's been 8 days, any update?"},
			Closings:  []string{"Thanks, that's a relief."},
		},
		{
			Key: "multi_issue", Tag: "onboarding", Team: "Support", Inbox: "Support", Weight: 4,
			Article:  "Welcome to Stowline: ship your first order in 15 minutes",
			Subjects: []string{"Getting set up - a few questions", "New account, lots of questions", "Moving over from another tool"},
			Openings: []string{
				"Hi! We just moved from another shipping tool. Few questions: can we import our saved boxes, do automation rules work like presets, and can we keep our UPS rates?",
				"Just signed up. How do I get my {platform} orders in and print to my Zebra?",
			},
			Replies: []string{
				"Welcome {first}! Saved boxes can be imported by CSV under Settings, Packages, Import. Automation rules are like presets but apply automatically when orders arrive.",
				"Your UPS rates come with your account: connect it under Settings, Carriers and the negotiated prices show up.",
				"For the Zebra, choose 4x6 ZPL and install the print helper; labels print as soon as they're bought.",
			},
			FollowUps: []string{"Can rules pick the cheapest service?", "What about our USPS rates, do we need an account?", "The print helper doesn't see the printer."},
			Closings:  []string{"This is great, thanks for the walkthrough!"},
		},
		{
			Key: "slack_enterprise", Tag: "enterprise", Team: "Enterprise", Inbox: "Enterprise", Weight: 3, Priority: 2,
			Article:  "Batch shipping: buy hundreds of labels at once",
			Subjects: []string{"Peak season capacity", "Labels failing for all FedEx orders", "Need a call about our renewal"},
			Openings: []string{
				"Hey team, FedEx labels are failing across all three warehouses since about 10:40. Error {code}. This is blocking our afternoon pickup.",
				"Heads up: we expect 4x volume from Black Friday to Cyber Monday. Anything we should do on our side?",
			},
			Replies: []string{
				"Thanks {first}, looking now. We see elevated FedEx errors across accounts; FedEx has an incident open. Batches will retry automatically.",
				"FedEx recovered at 11:22 and your queued labels are buying now. Your 3 p.m. pickup is still on.",
				"For peak, moving big batches before 9 a.m. local and raising the balance refill avoids most problems. Your success manager will reach out to plan.",
			},
			FollowUps: []string{"Can we get a status update every 30 minutes until fixed?", "Can we get a temporary higher API limit for that week?"},
			Closings:  []string{"Appreciate the quick response as always."},
			Notes:     []string{"FedEx incident INC-2291, comms sent to all enterprise accounts."},
		},
		{
			Key: "feature_request", Tag: "feature-request", Team: "Support", Inbox: "Support", Weight: 4,
			Subjects: []string{"Feature request: split shipments by SKU", "Can you add Evri?", "Request: bulk edit weights"},
			Openings: []string{
				"Would love the ability to bulk edit weights on the orders page. We have to open each order today.",
				"Any plans to support Evri? Half our UK customers want it.",
				"Can automation rules split an order into two parcels by SKU?",
			},
			Replies: []string{
				"Thanks for the idea, {first}! I've added it to our feedback board with your use case, and you'll get an email if it ships.",
				"For now, the CSV import can update weights in bulk: export, change the column, import again.",
			},
			FollowUps: []string{"Is there a rough timeline?"},
			Closings:  []string{"Thanks, will use the CSV for now."},
		},
		{
			Key: "pickup_missed", Tag: "pickups", Team: "Support", Inbox: "Support", Weight: 3,
			Subjects: []string{"{carrier} driver didn't show up", "Missed pickup today", "Pickup confirmation but no driver"},
			Openings: []string{
				"We booked a {carrier} pickup for today 1-5pm and nobody came. 60 parcels waiting.",
				"Pickup {shipment} was confirmed but canceled by {carrier} without telling us?",
			},
			Replies: []string{
				"Hi {first}, I'm sorry. I've reported the missed pickup to {carrier} and asked for a pickup tomorrow morning. The labels stay valid.",
				"{carrier} says the pickup was canceled because the ready time was after their route cut-off. For your area, a ready time before 2 p.m. avoids it.",
			},
			FollowUps: []string{"Can they come tonight instead?", "Will tracking show the wrong ship date?"},
			Closings:  []string{"Driver came this morning, thanks."},
		},
		{
			Key: "how_to_rules", Tag: "automation", Team: "Support", Inbox: "Support", Weight: 5, AIResolvable: true,
			Article:  "Setting up automation rules",
			Subjects: []string{"Automation rule not applying", "How do I set up a rule for international orders?", "Rule picks wrong service"},
			Openings: []string{
				"I made a rule: orders over $250 get insurance. It doesn't apply to new orders.",
				"How do I make a rule that sends everything to Canada with Canada Post Expedited?",
				"Two of my rules conflict and the wrong service wins. Which one is applied?",
			},
			Replies: []string{
				"Hi {first}, rules only apply to orders imported after they're saved. For orders already waiting, use Reapply rules on the Orders page.",
				"Rules run top to bottom and every matching rule applies, so the lower one wins when two set the same thing. Drag the one you want to win to the bottom.",
			},
			FollowUps: []string{"Reapplied and it worked. Can a rule look at the product tags?"},
			Closings:  []string{"Thanks!"},
			AIAnswers: []string{
				"Automation rules apply to new orders the moment they're imported, so orders already waiting need Reapply rules on the Orders page. Rules run top to bottom and every matching rule applies, so when two set the same thing the lower one wins. Use Test on the rule with an order number to see which conditions match.",
			},
		},
		{
			Key: "two_factor", Tag: "account-access", Team: "Support", Inbox: "Support", Weight: 3,
			Article:  "Turning on two-factor authentication",
			Subjects: []string{"Lost my phone, can't log in", "Reset 2FA please", "Locked out of account"},
			Openings: []string{
				"I got a new phone and lost my authenticator. I'm the account owner. Can you reset 2FA?",
				"A teammate left and we need to reset their 2FA so the new person can use the account.",
			},
			Replies: []string{
				"Hi {first}, if you saved recovery codes, any one of them gets you in. Otherwise, as the owner, we need to verify ownership: please reply from the email address on the account.",
				"Thanks, ownership is verified and two-factor is reset. You'll be asked to set it up again at your next sign-in.",
				"For a teammate, an admin can reset their two-factor from Settings, Team. But each person should have their own login rather than sharing one.",
			},
			FollowUps: []string{"Replying from the account email now."},
			Closings:  []string{"I'm back in. Thanks a lot."},
		},
	}
}

/*
foreignOpenings are first messages from customers who write in their
own language, with the follow-up they send next.
*/
func foreignOpenings() map[string][][2]string {
	return map[string][][2]string{
		"de": {
			{"Hallo, ich habe vor einer Woche ein DHL-Etikett storniert, aber das Geld ist noch nicht zurück. Wie lange dauert das?", "Danke, jetzt sehe ich es im Guthaben."},
			{"Unsere Bestellungen aus Shopify kommen seit heute Morgen nicht mehr an. Können Sie das prüfen?", "Super, jetzt sind alle da. Vielen Dank!"},
			{"Brauche ich für Sendungen in die Schweiz eine Zollerklärung?", "Alles klar, danke für die Erklärung."},
		},
		"fr": {
			{"Bonjour, l'étiquette pour la commande {order} ne s'imprime pas correctement, le code-barres est coupé.", "Merci, c'était bien l'échelle d'impression."},
			{"Bonjour, comment connecter notre compte Colissimo ? Je ne le trouve pas dans la liste.", "D'accord, merci pour la réponse."},
			{"Le suivi de mon colis est bloqué depuis cinq jours, que dois-je faire ?", "Merci beaucoup, le client est rassuré."},
		},
		"es": {
			{"Hola, me cobraron dos veces la etiqueta del pedido {order}. ¿Pueden revisarlo?", "Perfecto, ya veo el reembolso. Gracias."},
			{"¿Cómo puedo cambiar de plan? Queremos pasar a Pro.", "Genial, gracias."},
		},
		"pt": {
			{"Olá, nosso cartão foi recusado e não conseguimos comprar etiquetas. Podem ajudar?", "Funcionou, muito obrigado!"},
			{"Os pedidos do Shopify não estão sincronizando desde ontem.", "Agora apareceram todos, obrigado."},
		},
		"nl": {
			{"Hallo, DPD wijst het afleveradres van de klant steeds af. Wat kan ik doen?", "Gelukt, bedankt voor de hulp!"},
		},
	}
}

/*
smallTalk are the extra exchanges of long threads, when the team and
the customer go back and forth checking things.
*/
func smallTalk() [][2]string {
	return [][2]string{
		{"Could you send me a screenshot of what you see on the order page?", "Here you go. It's the red banner at the top."},
		{"Thanks. Which browser are you using, and does it happen in a private window too?", "Chrome. Same thing in a private window."},
		{"I've asked our engineers to look at the request logs for this order. I'll be back within the hour.", "OK, thanks. We have a pickup at 4 so the sooner the better."},
		{"Can you try once more now? We pushed a fix for the request that was failing.", "Tried again, same error unfortunately."},
		{"Understood. Could you tell me the exact time you last tried, with your time zone?", "Just now, 2:47 pm Eastern."},
		{"Thank you, that helped us find it. The carrier's test environment was answering instead of production for your connection.", "Weird. Is that something we changed?"},
		{"No, nothing on your side. We've switched it back. Can you try one more time?", "That worked!"},
		{"Is there anything else on this order you'd like me to check?", "Actually yes, the packing slip shows the old logo."},
		{"The packing slip template caches the logo for a day. I've cleared it for your account.", "Perfect, new logo is there."},
		{"Let me check with the billing team and come back to you.", "Sure, thanks."},
		{"Quick update: still waiting on the carrier, they promised an answer by end of day.", "Thanks for keeping me posted."},
		{"I can see the same thing on our side. Let me reproduce it with a test order.", "OK. Let me know if you need access to anything."},
	}
}
