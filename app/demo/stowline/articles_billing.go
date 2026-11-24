/*
|--------------------------------------------------------------------------
| Hand-Written Articles: Billing, Account and Returns
|--------------------------------------------------------------------------
|
| The questions customers ask most when money, access or a returned
| parcel is involved, and two pages only the agent reads: the policies
| support follows when a customer asks for an exception.
|
*/

package stowline

import "fmt"

func billingArticles() []Article {
	var rows [][]string
	for _, p := range Plans() {
		price := "Free"
		switch {
		case p.ByContract:
			price = "Priced per contract"
		case p.MonthlyUSD > 0:
			price = fmt.Sprintf("$%d a month", p.MonthlyUSD)
		}
		over := "No overage; buying stops at the limit"
		if p.OverageCents > 0 {
			over = fmt.Sprintf("$0.%02d per extra label", p.OverageCents)
		}
		rows = append(rows, []string{p.Name, price, fmt.Sprintf("%d", p.Labels), fmt.Sprintf("%d", p.Seats), over})
	}

	return []Article{
		{
			Collection: "billing", Title: "Plans and pricing explained",
			Description: "What each plan includes and how label allowances work.",
			Body: `Stowline charges a monthly plan fee. The plan sets how many labels you can buy each month without extra charges, how many teammates you can add and which features you get. The postage itself is billed separately at your carrier rate.

` + table([]string{"Plan", "Price", "Labels a month", "Seats", "Over the allowance"}, rows) + `
Annual billing gives two months free on Growth and Pro.

## What counts as a label

Every label you buy counts once, including return labels that are used. Voided labels do not count. Labels for a multi-parcel shipment count once per parcel.

## Features by plan

- **Free**: USPS discounted rates, one store, manual labels, tracking emails.
- **Growth**: every carrier, unlimited stores, automation rules, batches, branded tracking page, API access.
- **Pro**: everything in Growth, plus the returns portal, custom packing slips, insurance automation, priority support.
- **Enterprise**: everything in Pro, plus single sign-on, custom roles, audit log, a dedicated success manager and a 99.9% uptime commitment.

## When the allowance resets

Allowances reset on your billing date each month. Unused labels do not roll over.`,
		},
		{
			Collection: "billing", Title: "How postage is billed: Stowline balance and carrier accounts",
			Description: "Who charges you for each label and when.",
			Body: `There are two ways the postage for a label is paid, and an account can use both.

## Labels billed through Stowline

USPS labels on Stowline's rates, and labels from any carrier you have not connected your own account for, are paid from your **Stowline balance**. The balance is topped up from your card automatically: when it drops below $25 we charge your card to bring it back to the refill amount you chose, $100 by default.

You can change the threshold and the refill amount under **Billing → Balance**. Stores that ship a lot often raise the refill so they are charged a few times a week instead of several times a day.

## Labels billed by your carrier

When you connect your own UPS, FedEx, DHL Express or other account, the carrier bills you directly on its usual invoice. Stowline only charges the plan fee and any overage for those labels.

## Receipts

Every balance top-up has a receipt under **Billing → Balance history**. The monthly plan invoice is under **Billing → Invoices**. Both can be emailed automatically to an accounts address you set under **Billing → Billing contacts**.

## Why there are many small charges on my card

Each top-up is a separate card charge. If your bank flags them, raise the refill amount so there are fewer, larger charges.`,
		},
		{
			Collection: "billing", Title: "Downloading invoices and receipts",
			Description: "Find invoices, add your company details and VAT number.",
			Body: `Plan invoices are issued on your billing date each month. Balance top-ups each get their own receipt.

## Where to find them

- **Billing → Invoices**: monthly plan invoices, overage and seat charges. Download each as a PDF.
- **Billing → Balance history**: every top-up, label purchase, refund and adjustment. Export the history as a CSV for any date range.

## Adding company details

Your company name, address and tax ID appear on invoices issued after you save them under **Billing → Company details**. We cannot reissue past invoices with new details, but we can send you a letter confirming the change for your accountant.

## VAT and sales tax

Stowline charges VAT to customers in the EU and UK unless you provide a valid VAT number, in which case the reverse charge applies. US sales tax is charged in the states where software subscriptions are taxable. Tax appears as a separate line on each invoice.

## Sending invoices to your accounts team

Add up to five addresses under **Billing → Billing contacts**. They get every invoice and receipt by email without needing a Stowline login.`,
		},
		{
			Collection: "billing", Title: "What happens when a payment fails",
			Description: "Retries, the grace period and how to restore label buying.",
			Body: `If your card is declined for a plan invoice or a balance top-up, here is what happens and what to do.

## Plan invoices

We retry the card on days 1, 3, 5 and 7 after the failed charge and email the account owner and billing contacts each time. During these seven days everything keeps working. If the last retry fails, the account moves to the Free plan's limits until the invoice is paid: you can still print labels already bought, but not buy new ones on other carriers.

## Balance top-ups

A failed top-up is retried once an hour for a day. While your balance is below zero you cannot buy labels billed through Stowline. Labels on your own carrier accounts still work.

## Fixing it

` + steps(
				"Go to **Billing → Payment method** and add a new card or update the expiry date.",
				"Click **Pay now** on the unpaid invoice, or **Top up now** on the balance page.",
				"The account returns to normal within a minute of a successful payment.",
			) + `
## Common reasons for a decline

- The card expired or was replaced by the bank.
- The bank blocked the charge as unusual. Many small top-ups in a day look unusual; raise the refill amount.
- 3-D Secure confirmation was needed and nobody approved it. Charges made while you are away cannot be approved, so ask your bank to whitelist Stowline.`,
		},
		{
			Collection: "billing", Title: "Changing or canceling your plan",
			Description: "Upgrades, downgrades, proration and closing the account.",
			Body: `You can change plan at any time from **Billing → Plan**.

## Upgrading

An upgrade starts immediately. You are charged the difference for the rest of the current billing period, prorated to the day, and the higher label allowance applies straight away.

## Downgrading

A downgrade takes effect at the end of the current billing period, so you keep what you paid for. If the lower plan has fewer seats than you use, you will be asked to remove teammates before the change. Features the lower plan does not include, such as the returns portal, stop working on the change date; their settings are kept in case you upgrade again.

## Canceling

Canceling moves you to the Free plan at the end of the period. Your order history, labels and tracking pages stay available. To delete the account and all its data instead, see *Deleting your account*.

## Refunds

Plan fees are not refunded for partial months. If you were charged for a plan you did not mean to keep, write to us within 14 days of the charge and we will look at it. Your **Stowline balance** is never lost: unused balance can be refunded to your card on request when you cancel.`,
		},
		{
			Collection: "billing", Title: "Carrier adjustments and disputes",
			Description: "Why you were charged after the label was bought, and how to dispute it.",
			Body: `An adjustment is an extra charge a carrier sends after it has handled a parcel. Stowline passes adjustments on for labels billed through your Stowline balance.

## The usual reasons

` + table([]string{"Reason", "What happened"}, [][]string{
				{"Weight", "The carrier's scale read more than the weight on the label."},
				{"Dimensions", "The parcel was bigger than the package on the label, so dimensional weight went up."},
				{"Residential", "The address was a home, not a business."},
				{"Address correction", "The carrier corrected the address to deliver it."},
				{"Service change", "The parcel did not qualify for the service bought, for example Media Mail."},
			}) + `
## Seeing an adjustment

Go to **Billing → Adjustments**. Each line shows the order, the label, what was on the label, what the carrier measured and the amount.

## Disputing

You can dispute an adjustment within 30 days if you think the carrier was wrong. Click **Dispute**, explain why and attach a photo of the parcel on a scale with the tape measure visible if you have one. We send the dispute to the carrier; carriers decide most disputes within 15 business days. If the carrier agrees, the amount is credited back to your balance.

To prevent most adjustments, weigh every parcel after packing it and keep your saved box sizes up to date.`,
		},
		{
			Collection: "billing", Title: "Annual billing",
			Description: "Pay yearly and get two months free.",
			Body: `Growth and Pro can be paid yearly at ten times the monthly price, which is two months free. Enterprise contracts are yearly by default.

## Switching to annual

Go to **Billing → Plan → Switch to annual**. You are charged for twelve months straight away, minus a credit for the unused days of your current month.

## Label allowance on annual plans

The label allowance stays monthly and resets each month on your billing day. An annual plan does not give you twelve months of labels to use whenever you like.

## Switching back to monthly

You can switch back to monthly at the end of the annual term. Changing earlier is not possible, and annual plans are not refunded part-way through except where the law requires it.

## Upgrading during an annual term

Upgrading from Growth annual to Pro annual is prorated for the rest of the term and charged immediately.`,
		},
		{
			Collection: "billing", Title: "Using your Stowline balance",
			Description: "Top-ups, refunds of voided labels and paying out the balance.",
			Body: `Your Stowline balance pays for labels billed through Stowline. You can see it at the top of the **Billing** page.

## What goes in and out

- **In**: card top-ups, refunds of voided labels, approved insurance claims, credits from support, won adjustment disputes.
- **Out**: label purchases, carrier adjustments, insurance.

## Automatic top-ups

When the balance drops below the threshold you set (default $25), your card is charged the refill amount (default $100). You can also top up manually at any time.

## Refunds to your card

Voided labels are refunded to the balance, not your card, because most labels are replaced by another label straight away. If you want money back on your card, write to us. We can refund any balance that came from card top-ups made in the last 120 days, to the card it came from.

## Balance and closing your account

When you close your account, any positive balance is refunded to your card automatically within 10 business days.`,
		},
		{
			Collection: "billing", Title: "Discount programs for nonprofits and new businesses",
			Description: "Who qualifies for a discount, what paying yearly saves, and how to apply.",
			Body: `We offer two discount programs on plan fees, and a saving for paying yearly. None of them applies to postage.

## Paying yearly

Growth and Pro can be paid yearly at ten times the monthly price, which is two months free. Any account can switch under **Billing → Plan → Switch to annual**. Enterprise contracts are yearly by default.

## Nonprofits

Registered charities and nonprofits get 50% off Growth and Pro. Send us your registration document from the messenger and we will apply the discount to your next invoice. We may ask for proof again each year.

## New businesses

Stores less than a year old that have shipped fewer than 500 orders in total can get three months of Growth for free. Apply from **Billing → Plan → New business offer**. The offer cannot be combined with annual billing and is once per business.

## Education

Schools and universities shipping teaching materials can ask for the nonprofit discount.

Discounts are applied from the next invoice and are not backdated.`,
		},
	}
}

func accountArticles() []Article {
	return []Article{
		{
			Collection: "account-security", Title: "Turning on two-factor authentication",
			Description: "Protect your login with an authenticator app or a security key.",
			Body: `Two-factor authentication asks for a code or a security key after your password, so a stolen password alone is not enough to get in.

## Turning it on

` + steps(
				"Go to **Profile → Security → Two-factor authentication**.",
				"Choose **Authenticator app** and scan the QR code with an app such as 1Password, Authy or Google Authenticator. Or choose **Security key** and touch your key.",
				"Enter the six-digit code to confirm.",
				"Save the ten recovery codes somewhere safe. Each works once if you lose your phone.",
			) + `
## Requiring it for the whole team

Owners and admins can require two-factor authentication for everyone under **Settings → Security**. Teammates without it are asked to set it up the next time they sign in and cannot do anything else until they have.

## Lost your phone

Sign in with a recovery code, then set up two-factor again. If you have no recovery codes, ask the account owner to reset two-factor for you from **Settings → Team**. If you are the owner and have lost both, write to us from the email address on the account; we verify ownership before resetting, which takes up to two business days.`,
		},
		{
			Collection: "account-security", Title: "Single sign-on with SAML",
			Description: "Sign in through Okta, Microsoft Entra ID, Google Workspace or another identity provider.",
			Body: `Enterprise accounts can sign in through their identity provider with SAML 2.0.

## Setting up

` + steps(
				"Go to **Settings → Security → Single sign-on** and click **Set up SAML**.",
				"Copy the ACS URL and Entity ID into a new SAML app in your identity provider.",
				"Map the email attribute. Stowline needs the user's email as the NameID or as an attribute called *email*.",
				"Paste your identity provider's metadata URL, or upload the metadata XML.",
				"Click **Test** and sign in. Nothing is enforced until the test succeeds.",
			) + `
## Enforcing

Once tested, choose **Require SSO**. Teammates then sign in only through your identity provider. The account owner can always sign in with a password as a break-glass account.

## Verified domains

Add and verify your email domain under **Settings → Security → Domains** to let anyone with an address on that domain join automatically with the default role when they first sign in through SSO.

## Certificate rotation

Identity providers rotate their signing certificate every one to three years. When yours does, upload the new metadata in Stowline before the old certificate expires, or sign-ins fail with *Signature validation failed*. Stowline accepts the old and new certificate side by side while you switch.`,
		},
		{
			Collection: "account-security", Title: "Exporting your data",
			Description: "Download orders, labels, tracking events and returns as CSV.",
			Body: `You can export your data at any time, on every plan.

## What you can export

` + list(
				"**Orders** with their items, addresses and status.",
				"**Labels** with cost, carrier, service, tracking number and void status.",
				"**Tracking events** for every label.",
				"**Returns** with reasons and refund status.",
				"**Balance history** for accounting.",
			) + `
## How

Go to **Settings → Data → Export**, choose what to export and a date range. Small exports download straight away. Exports of more than 50,000 rows are prepared in the background; we email a link that is valid for 7 days.

## Full account export

Owners can request a full export of everything, including label PDFs, from the same page. It is delivered as a zip file, usually within 24 hours.

## Using the API

For regular exports into your own systems, the API is faster than downloading files. See *Listing shipments* in the API section.`,
		},
		{
			Collection: "account-security", Title: "Deleting your account",
			Description: "What is deleted, what is kept and how long it takes.",
			Body: `Only the account owner can delete the account.

## Before you delete

- Void any labels you will not use. Labels cannot be voided once the account is deleted.
- Export anything you need to keep. Deleted data cannot be recovered.
- Cancel your plan, or deletion will cancel it for you. Plan fees are not refunded for the current period.

## How

Go to **Settings → Account → Delete account**, type the account name and confirm. The account is closed straight away and permanently deleted after 30 days. Within those 30 days the owner can restore it by signing in.

## What we keep

We keep invoices and payment records for seven years because tax law requires it, and carriers keep their own records of labels you bought. Everything else, including orders, customer addresses, labels and tracking pages, is deleted.

## Your positive balance

Any unused balance is refunded to your card within 10 business days of the deletion.`,
		},
		{
			Collection: "account-security", Title: "Changing the account owner",
			Description: "Hand the account to another teammate.",
			Body: `Every account has exactly one owner. The owner is the only person who can delete the account or change the owner.

## Transferring ownership

` + steps(
				"Make sure the new owner is already a teammate with the Admin role.",
				"Go to **Settings → Team**, click the new owner and choose **Make owner**.",
				"Confirm with your password and, if it is on, your two-factor code.",
			) + `
The previous owner becomes an admin. Billing details stay as they are; update the payment method if it belonged to the previous owner.

## When the owner has left the company

If the owner has left and nobody can sign in as them, an admin can ask us to transfer ownership. Write to us from an address on your company's domain. We email the current owner and, if they do not object within five business days, we make the change. We may ask for a letter on company letterhead for accounts without a verified domain.`,
		},
		{
			Collection: "account-security", Title: "The audit log",
			Description: "See who changed what in your account.",
			Body: `Enterprise accounts have an audit log of every important change, kept for two years.

## What is recorded

Sign-ins and failed sign-ins, teammate invitations and removals, role changes, API key creation and revocation, carrier and integration connections, changes to billing details, label voids, bulk deletes and exports.

Each entry shows who did it, when, from which IP address and what changed, with the value before and after.

## Viewing and exporting

Go to **Settings → Security → Audit log**. Filter by person, kind of event or date. Export the filtered list as CSV, or stream entries to your security tooling with the *audit.entry_created* webhook.

## Retention

Entries are kept for two years and then deleted. Export them regularly if your policies require longer retention.`,
		},
		{
			Collection: "account-security", Title: "Managing API keys",
			Description: "Create, scope and rotate the keys your systems use.",
			Body: `API keys let your own systems use Stowline. Growth, Pro and Enterprise include API access.

## Creating a key

Go to **Settings → API keys → Create key**. Give it a name that says where it is used, choose **live** or **test** mode and pick its scopes. A key is shown once; copy it into your secrets manager straight away.

## Scopes

` + table([]string{"Scope", "Allows"}, [][]string{
				{"shipments:read", "Read shipments, labels and tracking."},
				{"shipments:write", "Create shipments and buy and void labels."},
				{"rates:read", "Get rates without buying."},
				{"returns:write", "Create return labels."},
				{"webhooks:write", "Manage webhook endpoints."},
				{"account:read", "Read account details and balance."},
			}) + `
## Rotating a key

Create the new key, deploy it, then revoke the old one. Revoked keys stop working within a minute. If a key leaked, revoke it first; we would rather you have a few minutes of failed requests than a key in the wrong hands.

## Test keys

Test keys start with *sk_test_* and return realistic fake labels that are never charged and cannot be shipped. Use them in development and CI.`,
		},
		{
			Collection: "account-security", Title: "Restricting sign-ins by IP address",
			Description: "Allow access only from your office or VPN.",
			Body: `Enterprise accounts can limit sign-ins and API calls to a list of IP addresses.

## Setting it up

Go to **Settings → Security → IP allowlist** and add the addresses or ranges in CIDR notation, such as *203.0.113.0/24*. Click **Save** and confirm; Stowline checks that your current address is on the list so you do not lock yourself out.

## What it covers

You can apply the list to dashboard sign-ins, API keys, or both. API keys used from a warehouse management system on a cloud host need that host's outbound addresses on the list.

## Locked out

If you are locked out, the account owner can sign in from an allowed address and change the list. If no allowed address is available any more, write to us from the owner's email address.`,
		},
	}
}

func returnsArticles() []Article {
	return []Article{
		{
			Collection: "tracking-returns", Title: "Setting up the returns portal",
			Description: "Let customers start returns themselves within your rules.",
			Body: `The returns portal is a page where your customers start a return, choose why they are returning and print or show a return label. It is included on Pro and Enterprise.

## Turning it on

Go to **Returns → Settings** and switch on the portal. Choose its address, either *returns.stowline.app/your-store* or your own domain, and add your logo and colors.

## Return rules

Set the rules customers see before they start:

` + list(
				"The return window, counted from delivery. 30 days is common.",
				"Which products cannot be returned, by tag or SKU, such as final sale items.",
				"The reasons customers can pick, and which reasons need a photo.",
				"Who pays for the return label, you or the customer. A customer-paid label is deducted from the refund.",
				"Whether returns need your approval before a label is issued.",
			) + `
## The return label

Return labels are created on the carrier and service you choose and are only charged when the carrier scans them. Unused return labels cost nothing. You can also offer QR code returns for USPS, UPS and Royal Mail, where the customer shows a code at the counter instead of printing.

## Refunds

Stowline does not move money for refunds. When a return arrives you mark it received, and Stowline can issue the refund in Shopify, BigCommerce or WooCommerce for you, or tell you to do it in your store.`,
		},
		{
			Collection: "tracking-returns", Title: "The branded tracking page",
			Description: "Send customers to a tracking page with your logo instead of the carrier's.",
			Body: `The tracking page shows your customer where their parcel is, with your logo, colors and links back to your store.

## Setting it up

Go to **Tracking → Page** and add your logo, colors and a banner. Choose which products to promote; the page can show a few items from your store.

The address is *track.stowline.app/your-store* unless you connect your own domain, such as *track.yourstore.com*, under **Tracking → Domain**.

## Tracking emails

Under **Tracking → Notifications** choose which updates customers get: shipped, out for delivery, delivered and delivery problems. Each email links to the tracking page. You can edit every email's subject and text, and send from your own address once your domain is verified.

## Where tracking comes from

Stowline checks each carrier for new events every 30 minutes for the first two weeks after shipping and every few hours after that. Some carriers push events to us as they happen. A parcel with no events for 7 days is flagged as possibly lost on the Labels page.`,
		},
		{
			Collection: "tracking-returns", Title: "My tracking has not updated",
			Description: "What a tracking gap usually means and when to worry.",
			Body: `Tracking that has not changed for a while is the most common question we get, and it is usually not a problem.

## Label created, not yet scanned

*Label created* means the label was bought but the carrier has not scanned the parcel yet. This is normal until the parcel is handed over or picked up. If a parcel was handed over more than two business days ago and still shows only this status, the first scan was probably missed; it usually appears at the next sorting center.

## No movement for a few days

Parcels are scanned when they enter and leave sorting centers, not while they are on a truck or plane. Domestic parcels can go two days without a scan; international parcels can go a week or more, especially while clearing customs.

## When to act

` + table([]string{"Service", "Wait before investigating"}, [][]string{
				{"Domestic ground", "5 business days without a scan"},
				{"Domestic express", "2 business days"},
				{"International", "10 business days"},
			}) + `
After that, open the label and click **Report a problem**. We will open a trace with the carrier. If you insured the parcel, you can file a claim after 10 days without movement for domestic or 20 days for international parcels.`,
		},
		{
			Collection: "tracking-returns", Title: "Delivered but not received",
			Description: "What to do when tracking says delivered and the customer has nothing.",
			Body: `Sometimes tracking shows a parcel as delivered but your customer cannot find it.

## Ask the customer to check

` + list(
				"Around the property, with neighbors and with any building reception or mail room.",
				"With others in the household who may have taken it in.",
				"For a delivery notice; some carriers mark a parcel delivered when it is left at a pickup point.",
			) + `
Most parcels marked delivered turn up within two days.

## If it does not turn up

After two days, open the label and click **Report a problem → Delivered, not received**. Carriers ask the recipient to sign a statement before they investigate, and Stowline sends your customer the form. Carrier investigations take 5 to 15 business days.

## Insurance

Stowline insurance covers parcels marked delivered that never arrived when a signature was not required, once the carrier's investigation is closed. Carrier declared value usually does not cover them.`,
		},
		{
			Collection: "tracking-returns", Title: "Exchanges and store credit in the returns portal",
			Description: "Offer an exchange or store credit before a refund.",
			Body: `Offering an exchange or store credit keeps more of the sale. Both are available on Pro and Enterprise with Shopify and BigCommerce.

## Exchanges

Customers can swap an item for another size or color of the same product, or for any product of the same price. When the return label is scanned, Stowline creates the exchange order in your store at no charge, so it can ship before the return arrives. Turn on **Ship exchanges on first scan** for this; otherwise the exchange order is created when you mark the return received.

## Store credit

Customers who choose store credit get a gift card in your store for the value of the returned items. You can add a bonus to make it more attractive, for example 10% extra.

## Price differences

When an exchange costs more, the customer pays the difference by card in the portal. When it costs less, the difference is refunded or given as store credit, as your settings say.`,
		},
		{
			Collection: "tracking-returns", Title: "Tracking webhooks and notifications for developers",
			Description: "Receive tracking events in your own systems.",
			Body: `Every tracking change can be sent to your systems as a webhook, so your customer service tools know where a parcel is.

## The tracker.updated event

Register an endpoint for *tracker.updated* under **Settings → Webhooks** or through the API. Each event carries the tracking number, the carrier, the new status, the carrier's own status text and location, and the estimated delivery date when the carrier gives one.

## Statuses

` + table([]string{"Status", "Meaning"}, [][]string{
				{"pre_transit", "Label created, not scanned."},
				{"in_transit", "Moving through the carrier's network."},
				{"out_for_delivery", "On the delivery vehicle."},
				{"delivered", "Delivered or left at a pickup point."},
				{"available_for_pickup", "Waiting at a pickup point."},
				{"return_to_sender", "Coming back to you."},
				{"failure", "Delivery failed, such as a damaged or refused parcel."},
				{"unknown", "The carrier sent something we could not map."},
			}) + `
Events can arrive out of order when carriers backfill scans. Use the event's *occurred_at* time, not the time it reached you, to decide the latest status.`,
		},
	}
}

/*
internalArticles are pages only the agent reads: the policies support
applies to requests for exceptions.
*/
func internalArticles() []Article {
	return []Article{
		{
			Collection: "billing", Title: "Support policy: refunds, credits and exceptions",
			Description: "What support can offer without a manager, for the agent's eyes only.",
			Visibility:  "agent_only",
			Body: `This page is for the support team and the AI agent. Customers do not see it.

## Plan fee refunds

- A plan charge can be refunded in full within 14 days if the account bought no labels on the new plan since the charge.
- An upgrade charged by mistake can be reversed within 7 days whatever the usage.
- Annual plans are never refunded part-way through without a manager's approval. Hand these to the Billing team.

## Balance refunds

- Balance from card top-ups in the last 120 days can be refunded to the card it came from.
- Balance from voided labels is refunded to the balance. Never promise a card refund for a void.

## Credits

- Up to $25 of goodwill credit can be offered when a Stowline bug cost the customer money, with a note on the conversation explaining why.
- Anything over $25, or any credit for carrier delays, goes to the Billing team.

## Carrier problems

Carrier delays, losses and adjustments are the carrier's decision. Support can open a trace, file a dispute or help with an insurance claim, but must never promise an outcome.`,
		},
		{
			Collection: "troubleshooting", Title: "Support policy: when to hand a conversation to a person",
			Description: "Topics the AI agent must hand to the team, for the agent's eyes only.",
			Visibility:  "agent_only",
			Body: `This page is for the AI agent. Customers do not see it.

Always hand the conversation to the team when:

` + list(
				"The customer asks for a person, a manager or a phone call.",
				"The customer says they will cancel, dispute a charge with their bank or take legal action.",
				"Money must be moved: a refund, a credit, a balance payout or a disputed adjustment.",
				"The account was compromised or the customer cannot sign in and has no recovery codes.",
				"An Enterprise customer reports that labels cannot be bought at all.",
				"The customer reports the same problem a third time in the conversation.",
			) + `
When handing over, summarize what the customer needs and what was already tried, so the teammate does not have to ask again.

Do not hand over just because a question is technical. API and integration questions are well covered in the help center; search it first.`,
		},
	}
}
