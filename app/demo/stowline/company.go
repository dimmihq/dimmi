/*
|--------------------------------------------------------------------------
| The Company
|--------------------------------------------------------------------------
|
| Stowline sells shipping software to online stores: it compares carrier
| rates, buys and prints labels, sends tracking updates, runs a returns
| portal and offers all of it through an API. The facts here are the
| ones every article, conversation and scenario draws on, so a carrier's
| pickup cut-off or a plan's label allowance reads the same everywhere.
|
*/

package stowline

/*
Name is the company the workspace belongs to.
*/
const Name = "Stowline"

/*
Slug marks the seeded workspace in its settings, so a later run can
find it again whatever address it was given.
*/
const Slug = "stowline"

/*
Domain is the made-up domain the company's people and links use.
*/
const Domain = "stowline.test"

/*
PricePlan is one of the company's price plans. A plan sold ByContract
has no list price: its MonthlyUSD is what such a contract usually comes
to, and the help center says only that the team prices it.
*/
type PricePlan struct {
	Key, Name     string
	MonthlyUSD    int
	ByContract    bool
	Labels        int
	Seats         int
	OverageCents  int
	SupportsSSO   bool
	SupportsAPI   bool
	ShareOfBuyers float64
}

/*
Plans returns the plans, cheapest first.
*/
func Plans() []PricePlan {
	return []PricePlan{
		{Key: "free", Name: "Free", MonthlyUSD: 0, Labels: 50, Seats: 1, OverageCents: 0, ShareOfBuyers: 0.38},
		{
			Key: "growth", Name: "Growth", MonthlyUSD: 49, Labels: 1500, Seats: 3, OverageCents: 5,
			SupportsAPI: true, ShareOfBuyers: 0.34,
		},
		{
			Key: "pro", Name: "Pro", MonthlyUSD: 149, Labels: 7500, Seats: 10, OverageCents: 4,
			SupportsAPI: true, ShareOfBuyers: 0.21,
		},
		{
			Key: "enterprise", Name: "Enterprise", MonthlyUSD: 899, ByContract: true, Labels: 60000, Seats: 50,
			OverageCents: 3,
			SupportsSSO:  true, SupportsAPI: true, ShareOfBuyers: 0.07,
		},
	}
}

/*
Carrier is one carrier stores ship with through Stowline.
*/
type Carrier struct {
	Key, Name, Country string
	Services           []string
	AccountField       string
	AccountExample     string
	PickupCutoff       string
	LabelFormats       []string
	MaxWeightKg        int
	ErrorCode          string
	ErrorText          string
	Quirk              string
}

/*
Carriers returns the carriers Stowline connects to.
*/
func Carriers() []Carrier {
	return []Carrier{
		{
			Key: "usps", Name: "USPS", Country: "United States",
			Services:     []string{"Ground Advantage", "Priority Mail", "Priority Mail Express", "Media Mail"},
			AccountField: "Stowline's own USPS account (no number needed)", AccountExample: "built-in",
			PickupCutoff: "the evening before, by 2:00 a.m. Central", LabelFormats: []string{"4x6 PDF", "4x6 ZPL", "8.5x11 PDF"},
			MaxWeightKg: 31, ErrorCode: "USPS-2010", ErrorText: "Address not found in the ZIP+4 database",
			Quirk: "Media Mail is only allowed for books, sound recordings and educational material, and USPS opens parcels to check.",
		},
		{
			Key: "ups", Name: "UPS", Country: "United States",
			Services:     []string{"UPS Ground", "UPS 2nd Day Air", "UPS Next Day Air", "UPS Worldwide Saver"},
			AccountField: "six-character UPS account number", AccountExample: "A1B2C3",
			PickupCutoff: "the same day, by 11:00 a.m. local time", LabelFormats: []string{"4x6 PDF", "4x6 ZPL", "4x8 ZPL"},
			MaxWeightKg: 68, ErrorCode: "UPS-120802", ErrorText: "Address validation failed: the street number is outside the valid range",
			Quirk: "UPS adds a residential surcharge after the label is bought when an address it thought was commercial turns out to be a home.",
		},
		{
			Key: "fedex", Name: "FedEx", Country: "United States",
			Services:     []string{"FedEx Ground", "FedEx Home Delivery", "FedEx 2Day", "FedEx International Priority"},
			AccountField: "nine-digit FedEx account number", AccountExample: "510087020",
			PickupCutoff: "the same day, by noon local time", LabelFormats: []string{"4x6 PDF", "4x6 ZPL", "4x6.75 ZPL with doc tab"},
			MaxWeightKg: 68, ErrorCode: "FEDEX-1237", ErrorText: "The recipient postal code does not match the city",
			Quirk: "FedEx requires the account to pass its end-user license check before the first label, which can take up to one business day.",
		},
		{
			Key: "dhl_express", Name: "DHL Express", Country: "Germany",
			Services:     []string{"Express Worldwide", "Express 12:00", "Economy Select"},
			AccountField: "nine-digit DHL Express import or export account", AccountExample: "960123456",
			PickupCutoff: "the same day, by 3:00 p.m. local time", LabelFormats: []string{"4x6 PDF", "4x6 ZPL", "A4 PDF with invoice"},
			MaxWeightKg: 70, ErrorCode: "DHL-420504", ErrorText: "Commodity code missing or invalid for a dutiable shipment",
			Quirk: "Every dutiable shipment needs a six-digit HS code on each line item, or DHL refuses the label.",
		},
		{
			Key: "canada_post", Name: "Canada Post", Country: "Canada",
			Services:     []string{"Regular Parcel", "Expedited Parcel", "Xpresspost", "Priority"},
			AccountField: "ten-digit customer number and contract number", AccountExample: "0004012345",
			PickupCutoff: "the day before, by 6:00 p.m. local time", LabelFormats: []string{"4x6 PDF", "8.5x11 PDF"},
			MaxWeightKg: 30, ErrorCode: "CPC-9122", ErrorText: "Contract number is not valid for the paying customer",
			Quirk: "Contract rates only apply when both the customer number and the contract number are saved on the connection.",
		},
		{
			Key: "royal_mail", Name: "Royal Mail", Country: "United Kingdom",
			Services:     []string{"Tracked 24", "Tracked 48", "Special Delivery Guaranteed", "International Tracked"},
			AccountField: "Click & Drop API key", AccountExample: "rm_live_9f3c...",
			PickupCutoff: "the day before, by 8:00 p.m. UK time", LabelFormats: []string{"4x6 PDF", "A4 PDF"},
			MaxWeightKg: 20, ErrorCode: "RM-E1101", ErrorText: "Service not available for this destination and weight",
			Quirk: "Tracked 24 and Tracked 48 stop at 20 kg; anything heavier needs a parcel carrier.",
		},
		{
			Key: "dpd", Name: "DPD", Country: "United Kingdom",
			Services:     []string{"DPD Next Day", "DPD Saturday", "DPD Classic", "DPD Air Express"},
			AccountField: "account number and GeoSession username", AccountExample: "1234567",
			PickupCutoff: "a standing daily collection agreed with your depot", LabelFormats: []string{"4x6 PDF", "4x6 ZPL"},
			MaxWeightKg: 30, ErrorCode: "DPD-0105", ErrorText: "Collection date is not a working day for the depot",
			Quirk: "DPD labels carry the collection date, so a label bought for Friday cannot be handed over on Monday.",
		},
		{
			Key: "australia_post", Name: "Australia Post", Country: "Australia",
			Services:     []string{"Parcel Post", "Express Post", "International Standard", "International Express"},
			AccountField: "MyPost Business or eParcel API key and account number", AccountExample: "1012345678",
			PickupCutoff: "the same day, by 1:00 p.m. local time", LabelFormats: []string{"4x6 PDF", "A4 PDF"},
			MaxWeightKg: 22, ErrorCode: "AP-44017", ErrorText: "Suburb and postcode combination is not valid",
			Quirk: "Australia Post checks the suburb and postcode as a pair, so a correct postcode with a misspelled suburb still fails.",
		},
		{
			Key: "purolator", Name: "Purolator", Country: "Canada",
			Services:     []string{"Purolator Ground", "Purolator Express", "Purolator Express 9AM"},
			AccountField: "registered account number and production key", AccountExample: "9999999999",
			PickupCutoff: "the same day, by 4:00 p.m. local time", LabelFormats: []string{"4x6 PDF", "4x6 ZPL"},
			MaxWeightKg: 68, ErrorCode: "PUR-1100636", ErrorText: "Production key is not activated for this account",
			Quirk: "Purolator issues test and production keys separately; a test key looks valid but never buys a real label.",
		},
		{
			Key: "gls", Name: "GLS", Country: "Netherlands",
			Services:     []string{"BusinessParcel", "ExpressParcel", "EuroBusinessParcel"},
			AccountField: "customer ID, contact ID and API user", AccountExample: "2764200001",
			PickupCutoff: "a standing daily collection agreed with your depot", LabelFormats: []string{"A6 PDF", "4x6 ZPL"},
			MaxWeightKg: 40, ErrorCode: "GLS-T8", ErrorText: "Recipient phone number required for this service",
			Quirk: "ExpressParcel and deliveries to parcel shops need the recipient's mobile number on the label.",
		},
	}
}

/*
Platform is one store or back-office system Stowline imports orders
from or sends data to.
*/
type Platform struct {
	Key, Name, Kind  string
	SyncMinutes      int
	ConnectsWith     string
	Fulfills         bool
	CommonProblem    string
	ProblemFix       string
	OrderNumberStyle string
}

/*
Platforms returns the integrations.
*/
func Platforms() []Platform {
	return []Platform{
		{
			Key: "shopify", Name: "Shopify", Kind: "store", SyncMinutes: 5, ConnectsWith: "the Stowline app from the Shopify App Store",
			Fulfills: true, OrderNumberStyle: "#1042",
			CommonProblem: "orders marked as fulfilled in Shopify before Stowline imports them never show up",
			ProblemFix:    "turn off automatic fulfillment in Shopify's checkout settings, or set the import filter to include fulfilled orders",
		},
		{
			Key: "woocommerce", Name: "WooCommerce", Kind: "store", SyncMinutes: 10, ConnectsWith: "a REST API key with read and write access",
			Fulfills: true, OrderNumberStyle: "#5521",
			CommonProblem: "a security plugin blocks the REST API and the connection test returns 401",
			ProblemFix:    "allow the /wp-json/wc/v3 routes in the security plugin and make sure permalinks are not set to Plain",
		},
		{
			Key: "bigcommerce", Name: "BigCommerce", Kind: "store", SyncMinutes: 5, ConnectsWith: "the Stowline app from the BigCommerce marketplace",
			Fulfills: true, OrderNumberStyle: "100231",
			CommonProblem: "orders in the Awaiting Payment status are skipped",
			ProblemFix:    "add Awaiting Payment to the statuses Stowline imports, or capture payment first",
		},
		{
			Key: "adobe_commerce", Name: "Adobe Commerce", Kind: "store", SyncMinutes: 15, ConnectsWith: "an integration token from System, Integrations",
			Fulfills: true, OrderNumberStyle: "000004521",
			CommonProblem: "multi-source inventory stores import orders without the right ship-from location",
			ProblemFix:    "map each Adobe Commerce source to a Stowline warehouse under Settings, Warehouses",
		},
		{
			Key: "amazon", Name: "Amazon Seller Central", Kind: "marketplace", SyncMinutes: 15, ConnectsWith: "authorizing Stowline as a developer in Seller Central",
			Fulfills: true, OrderNumberStyle: "113-4829156-2208222",
			CommonProblem: "tracking uploads are rejected because the ship date is in the future",
			ProblemFix:    "buy labels on the day they ship, or change the label date before printing",
		},
		{
			Key: "ebay", Name: "eBay", Kind: "marketplace", SyncMinutes: 15, ConnectsWith: "signing in to eBay from Settings, Integrations",
			Fulfills: true, OrderNumberStyle: "12-10345-88721",
			CommonProblem: "the eBay token expires after 18 months and orders silently stop importing",
			ProblemFix:    "reconnect eBay from Settings, Integrations; Stowline emails the account owner 14 days before the token expires",
		},
		{
			Key: "etsy", Name: "Etsy", Kind: "marketplace", SyncMinutes: 15, ConnectsWith: "signing in to Etsy from Settings, Integrations",
			Fulfills: true, OrderNumberStyle: "2918374650",
			CommonProblem: "personalization notes are cut off on the packing slip",
			ProblemFix:    "switch the packing slip template to the long notes layout under Settings, Packing slips",
		},
		{
			Key: "squarespace", Name: "Squarespace", Kind: "store", SyncMinutes: 10, ConnectsWith: "a Commerce API key from Squarespace's developer settings",
			Fulfills: true, OrderNumberStyle: "00418",
			CommonProblem: "digital products appear as orders that need shipping",
			ProblemFix:    "turn on Skip orders without physical items in the Squarespace import settings",
		},
		{
			Key: "wix", Name: "Wix", Kind: "store", SyncMinutes: 10, ConnectsWith: "the Stowline app from the Wix App Market",
			Fulfills: true, OrderNumberStyle: "10087",
			CommonProblem: "orders placed in a currency other than the store's default show the wrong customs value",
			ProblemFix:    "set the customs currency on the Wix connection to match the currency orders are paid in",
		},
		{
			Key: "netsuite", Name: "NetSuite", Kind: "erp", SyncMinutes: 30, ConnectsWith: "token-based authentication and the Stowline SuiteApp",
			Fulfills: true, OrderNumberStyle: "SO-48211",
			CommonProblem: "item fulfillments fail because the location on the sales order is empty",
			ProblemFix:    "give every sales order a location, or set a default location on the NetSuite connection",
		},
		{
			Key: "quickbooks", Name: "QuickBooks Online", Kind: "accounting", SyncMinutes: 60, ConnectsWith: "signing in to Intuit from Settings, Integrations",
			Fulfills: false, OrderNumberStyle: "INV-2291",
			CommonProblem: "shipping costs are posted to the wrong expense account",
			ProblemFix:    "pick the expense account under Settings, Integrations, QuickBooks, Accounts",
		},
		{
			Key: "zapier", Name: "Zapier", Kind: "automation", SyncMinutes: 0, ConnectsWith: "a Stowline API key pasted into Zapier",
			Fulfills: false, OrderNumberStyle: "any",
			CommonProblem: "the Label Created trigger fires twice when a label is voided and bought again",
			ProblemFix:    "filter on the event's label_id in the Zap, or use the Label Purchased trigger instead",
		},
	}
}

/*
Endpoint is one operation of Stowline's public API.
*/
type Endpoint struct {
	Method, Path, Title, Summary string
	Params                       [][3]string // Name, type and description of each.
	Request, Response            string
	Errors                       []string
	Notes                        string
}

/*
ErrorCode is one error the Stowline API and dashboard show.
*/
type ErrorCode struct {
	Code, Title, Meaning string
	Causes               []string
	Fixes                []string
	Status               int
}
