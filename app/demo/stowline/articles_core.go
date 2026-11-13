/*
|--------------------------------------------------------------------------
| Hand-Written Articles: Getting Started and Labels
|--------------------------------------------------------------------------
|
| The articles a new store reads first and the ones about everyday label
| work, each written by hand.
|
*/

package stowline

func gettingStartedArticles() []Article {
	return []Article{
		{
			Collection: "getting-started", Title: "Welcome to Stowline: ship your first order in 15 minutes",
			Description: "The five steps from a new account to a printed label.",
			Body: `Stowline compares rates across your carriers, buys the label you pick and sends the tracking number back to your store. This guide takes you from a new account to your first printed label. Most stores finish it in about fifteen minutes.

## 1. Add your ship-from address

Go to **Settings → Warehouses** and add the address your parcels leave from. Carriers use it to work out rates and pickups, so use the exact street address, not a PO box. If you ship from more than one place, add each one now; you can pick a default and change it per order later.

## 2. Connect a store or marketplace

Open **Settings → Integrations** and choose your platform. Shopify, BigCommerce and Wix connect through their app stores; WooCommerce and Squarespace need an API key; Amazon, eBay and Etsy ask you to sign in. New orders appear on the **Orders** page within a few minutes of being paid.

No store? You can create orders by hand on the Orders page, import a CSV, or send them through the API.

## 3. Choose your carriers

Every account can buy discounted USPS labels straight away with no carrier account. To use your own UPS, FedEx, DHL Express or other negotiated rates, go to **Settings → Carriers** and connect the account. See *Connecting a carrier account* for what each carrier asks for.

## 4. Set package defaults

Under **Settings → Packages**, save the boxes and mailers you use with their inside dimensions and empty weight. Rates depend on size as well as weight: carriers charge dimensional weight for light, bulky parcels, so an accurate box saves money and surprise adjustments.

## 5. Buy and print a label

Open an order, check the weight and package, and click **Get rates**. Stowline shows every service from every connected carrier with its price and delivery estimate. Pick one and click **Buy label**. The label opens as a PDF ready for a 4x6 thermal printer or a letter-size sheet.

As soon as the label is bought, the tracking number is sent to your store and your customer gets their shipping email.

## What next

- Set up **automation rules** so common orders get the right service without clicking.
- Turn on the **branded tracking page** and the **returns portal**.
- Invite your team under **Settings → Team**.

If anything in these steps does not look the way it is described here, write to us from the messenger and we will help you finish.`,
		},
		{
			Collection: "getting-started", Title: "Importing orders from a CSV file",
			Description: "Upload orders from a spreadsheet when your platform is not connected.",
			Body: `If you sell somewhere Stowline does not connect to, or you are moving over from another tool, you can import orders from a CSV file.

## Prepare the file

Download the template from **Orders → Import → Download template**. Keep the header row exactly as it is. The required columns are:

` + table([]string{"Column", "What to put in it"}, [][]string{
				{"order_number", "Your order reference. Must be unique; a repeated number updates the earlier order."},
				{"recipient_name", "The person the parcel is addressed to."},
				{"address_line1", "Street and number."},
				{"city, postal_code, country", "Country as a two-letter code, such as US, GB or DE."},
				{"weight", "In the unit chosen on the import screen, grams, kilograms, ounces or pounds."},
				{"item_sku, item_quantity, item_value", "One row per item; repeat the order number on each row of a multi-item order."},
			}) + `
Optional columns include phone, email, company, address_line2, state, package name and service. If you name a saved package, its dimensions are used.

## Upload

Go to **Orders → Import**, pick the file and choose the weight unit. Stowline checks every row before anything is created and shows the problems it found with their row numbers. Fix them in the spreadsheet and upload again; rows that were already right are not duplicated.

## Common problems

- **Country not recognized**: use two-letter ISO codes. "UK" is not valid; use "GB".
- **Weight is zero**: an empty weight column is read as zero. Carriers refuse labels for parcels under 1 oz or 28 g.
- **Special characters look wrong**: save the file as *CSV UTF-8*. Excel's plain CSV uses a different encoding and turns accents into question marks.
- **Phone numbers lose their leading zero**: format the column as text before typing.

Large files are fine. Imports of up to 10,000 rows finish in under a minute; bigger files are split into batches automatically and you get an email when the last one is done.`,
		},
		{
			Collection: "getting-started", Title: "Setting up automation rules",
			Description: "Pick the service, package and insurance automatically from the order's details.",
			Body: `Automation rules apply settings to new orders the moment they are imported, so most orders are ready to ship without opening them.

## How rules work

A rule has **conditions** and **actions**. When an order matches every condition of a rule, the rule's actions are applied. Rules run from top to bottom and every matching rule applies; if two rules set the same thing, the lower one wins. Drag rules to change their order.

Conditions can look at the destination country, state or postal code, the order total, weight, item SKUs, tags from your store, the sales channel and the requested shipping method.

Actions can set the carrier and service, the package, the ship-from warehouse, insurance, signature confirmation, customs contents type, and add tags or a note.

## Examples

` + list(
				"Orders with *Express* in the requested shipping method → UPS Next Day Air.",
				"Destination outside the United States and weight under 2 kg → USPS First-Class International in the padded mailer.",
				"Order total over $250 → add insurance for the full value and require a signature.",
				"SKU starts with *BOOK-* → USPS Media Mail.",
			) + `
## Cheapest rate rules

Instead of a fixed service, an action can choose **cheapest rate** within a delivery promise, for example *cheapest service that arrives within 3 business days*. Stowline fetches rates when the order arrives and picks the winner. Because rates change, the label price can differ by a few cents from the rate shown on import; the label is always bought at the current rate.

## Testing a rule

Open **Settings → Automation**, click **Test** on a rule and paste an order number. Stowline shows which conditions matched and what would change without changing anything. Rules only apply to orders imported after you save them; use **Reapply rules** on the Orders page for orders already waiting.`,
		},
		{
			Collection: "getting-started", Title: "Inviting your team and choosing roles",
			Description: "Add teammates and decide what each one can do.",
			Body: `You can add as many teammates as your plan has seats: one on Free, three on Growth, ten on Pro and fifty on Enterprise.

## Invite someone

Go to **Settings → Team → Invite** and enter their email address and role. They get an email with a link that is valid for seven days. If it expires, open the team list and click **Resend**.

## Roles

` + table([]string{"Role", "Can do"}, [][]string{
				{"Owner", "Everything, including billing, deleting the account and transferring ownership. One per account."},
				{"Admin", "Everything except changing the owner and closing the account."},
				{"Shipper", "Buy, void and print labels, manage orders and returns. Cannot see billing or change settings."},
				{"Viewer", "See orders, labels and reports. Cannot buy anything."},
			}) + `
Enterprise accounts can also create custom roles with exactly the permissions they need, for example a warehouse role that can print but never void labels.

## Removing someone

Open the team list, click the person and choose **Remove from team**. Their labels and history stay. If they had API keys, those keys keep working until you revoke them under **Settings → API keys**, so check there too.

## Seats and billing

A pending invitation does not use a seat until it is accepted. Adding a seat beyond your plan's allowance is not possible on Free and Growth; on Pro and Enterprise extra seats are $15 a month each, prorated to the day.`,
		},
		{
			Collection: "getting-started", Title: "Setting up warehouses and ship-from addresses",
			Description: "Ship from more than one place and route orders to the right one.",
			Body: `A warehouse in Stowline is any place parcels leave from: your garage, a 3PL or a store's back room.

## Adding a warehouse

Under **Settings → Warehouses → Add warehouse**, enter the full address, a phone number carriers can call and the opening hours. Opening hours are used for pickups; carriers will not schedule a pickup outside them.

You can also set a **return address** that differs from the ship-from address, for example a returns center.

## Choosing where an order ships from

New orders ship from your default warehouse unless something says otherwise:

` + steps(
				"An automation rule with the action *Ship from*.",
				"Inventory location from your store, for platforms that send it (Shopify, Adobe Commerce, NetSuite). Map each store location to a warehouse under Settings, Warehouses, Mapping.",
				"The default warehouse.",
			) + `
## Carrier accounts per warehouse

Some negotiated rates are tied to a location. Under each carrier connection you can limit it to certain warehouses. A connection limited to one warehouse does not show rates for orders shipping from another.

## Deleting a warehouse

A warehouse with labels bought in the last 90 days cannot be deleted, because voids and returns still need its address. Archive it instead; archived warehouses disappear from menus but keep their history.`,
		},
		{
			Collection: "getting-started", Title: "Understanding rates, surcharges and delivery estimates",
			Description: "Why a rate is what it is, and why it can change after you buy.",
			Body: `When you click **Get rates**, Stowline asks every connected carrier for a price for that exact parcel and shows them side by side. Each rate is the carrier's price for your account, including any discount you negotiated.

## What makes up a rate

- **Base price** for the service, weight and distance (the carrier's zone).
- **Dimensional weight**: carriers bill the greater of the actual weight and the size-based weight, calculated as length × width × height divided by a divisor (139 for most US services in inches). A big light box can cost more than a small heavy one.
- **Surcharges** for residential delivery, remote areas, extended areas, Saturday delivery, large packages and fuel. Fuel changes weekly.
- **Options** you add, such as signature, insurance or adult signature.

## Delivery estimates

The estimate comes from the carrier and assumes the parcel is handed over before the day's cut-off. It is not a guarantee except for services with a money-back guarantee, and those guarantees are suspended by most carriers between late November and early January.

## Why a carrier charged more later

Carriers measure and weigh parcels in their network. If the measured weight or size is bigger than what was on the label, or an address turns out to be residential, they send an **adjustment**. Stowline passes adjustments on and shows each one under **Billing → Adjustments** with the carrier's measurements, so you can see which order caused it.

To avoid adjustments, weigh every parcel, save accurate box sizes and keep residential delivery set to *Detect automatically*.`,
		},
	}
}

func labelArticles() []Article {
	return []Article{
		{
			Collection: "shipping-labels", Title: "Voiding a label and getting your money back",
			Description: "How long you have to void, and when the refund arrives.",
			Body: `If you bought a label you will not use, void it so you are not charged for it.

## How to void

Open the order, click the label and choose **Void label**. To void many labels at once, select them on the **Labels** page and choose **Void selected**.

## Time limits

` + table([]string{"Carrier", "Void window", "When the money comes back"}, [][]string{
				{"USPS", "30 days after purchase", "14 to 21 days after the void, once USPS confirms the label was never scanned"},
				{"UPS", "Until the end-of-day close, or 90 days if never scanned", "On the next invoice cycle, usually within 7 days"},
				{"FedEx", "Until the parcel is first scanned", "Immediately for Stowline-billed labels"},
				{"DHL Express", "Until pickup", "Immediately"},
				{"Royal Mail", "Until the end of the day of purchase", "Within 3 working days"},
				{"Other carriers", "Until the first scan", "Within 7 days"},
			}) + `
## Where the refund goes

For labels billed through Stowline, the refund goes back to your **Stowline balance**, not to your card. You can see it under **Billing → Balance history**. If you want the balance paid out to your card, write to us; we can refund balances older than 30 days to the original payment method.

For labels billed on your own carrier account, the carrier credits that account directly. Stowline cannot speed this up.

## A void was refused

A label that has been scanned by the carrier cannot be voided, even if the parcel was never sent. USPS sometimes scans labels on arrival at a post office in bulk. If you believe a label was scanned by mistake, open a refund request from the label's page within 30 days and we will ask the carrier for you. These requests take up to 30 days and are decided by the carrier.`,
		},
		{
			Collection: "shipping-labels", Title: "Printing labels on a thermal printer",
			Description: "Set up a 4x6 thermal printer and fix blank or cut-off labels.",
			Body: `Thermal printers are the fastest way to print labels and need no ink. Stowline works with any 4x6 thermal printer, including Zebra, Rollo, MUNBYN and DYMO 4XL.

## Choose the right format

Under **Settings → Printing**, set the label format to **4x6 PDF** to print from your browser, or **4x6 ZPL** if you use the Stowline print helper with a Zebra printer. ZPL prints faster and sharper because the printer draws the label itself.

## Browser printing settings

When the PDF opens, print it with these settings:

` + list(
				"Paper size: 4 x 6 in (100 x 150 mm). If the size is missing, add it in the printer's driver preferences.",
				"Scale: 100% or *Actual size*. *Fit to page* shrinks the barcode and some carriers reject shrunk barcodes.",
				"Margins: none.",
				"Orientation: portrait.",
			) + `
## The print helper

The Stowline print helper is a small app for Windows and macOS that prints labels the moment they are bought, without a print dialog. Download it from **Settings → Printing → Print helper**, sign in, and choose a printer for labels and one for packing slips.

## Fixing common problems

- **Blank labels**: the labels are upside down or not direct thermal labels. Thermal labels only darken on one side; scratch the surface with a fingernail to find it.
- **Label is cut off**: the scale is not 100%, or the paper size is letter. Check both settings.
- **Every other label is blank**: the printer has not calibrated to the label gap. Hold the feed button until it feeds two labels, or run calibration from the printer's utility.
- **Faded print**: raise the darkness setting in the printer driver to 20 or more.`,
		},
		{
			Collection: "shipping-labels", Title: "Batch shipping: buy hundreds of labels at once",
			Description: "Create a batch, fix the orders that fail and print in one go.",
			Body: `Batches let you buy labels for many orders in one go. A batch can hold up to 1,000 orders.

## Creating a batch

On the **Orders** page, select the orders and click **Create batch**. Every order must have a weight, a package and a service; orders missing one are listed so you can fix them or remove them from the batch. Automation rules are the easiest way to make sure orders arrive complete.

## Buying the batch

Click **Buy labels**. Stowline buys the labels in the background, usually 50 to 100 a minute depending on the carrier. You can leave the page; you will get a notification when the batch finishes.

## When some labels fail

A batch never stops for one bad order. Orders that fail stay in the batch with the carrier's error, such as an address the carrier cannot find. Fix them and click **Retry failed**. The labels that worked are not bought twice.

## Printing

When the batch is done, click **Print all**. Labels are sorted in the order you choose: by order number, by SKU (handy for picking) or by service. You can also print packing slips in the same order.

## Batch limits by plan

Batches count toward your plan's monthly label allowance like any other label. On the Free plan batches are limited to 25 orders.`,
		},
		{
			Collection: "shipping-labels", Title: "Customs forms and international shipping",
			Description: "What Stowline fills in for you and what you need to provide.",
			Body: `Every parcel that crosses a border needs a customs declaration. Stowline creates it electronically and, where the carrier needs paper, prints the forms with the label.

## What you need on each order

` + list(
				"A description of each item that a customs officer would understand. \"Cotton t-shirt\" is good; \"Apparel\" or \"Gift\" is not.",
				"Quantity and value of each item, in the order's currency.",
				"The country of origin, meaning where the item was made, not where you ship from.",
				"An HS tariff code for each item. Most carriers now require six digits for shipments to the EU, UK and Australia, and DHL Express refuses dutiable shipments without them.",
			) + `
Save these on your products under **Products** and they are filled in automatically. Products imported from Shopify bring the HS code and country of origin if you have set them there.

## Duties and taxes

Choose who pays duties under the order's **International** section:

- **DDU / DAP** (default): the recipient pays when the parcel arrives. Carriers may charge them a handling fee.
- **DDP**: you pay, and the carrier bills you afterwards. Supported for UPS, FedEx and DHL Express.

## EU and UK VAT

Since 2021 low-value goods sent to the EU or UK carry VAT at the point of sale. If you are registered for IOSS (EU) or have a UK VAT number, add them under **Settings → Customs** and they are sent with every declaration to those destinations.

## Paperless trade

UPS, FedEx and DHL Express accept the commercial invoice electronically for most countries, so nothing needs to be printed. Stowline turns on paperless trade automatically when the destination supports it.`,
		},
		{
			Collection: "shipping-labels", Title: "Insurance: what it covers and how to file a claim",
			Description: "Stowline shipping insurance, carrier declared value and claims.",
			Body: `You can insure parcels either through Stowline's insurance or through the carrier's declared value.

## Stowline insurance

Stowline insurance covers loss, damage and theft up to $5,000 per parcel for $0.95 per $100 of value, with no deductible. It covers every carrier and destination except a short list of restricted countries. Add it per order, or automatically with an automation rule.

## Carrier declared value

UPS and FedEx include $100 of liability on every parcel and sell more on top. Claims for declared value go to the carrier and follow the carrier's rules, which usually exclude items packed poorly.

## Filing a Stowline claim

` + steps(
				"Wait until the tracking has shown no movement for 10 days (domestic) or 20 days (international) for a lost parcel. Damaged parcels can be claimed straight away.",
				"Open the label and click **File a claim**.",
				"Upload the invoice or receipt showing what the item cost you, and for damage, photos of the item and the box.",
				"Claims are decided within 10 business days. Approved claims are paid to your Stowline balance or by bank transfer.",
			) + `
Claims must be filed within 60 days of the ship date. We cover the cost of the item and the original shipping cost, not the retail price you sold it for.`,
		},
		{
			Collection: "shipping-labels", Title: "Packing slips and pick lists",
			Description: "Customize what goes in the box and print it with the label.",
			Body: `A packing slip lists what is in the parcel. A pick list tells your team what to take off the shelves for a batch of orders.

## Customizing the packing slip

Under **Settings → Packing slips** you can add your logo, change which columns show (SKU, image, weight, price), add a message at the bottom and choose between the standard and the long notes layout. The long notes layout prints gift messages and personalization notes in full.

You can print the slip on its own letter or A4 page, or on the 4x6 label stock right after the label.

## Pick lists

Select orders and click **Print pick list**. The list groups items across all selected orders by SKU and bin location, so a picker walks the shelves once. Bin locations come from your products page or from your store if it sends them.

## Hiding prices

Gift orders often should not show prices. Turn on **Hide prices on gift orders**; Stowline treats an order as a gift when your store marks it as one or when it has a gift message.`,
		},
		{
			Collection: "shipping-labels", Title: "Scheduling a carrier pickup",
			Description: "Book a pickup so you do not have to drop parcels off.",
			Body: `Most carriers can collect parcels from your warehouse. Stowline books pickups with UPS, FedEx, DHL Express, USPS, Canada Post, Australia Post and Purolator. DPD and GLS collect on a standing daily schedule you agree with your local depot, so there is nothing to book.

## Booking a pickup

On the **Labels** page, filter by carrier and date, select the labels and click **Schedule pickup**. Choose the warehouse, the date and a ready-by and close time. Stowline sends the parcel count and total weight to the carrier.

## Cut-off times

Each carrier has its own deadline for booking, for example the same morning for UPS and FedEx, and the evening before for USPS. If you miss it, the next available day is offered.

## Fees

USPS pickups are free when you hand over at least one Priority Mail or Ground Advantage parcel. UPS and FedEx charge an on-call pickup fee unless you have a daily pickup on your account; the fee shows before you confirm.

## Canceling

Open **Pickups**, find the booking and click **Cancel**. Cancel before the carrier's driver is dispatched or you may be charged.`,
		},
	}
}
