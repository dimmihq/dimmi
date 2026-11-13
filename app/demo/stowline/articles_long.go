/*
|--------------------------------------------------------------------------
| Long Reference Documents
|--------------------------------------------------------------------------
|
| The documents a real shipping help center grows into: a complete guide
| per carrier with its services, zones, rate tables, surcharges, limits,
| label specifications and holiday calendar; a page per carrier service;
| an error reference per product area; release notes going back two
| years; and the all-in-one documents teams export, which repeat the
| articles they gather. Tables are computed from each carrier's own
| facts, so no two carriers share a number.
|
*/

package stowline

import (
	"fmt"
	"slices"
	"strings"
)

/*
carrierGuide is the complete guide to one carrier.
*/
func carrierGuide(i int, c Carrier) Article {
	var b strings.Builder
	fmt.Fprintf(&b, "This is the complete reference for shipping with %s through Stowline: every service, how %s "+
		"prices by zone and weight, what it adds on top, what it accepts, how its labels must be printed, "+
		"when it collects and when it does not deliver. Shorter articles cover connecting %s and fixing its "+
		"errors; this page is for when you need the detail.\n\n", c.Name, c.Name, c.Name)
	b.WriteString(carrierServices(i, c))
	b.WriteString(carrierRates(i, c))
	b.WriteString(carrierSurcharges(i, c))
	b.WriteString(carrierLimits(i, c))
	b.WriteString(carrierLabels(c))
	b.WriteString(carrierCalendar(i, c))
	b.WriteString(carrierInternational(c))
	b.WriteString(carrierClaims(i, c))
	for _, a := range carrierArticles(c) {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", a.Title, demote(a.Body))
	}
	return Article{
		Collection: "carriers", Title: "The complete " + c.Name + " guide",
		Description: "Services, zones, rates, surcharges, limits, labels and holidays for " + c.Name + ".",
		Body:        b.String(),
	}
}

func carrierServices(i int, c Carrier) string {
	rows := [][]string{}
	for k, s := range c.Services {
		days := []string{"3 to 7", "2 to 5", "1 to 3", "1"}[min(k, 3)]
		guarantee := map[bool]string{true: "Money-back guarantee, suspended in peak season", false: "No guarantee"}[k >= 2]
		rows = append(rows, []string{
			s, days + " business days", guarantee,
			fmt.Sprintf("%d kg", max(c.MaxWeightKg-k*(2+i%3), 2)), map[bool]string{true: "Yes", false: "No"}[k%2 == 0],
		})
	}
	return "## Services\n\n" + table([]string{"Service", "Transit", "Guarantee", "Weight limit", "Signature available"}, rows) +
		fmt.Sprintf("\nTransit times count from the first scan, not the label date, and assume the parcel was handed "+
			"over before the day's cut-off. %s publishes guaranteed times only for its faster services; the "+
			"guarantee covers the postage, never the goods, and is claimed from %s through Stowline's refund "+
			"request on the label page.\n\n", c.Name, c.Name)
}

func carrierRates(i int, c Carrier) string {
	var b strings.Builder
	b.WriteString("## Zones and sample rates\n\n")
	fmt.Fprintf(&b, "%s prices by zone, counted from the distance between the origin and destination postal codes, "+
		"and by billable weight, the greater of the actual and the dimensional weight. These are sample commercial "+
		"rates in USD for a parcel from a %s warehouse; your own account may have negotiated rates, and fuel and "+
		"surcharges are added on top.\n\n", c.Name, c.Country)
	for k, s := range c.Services {
		rows := [][]string{}
		for _, lb := range []int{1, 2, 3, 4, 5, 7, 10, 15, 20, 25, 30, 40, 50, 70} {
			if lb > c.MaxWeightKg*2 {
				break
			}
			row := []string{fmt.Sprintf("%d lb", lb)}
			for zone := 2; zone <= 8; zone++ {
				price := 6.2 + float64(i)*0.37 + float64(k)*4.1*float64(k+1) + float64(lb)*(0.55+0.08*float64(zone)) +
					float64(zone)*0.9*float64(k+1)
				row = append(row, fmt.Sprintf("%.2f", price))
			}
			rows = append(rows, row)
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n", s, table([]string{
			"Weight", "Zone 2", "Zone 3", "Zone 4", "Zone 5", "Zone 6",
			"Zone 7", "Zone 8",
		}, rows))
	}
	divisor := []int{139, 139, 139, 139, 166, 5000, 5000, 4000, 139, 5000}[i%10]
	fmt.Fprintf(&b, "Dimensional weight on %s is length × width × height divided by %d (inches and pounds, or "+
		"centimeters and kilograms for divisors of 4000 and above). A 14 × 12 × 10 box is billed at least as "+
		"heavy as the result, rounded up to the next whole unit.\n\n", c.Name, divisor)
	return b.String()
}

func carrierSurcharges(i int, c Carrier) string {
	names := []string{
		"Residential delivery", "Delivery area, extended", "Delivery area, remote", "Additional handling, weight",
		"Additional handling, dimensions", "Large package", "Saturday delivery", "Signature required", "Adult signature",
		"Address correction", "Fuel", "Peak season, per parcel",
	}
	when := []string{
		"Delivered to a home", "Postal code on the carrier's extended list", "Postal code on the remote list",
		"Over 50 lb", "Longest side over 48 in", "Length plus girth over 130 in", "Requested on the label",
		"Requested on the label", "Requested on the label", "The carrier corrected the address",
		"Every parcel", "Between late November and mid January",
	}
	rows := [][]string{}
	for k, n := range names {
		amount := fmt.Sprintf("$%.2f", 1.1+float64((i*7+k*13)%40)*0.45)
		if n == "Fuel" {
			amount = fmt.Sprintf("%.1f%% of the base rate, updated weekly", 12.5+float64(i%5)*1.25)
		}
		rows = append(rows, []string{n, amount, when[k]})
	}
	return "## Surcharges\n\n" + table([]string{"Surcharge", "Amount", "Applies when"}, rows) +
		fmt.Sprintf("\nResidential and address correction surcharges are often only known after %s handles the "+
			"parcel, so they arrive as adjustments. Keep residential detection on to price them up front.\n\n", c.Name)
}

func carrierLimits(i int, c Carrier) string {
	return "## Size and weight limits\n\n" + table([]string{"Limit", "Value"}, [][]string{
		{"Maximum weight", fmt.Sprintf("%d kg", c.MaxWeightKg)},
		{"Maximum length", fmt.Sprintf("%d cm", 120+i*10)},
		{"Maximum length plus girth", fmt.Sprintf("%d cm", 300+i*12)},
		{"Minimum size", "14 × 9 cm label face"},
		{"Pieces per multi-parcel shipment", fmt.Sprint(20 + i*5)},
		{"Declared value without signature", fmt.Sprintf("$%d", 500+i*100)},
	}) + "\nParcels over any limit are refused at label purchase with the limit named in the error. " +
		"Some services have lower limits; the service's own limit is shown on its rate.\n\n" + c.Quirk + "\n\n"
}

func carrierLabels(c Carrier) string {
	rows := [][]string{}
	for _, f := range c.LabelFormats {
		thermal := strings.Contains(f, "ZPL") || strings.Contains(f, "4x6")
		rows = append(rows, []string{
			f, map[bool]string{true: "Thermal printers", false: "Laser or inkjet"}[thermal],
			map[bool]string{true: "203 dpi", false: "300 dpi"}[strings.Contains(f, "ZPL")],
		})
	}
	return "## Label specifications\n\n" + table([]string{"Format", "Printer", "Resolution"}, rows) +
		"\nPrint at 100% scale. A barcode shrunk to fit the page can be refused at the counter or scanned as " +
		"unreadable, and the parcel then waits for a manual entry, which shows as a gap in tracking.\n\n" +
		"Labels carry the ship date. A label printed for one day and handed over days later may be refused or " +
		"surcharged; reprint it with a new date from the label page instead of buying a new one.\n\n"
}

func carrierCalendar(i int, c Carrier) string {
	days := [][]string{
		{"1 January", "New Year's Day", "No pickups or deliveries"},
		{"Third Monday of January", "Regional holiday", "Pickups only"},
		{"Last Monday of May", "Spring holiday", "No pickups or deliveries"},
		{"4 July", "Independence Day (US services)", "No pickups or deliveries"},
		{"First Monday of September", "Labor Day", "Express only"},
		{"Fourth Thursday of November", "Thanksgiving", "No pickups or deliveries"},
		{"24 December", "Christmas Eve", "Early cut-off, pickups by noon"},
		{"25 December", "Christmas Day", "No pickups or deliveries"},
		{"26 December", "Boxing Day (UK, Canada, Australia)", "No deliveries"},
		{"31 December", "New Year's Eve", "Early cut-off"},
	}
	rows := make([][]string, 0, len(days))
	for k, d := range days {
		if (k+i)%5 != 4 {
			rows = append(rows, d)
		}
	}
	return "## Holiday calendar\n\n" + table([]string{"Date", "Holiday", "Service"}, rows) +
		fmt.Sprintf("\nDelivery estimates skip these days automatically. Booking a pickup on one of them offers the "+
			"next working day instead. Pickup cut-off: %s.\n\n", c.PickupCutoff)
}

func carrierInternational(c Carrier) string {
	return "## International shipments\n\n" + steps(
		"Every item needs a description a customs officer would understand, its value, weight, origin and a six-digit HS code.",
		"Choose who pays duties: DAP by default, or DDP where "+c.Name+" supports it, billed to you afterwards.",
		"Add IOSS for the EU and a UK VAT number for the UK under Settings, Customs, so they travel with every declaration.",
		"Paperless trade is used wherever the destination accepts it; otherwise the commercial invoice prints with the label.",
	) + "\nThe most common refusal is " + c.ErrorCode + ": " + c.ErrorText + ". See *" + c.Name +
		" errors and how to fix them*.\n\n"
}

func carrierClaims(i int, c Carrier) string {
	return "## Claims and liability\n\n" + fmt.Sprintf("%s's own liability is $%d per parcel unless you declare a "+
		"higher value, for a fee shown on the rate. Claims for declared value are filed with %s, who decides "+
		"them under their terms, usually within %d business days. Stowline insurance is an alternative for any "+
		"carrier and is claimed through Stowline. Lost parcels can be claimed after 10 days without a scan "+
		"(20 for international); damaged ones straight away, with photos of the item and the box.\n\n",
		c.Name, 50+i*25, c.Name, 10+i)
}

/*
serviceArticles are one page per carrier service: when to choose it,
its rates by zone, its limits and its common problems.
*/
func serviceArticles() []Article {
	var out []Article
	for i, c := range Carriers() {
		for k, s := range c.Services {
			out = append(out, serviceArticle(i, k, c, s))
		}
	}
	return out
}

func serviceArticle(i, k int, c Carrier, s string) Article {
	var b strings.Builder
	fmt.Fprintf(&b, "%s is %s's %s service. This page covers when to choose it, what it costs by zone "+
		"and weight, its limits and the options it supports.\n\n", s, c.Name,
		[]string{"economy", "standard", "express", "fastest"}[min(k, 3)])
	fmt.Fprintf(&b, "## When to choose %s\n\n%s\n\n", s, []string{
		"Choose it for light, non-urgent parcels where price matters more than speed. Transit is 3 to 7 business days and there is no delivery guarantee.",
		"Choose it as the everyday service for most orders: reliable transit of 2 to 5 business days and full tracking.",
		"Choose it when customers pay for faster delivery or the order is time-sensitive. Transit is 1 to 3 business days with a guarantee outside peak season.",
		"Choose it for urgent replacements and high-value orders. Next business day or time-definite delivery, guaranteed outside peak season.",
	}[min(k, 3)])
	b.WriteString(carrierRates(i, Carrier{Name: c.Name, Country: c.Country, Services: []string{s}, MaxWeightKg: c.MaxWeightKg}))
	fmt.Fprintf(&b, "## Limits and options\n\n%s\n", table([]string{"Item", "Value"}, [][]string{
		{"Maximum weight", fmt.Sprintf("%d kg", max(c.MaxWeightKg-k*(2+i%3), 2))},
		{"Signature confirmation", map[bool]string{true: "Available", false: "Not available"}[k%2 == 0]},
		{"Saturday delivery", map[bool]string{true: "Available for a surcharge", false: "Not available"}[k >= 2]},
		{"Insurance", "Stowline insurance up to $5,000, or carrier declared value"},
		{"Label formats", strings.Join(c.LabelFormats, ", ")},
		{"Pickup cut-off", c.PickupCutoff},
	}))
	fmt.Fprintf(&b, "\n## Common problems\n\nThe error we see most with %s is %s: %s. %s\n", c.Name, c.ErrorCode,
		c.ErrorText, c.Quirk)
	return Article{
		Collection: "carriers", Title: c.Name + " " + s, Slug: Slugify(c.Name + " " + s + " service"),
		Description: "Rates, limits and options for " + c.Name + " " + s + ".", Body: b.String(),
	}
}

/*
errorAreas groups the error codes into the areas each reference page
covers, with an introduction per area.
*/
func errorAreas() []struct{ Title, Intro string } {
	return []struct{ Title, Intro string }{
		{"Shipments and labels", "Errors when creating shipments, buying, voiding and reprinting labels."},
		{"Addresses and rating", "Errors when verifying addresses and fetching rates."},
		{"Customs and international", "Errors from customs declarations and international services."},
		{"Accounts, billing and limits", "Errors about your balance, plan and API limits."},
		{"Integrations, webhooks and the API", "Errors from store connections, webhooks and API requests."},
	}
}

/*
errorReference is the long reference page of one area: every code in
it with its causes, fixes and how to handle it in code.
*/
func errorReference(area int) Article {
	a := errorAreas()[area]
	var b strings.Builder
	fmt.Fprintf(&b, "%s Each code below has its own section: what it means, why it happens, how to fix it in "+
		"the dashboard and how to handle it in an integration.\n\n", a.Intro)
	for k, e := range ErrorCodes() {
		if k%len(errorAreas()) != area {
			continue
		}
		fmt.Fprintf(&b, "## %s: %s\n\nHTTP status %d. %s\n\nWhy it happens:\n\n%s\nHow to fix it:\n\n%s\n%s\n",
			e.Code, e.Title, e.Status, e.Meaning, list(e.Causes...), steps(e.Fixes...), demote(handling(e)))
	}
	return Article{
		Collection: "troubleshooting", Title: "Error reference: " + a.Title, Description: a.Intro,
		Body: b.String(),
	}
}

/*
releaseArticles are two years of monthly release notes, and all of
them in one document.
*/
func releaseArticles() []Article {
	notes := releaseNotes()
	months := []string{
		"January", "February", "March", "April", "May", "June", "July", "August", "September",
		"October", "November", "December",
	}
	out := make([]Article, 0, 25)
	for m := range 24 {
		year, month := 2024+(m+10)/12, months[(m+10)%12]
		var b strings.Builder
		fmt.Fprintf(&b, "What changed in Stowline in %s %d. Changes reach every account on the date shown unless "+
			"a note says otherwise.\n\n", month, year)
		for k := range 3 {
			n := notes[(m*3+k)%len(notes)]
			fmt.Fprintf(&b, "## %s\n\n%s\n\n**Who it affects:** %s\n\n**How to use it:** %s\n\n", n[0], n[1], n[2], n[3])
		}
		out = append(out, Article{
			Collection: "getting-started", Title: fmt.Sprintf("Release notes: %s %d", month, year),
			Slug:        fmt.Sprintf("release-notes-%d-%02d", year, (m+10)%12+1),
			Description: fmt.Sprintf("Everything that changed in %s %d.", month, year), Body: b.String(),
		})
	}
	return append(out, gathered("getting-started", "Release notes: every change since November 2024",
		"Two years of release notes in one document.", out)...)
}

/*
sectionArticles publish each guide section as an article of its own,
as help centers do with the answers their guides repeat.
*/
func sectionArticles() []Article {
	sections := guideSections()
	keys := make([]string, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]Article, 0, len(keys))
	for _, k := range keys {
		s := sections[k]
		out = append(out, Article{
			Collection: sectionCollection(k), Title: s[0], Description: firstSentence(s[1]),
			Body: s[1],
		})
	}
	return out
}

/*
sectionCollection files a section's own article where its guide sits.
*/
func sectionCollection(key string) string {
	for _, g := range guides() {
		if slices.Contains(g.sections, key) {
			return g.collection
		}
	}
	return "getting-started"
}

/*
completeReferences gather each section's articles into all-in-one
documents of about 6,000 words, up to three per section, the kind teams
export, and the API reference by area.
*/
func completeReferences(all []Article) []Article {
	var out []Article
	for _, c := range Collections() {
		if c.Slug == "api" {
			continue
		}
		var pages []Article
		for _, a := range all {
			if a.Collection == c.Slug && a.Visibility != "agent_only" {
				pages = append(pages, a)
			}
		}
		docs := gathered(c.Slug, c.Name+": the complete reference",
			"Articles in "+c.Name+", gathered in one document. The individual articles are kept up to date first; "+
				"this document is regenerated weekly and may lag behind them.", pages)
		out = append(out, docs[:min(len(docs), 3)]...)
	}
	return append(out, apiAreaReferences()...)
}

/*
apiAreaReferences gather the API reference into documents by area, as
the downloadable reference does.
*/
func apiAreaReferences() []Article {
	areas := []struct {
		title string
		keys  []string
	}{
		{"Shipments, rates and addresses", []string{"shipment", "rate", "address"}},
		{"Tracking, returns and refunds", []string{"tracker", "return", "refund", "insurance"}},
		{"Pickups, manifests and batches", []string{"pickup", "manifest", "batch"}},
		{"Webhooks and events", []string{"webhook", "event"}},
		{"Account, billing and keys", []string{"carrier_account", "balance_transaction", "adjustment", "api_key", "warehouse", "report"}},
		{"Orders and customs", []string{"order", "customs"}},
	}
	var out []Article
	for _, area := range areas {
		var pages []Article
		for _, r := range Resources() {
			if !slices.Contains(area.keys, r.Key) {
				continue
			}
			for _, op := range operations(r) {
				pages = append(pages, operationPage(r, op))
			}
		}
		out = append(out, gathered("api", "API reference: "+area.title,
			"The API reference for "+strings.ToLower(area.title)+" in one document. Each operation also has its own page.",
			pages)...)
	}
	return out
}

/*
gathered collects pages into documents of about 6,000 words, numbered
when there is more than one.
*/
func gathered(collection, title, intro string, pages []Article) []Article {
	var docs []*strings.Builder
	words := 6_000
	for _, p := range pages {
		if words >= 6_000 {
			docs = append(docs, &strings.Builder{})
			_, _ = docs[len(docs)-1].WriteString(intro + "\n\n")
			words = 0
		}
		_, _ = fmt.Fprintf(docs[len(docs)-1], "## %s\n\n%s\n\n", p.Title, demote(p.Body))
		words += WordCount(p.Body)
	}
	out := make([]Article, 0, len(docs))
	for n, d := range docs {
		t := title
		if len(docs) > 1 {
			t = fmt.Sprintf("%s, part %d", title, n+1)
		}
		out = append(out, Article{Collection: collection, Title: t, Description: intro, Body: d.String()})
	}
	return out
}
