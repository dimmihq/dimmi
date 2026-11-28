/*
|--------------------------------------------------------------------------
| Article Families: Carriers and Integrations
|--------------------------------------------------------------------------
|
| Four articles per carrier, connecting it, its services and labels,
| its pickups, and its errors, and two per integration, connecting it
| and fixing its imports. Each template is written once, by hand, and
| filled with the carrier's or platform's own facts.
|
*/

package stowline

import (
	"fmt"
	"strings"
)

func carrierArticles(c Carrier) []Article {
	services := make([][]string, 0, len(c.Services))
	for i, s := range c.Services {
		speed := []string{
			"Economy, 3 to 7 business days", "Standard, 2 to 5 business days",
			"Express, 1 to 2 business days", "Fastest, next business day or time-definite",
		}[min(i, 3)]
		services = append(services, []string{s, speed})
	}

	connect := Article{
		Collection: "carriers", Title: "Connecting your " + c.Name + " account",
		Description: "Use your own " + c.Name + " rates and account in Stowline.",
		Body: fmt.Sprintf(`Connecting your own %[1]s account lets you buy labels at the rates you negotiated with %[1]s, billed by %[1]s on your usual invoice. You can connect %[1]s on the Growth plan and above.

## What you need

- Your %[2]s, for example *%[3]s*.
- Access to the %[1]s account's online portal, in case %[1]s asks you to confirm the connection.
- The ship-from address on the %[1]s account. It must match a warehouse in Stowline, or %[1]s may refuse labels.

## Connecting

%[4]s
## After connecting

Stowline fetches a test rate straight away. If it works, %[1]s services appear in **Get rates** within a minute. If you use more than one %[1]s account, for example one per warehouse, connect each one and limit it to its warehouse under the connection's **Warehouses** setting.

%[5]s

## Disconnecting

Disconnecting stops new labels on the account. Labels already bought stay valid, and you can still void them until their void window closes, because Stowline keeps a read-only link to the account for 90 days.

## If the connection fails

The most common reasons are a mistyped account number, an account that is not yet enabled for API access, or an address that does not match. See *%[1]s errors and how to fix them* for the exact messages %[1]s returns.`,
			c.Name, c.AccountField, c.AccountExample,
			steps("Go to **Settings → Carriers → Add carrier** and choose **"+c.Name+"**.",
				"Enter your "+c.AccountField+" and the account's billing postal code.",
				"Accept "+c.Name+"'s technology agreement. Stowline shows it word for word; it is between you and "+c.Name+".",
				"Click **Connect**."),
			carrierNote(c)),
	}

	labels := Article{
		Collection: "carriers", Title: c.Name + " services, label formats and limits",
		Description: "Which " + c.Name + " services Stowline supports and what they accept.",
		Body: fmt.Sprintf(`Stowline supports these %[1]s services:

%[2]s
Availability depends on the origin and destination. **Get rates** only shows services %[1]s offers for that parcel.

## Weight and size limits

%[1]s accepts parcels up to %[3]d kg on the services above. Parcels over the limit are refused at label purchase with an error that names the limit. Some services, especially letter and flat services, have much lower limits; the service's own limit is shown on its rate.

## Label formats

%[1]s labels are available as %[4]s. Choose the format under **Settings → Printing**. If a format you chose is not available for %[1]s, Stowline falls back to 4x6 PDF and tells you on the label.

## Good to know

%[5]s

## Signature and delivery options

You can add signature confirmation, adult signature and Saturday delivery where %[1]s offers them. Options are priced by %[1]s and shown on the rate before you buy. An option that is not available for the destination is greyed out with the reason.

## Insurance

%[1]s's own liability and declared value follow %[1]s's terms. Stowline insurance covers %[1]s parcels too and is often cheaper for values over $100; see *Insurance: what it covers and how to file a claim*.`,
			c.Name, table([]string{"Service", "Typical speed"}, services), c.MaxWeightKg,
			strings.Join(c.LabelFormats, ", "), c.Quirk),
	}

	pickups := Article{
		Collection: "carriers", Title: "Pickups and end-of-day manifests for " + c.Name,
		Description: "How " + c.Name + " collects your parcels and what to hand over.",
		Body: fmt.Sprintf(`%[1]s collects from your warehouse either on a booked pickup or on a standing daily collection.

## Booking

Pickups for %[1]s must be booked %[2]s. Select the labels on the **Labels** page and click **Schedule pickup**. Stowline sends %[1]s the number of parcels and the total weight, and shows the confirmation number when %[1]s accepts.

## End-of-day manifest

At the end of each day, Stowline closes the day's %[1]s labels into a manifest, also called an end-of-day form or SCAN form. The driver scans the manifest's single barcode instead of every parcel, and tracking starts for all of them at once.

%[3]s
Labels bought after the manifest is closed go on the next day's manifest. You can close a manifest early from **Labels → Manifests → Close now**, for example before an early pickup.

## Missed pickups

If the driver did not come, open **Pickups**, find the booking and click **Report missed pickup**. Stowline asks %[1]s to come back on the next working day. Parcels on a missed pickup keep their labels; nothing needs to be bought again, though %[1]s may show the original ship date until the first scan.

## Dropping off instead

You can always drop %[1]s parcels at a counter or drop box. Hand over the manifest too if you have one, or tracking may not start until each parcel is scanned individually.`,
			c.Name, c.PickupCutoff, steps(
				"Manifests close automatically at 6:00 p.m. warehouse time.",
				"Print the manifest from **Labels → Manifests** and hand it to the driver with the parcels.",
				"Voided labels are left off the manifest automatically.",
			)),
	}

	errs := Article{
		Collection: "troubleshooting", Title: c.Name + " errors and how to fix them",
		Description: "The messages " + c.Name + " returns when a rate or label fails.",
		Body: fmt.Sprintf(`When %[1]s refuses a rate or a label, Stowline shows %[1]s's own error code and message next to an explanation. These are the ones we see most.

## %[2]s: %[3]s

This is the most common %[1]s error. %[4]s

To fix it:

%[5]s
## Authentication failed

%[1]s no longer accepts the account details saved on the connection. This happens when the account was closed, the password or key was changed in %[1]s's portal, or %[1]s disabled API access. Reconnect the account under **Settings → Carriers**.

## Service unavailable for this route

The service you picked does not run between these two places, or not for this weight. Choose another service in **Get rates**; only services %[1]s offers for the parcel are shown there, so this error usually means an automation rule picked the service.

## Rate expired

Rates are valid for a short time. When a label is bought more than 24 hours after its rate was fetched, Stowline fetches a fresh rate first; if the price changed, you are asked to confirm.

## %[1]s is not responding

%[1]s's systems are down or slow. Stowline retries for you; labels in a batch that failed this way are retried automatically for an hour. Check our status page for incidents.

If none of these match what you see, send us the order number and the full error text.`,
			c.Name, c.ErrorCode, c.ErrorText, errorStory(c), steps(
				"Open the order and click **Edit address**.",
				"Use **Validate address** to see Stowline's suggestion and accept it if it is right.",
				"If the address is right but "+c.Name+" still refuses it, confirm it with the recipient and turn on **Ship to unverified address** for this order.",
				"Buy the label again.",
			)),
	}

	return []Article{connect, labels, pickups, errs}
}

/*
carrierNote is the paragraph only some carriers need on connecting.
*/
func carrierNote(c Carrier) string {
	switch c.Key {
	case "usps":
		return "You do not need a USPS account: every Stowline account buys USPS labels at Stowline's commercial rates. Connect your own USPS account only if you have a USPS contract with its own pricing."
	case "fedex":
		return "FedEx checks every new connection before the first label. Until the check passes, rates show but labels fail with *Account pending*. The check usually finishes within one business day."
	case "canada_post":
		return "To get your contract rates, save both the customer number and the contract number. With only the customer number you see counter rates."
	case "purolator":
		return "Make sure you paste the production key, not the test key. Test keys connect successfully but every label fails."
	default:
		return c.Name + " may take a few minutes to activate API access on a new account. If the test rate fails right after you opened the account, wait an hour and reconnect."
	}
}

/*
errorStory explains a carrier's most common error in its own terms.
*/
func errorStory(c Carrier) string {
	if strings.Contains(strings.ToLower(c.ErrorText), "address") || strings.Contains(strings.ToLower(c.ErrorText), "postal") ||
		strings.Contains(strings.ToLower(c.ErrorText), "suburb") {
		return c.Name + " checks the recipient's address against its own database before it sells a label. A missing apartment number, a misspelled street or a postal code that belongs to the next town is enough to fail the check, even when the address looks right to a person."
	}
	return c.Name + " refused the label because something about the parcel or the account does not meet its rules. The message names the exact field. It is usually quick to fix on the order and does not need a new connection."
}

func platformArticles(p Platform) []Article {
	sync := fmt.Sprintf("every %d minutes", p.SyncMinutes)
	if p.SyncMinutes == 0 {
		sync = "as events happen"
	}
	back := "Stowline sends the tracking number and carrier back to " + p.Name + " as soon as a label is bought and marks the order fulfilled."
	if !p.Fulfills {
		back = "Stowline does not fulfill orders in " + p.Name + "; it sends shipping costs and label data for your records."
	}

	connect := Article{
		Collection: "integrations", Title: "Connecting " + p.Name,
		Description: "Import " + p.Name + " orders and send tracking back automatically.",
		Body: fmt.Sprintf(`Stowline connects to %[1]s through %[2]s. Once connected, it imports %[3]s %[4]s.

## Connecting

%[5]s
## What is imported

- Orders that are paid and not yet fulfilled.
- The items with their SKU, quantity, price, weight and, where %[1]s has them, the HS code and country of origin.
- The shipping method the customer chose at checkout, which automation rules can use.
- Order tags and notes.

Orders older than 30 days are not imported on the first sync. Use **Import older orders** on the connection to bring in up to 180 days.

## What is sent back

%[6]s

## Sync settings

On the connection's page you can choose which order statuses to import, whether to import orders from all locations or only some, and whether to notify the customer when Stowline marks the order fulfilled. Changes apply from the next sync. Click **Sync now** to import immediately.

## Several %[1]s accounts

You can connect as many %[1]s accounts as your plan's store limit allows: one on Free, unlimited on Growth and above. Each order shows which account it came from, and automation rules can filter on it.`,
			p.Name, p.ConnectsWith, map[string]string{
				"store": "orders", "marketplace": "orders", "erp": "sales orders",
				"accounting": "nothing, and exports shipping costs", "automation": "nothing by itself, and runs your Zaps",
			}[p.Kind], sync,
			steps("Go to **Settings → Integrations** and choose **"+p.Name+"**.",
				"Follow the prompt: "+p.ConnectsWith+".",
				"Choose the warehouse new orders ship from and the statuses to import.",
				"Click **Save and sync**. The first import can take a few minutes for large stores."),
			back),
	}

	trouble := Article{
		Collection: "troubleshooting", Title: p.Name + " orders are not importing",
		Description: "Find out why " + p.Name + " orders are missing from Stowline.",
		Body: fmt.Sprintf(`If a %[1]s order is not on the **Orders** page, work through these checks in order. Most missing orders are explained by the first three.

## 1. Has the sync run since the order was placed?

%[1]s orders are imported %[2]s. Open **Settings → Integrations → %[1]s** to see when the last sync ran and how many orders it imported. Click **Sync now** if the order is newer than the last sync.

## 2. Does the order match the import filter?

The connection only imports orders in the statuses you chose. An order that is unpaid, on hold, already fulfilled or canceled is skipped. Order %[3]s style numbers appear in the sync log with the reason they were skipped.

## 3. Is the connection still authorized?

If the last sync shows an authorisation error, reconnect. Sign-in details, app permissions and tokens change on the %[1]s side without Stowline being told.

## 4. The problem we see most with %[1]s

The most common %[1]s-specific cause is that %[4]s. To fix it, %[5]s.

## 5. Is the order in another view?

Check **Orders → All**, not only **Awaiting shipment**. Orders that were archived, merged or split by an automation rule may be in another view.

## Still missing

Send us the order number, the time it was placed and a screenshot of the connection's sync log. We can see the request %[1]s answered and tell you exactly what happened.`,
			p.Name, sync, p.OrderNumberStyle, p.CommonProblem, p.ProblemFix),
	}

	return []Article{connect, trouble}
}
