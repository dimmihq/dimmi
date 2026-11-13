/*
|--------------------------------------------------------------------------
| The Team's Vocabulary
|--------------------------------------------------------------------------
|
| The tags Stowline's team files conversations under, the saved replies
| they reach for every day, and the attributes their customers carry.
|
*/

package stowline

/*
Tags returns each tag's name and color.
*/
func Tags() [][2]string {
	return [][2]string{
		{"voids", "orange"},
		{"tracking", "blue"},
		{"address-validation", "yellow"},
		{"plans", "green"},
		{"payments", "red"},
		{"integrations", "purple"},
		{"api", "gray"},
		{"adjustments", "orange"},
		{"printing", "blue"},
		{"international", "teal"},
		{"returns", "pink"},
		{"carriers", "blue"},
		{"batches", "gray"},
		{"sso", "purple"},
		{"cancellation", "red"},
		{"claims", "yellow"},
		{"onboarding", "green"},
		{"enterprise", "purple"},
		{"feature-request", "teal"},
		{"pickups", "gray"},
		{"automation", "green"},
		{"account-access", "red"},
		{"urgent", "red"},
		{"bug", "orange"},
		{"non-english", "teal"},
		{"vip", "purple"},
	}
}

/*
SavedReplies returns each reply's title, text and shortcut.
*/
func SavedReplies() [][3]string {
	return [][3]string{
		{"Void refund timing", "Voided labels are refunded to your Stowline balance once the carrier confirms the label was never scanned. For USPS that is 14 to 21 days after the void; for most other carriers it is within 7 days. You can follow it under **Billing → Balance history**.", "void"},
		{"Ask for the order number", "Thanks for reaching out! Could you send me the order number or the tracking number? That lets me look at exactly what the carrier answered.", "order"},
		{"Open a carrier trace", "I've opened a trace with the carrier for this parcel. They usually reply within 3 business days, and I'll update you here as soon as I hear back.", "trace"},
		{"Ship to unverified address", "If you're sure the address is right, open the order and turn on **Ship to unverified address**. The label will buy and the carrier will deliver it as written.", "unverified"},
		{"Thermal printer settings", "Please print with paper size 4x6 in, scale 100% (or *Actual size*) and no margins. *Fit to page* shrinks the barcode, and some carriers reject shrunk barcodes.", "print"},
		{"Adjustment dispute", "You can dispute this adjustment within 30 days. If you send me a photo of the box with a tape measure visible, I'll file the dispute with the carrier for you.", "dispute"},
		{"Plans overview", "Growth includes 1,500 labels a month for $49, Pro includes 7,500 for $149 with the returns portal, and Enterprise adds single sign-on and a dedicated success manager. Upgrades are prorated to the day.", "plans"},
		{"Failed payment steps", "To restore label buying, add a new card under **Billing → Payment method** and click **Top up now**. It's back within a minute of a successful payment.", "card"},
		{"Reapply automation rules", "Automation rules only apply to orders imported after they're saved. For orders already waiting, select them and click **Reapply rules** on the Orders page.", "rules"},
		{"Feature request logged", "Thanks for the idea! I've added it to our feedback board with your use case. You'll get an email if it ships.", "idea"},
		{"Closing check-in", "I'll close this conversation for now. If anything comes up, just reply here and it reopens with the whole history.", "close"},
		{"Escalated to engineering", "I've passed this to our engineers with the request ids and the steps to reproduce. I'll update you as soon as I hear back, at the latest by end of day tomorrow.", "eng"},
		{"HS codes for customs", "DHL Express and most EU and UK destinations now need a six-digit HS code on every item. Save it on each product under **Products** and it fills in automatically.", "hs"},
		{"Webhook raw body", "The signature is computed over the timestamp, a dot and the **raw** request body. If your framework parses the JSON before you hash it, the bytes change and the signature never matches.", "rawbody"},
		{"Recovery codes", "If you saved recovery codes when you set up two-factor, any one of them gets you in. Otherwise, please reply from the email address on the account so we can verify ownership.", "2fa"},
	}
}

/*
attributeDefinitions are the custom attributes on people and
companies: object, key, label, description, type and options.
*/
func attributeDefinitions() [][6]string {
	return [][6]string{
		{"contact", "plan", "Plan", "The Stowline plan the person's account is on.", "string", `["free", "growth", "pro", "enterprise"]`},
		{"contact", "role", "Account role", "Whether the person administers their Stowline account.", "string", `["admin", "member"]`},
		{"contact", "labels_last_30_days", "Labels in the last 30 days", "Labels bought on the person's account in the last 30 days.", "number", `[]`},
		{"contact", "platform", "Store platform", "Where the person's orders come from.", "string", `[]`},
		{"company", "plan", "Plan", "The company's Stowline plan.", "string", `["growth", "pro", "enterprise"]`},
		{"company", "seats", "Seats", "Teammates on the company's account.", "number", `[]`},
		{"company", "monthly_labels", "Monthly labels", "Labels the company buys in a typical month.", "number", `[]`},
		{"company", "industry", "Industry", "What the company sells.", "string", `[]`},
		{"company", "platform", "Store platform", "The company's main store platform.", "string", `[]`},
		{"company", "country", "Country", "Where the company ships from.", "string", `[]`},
	}
}
