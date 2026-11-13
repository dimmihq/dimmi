/*
|--------------------------------------------------------------------------
| Long Guides
|--------------------------------------------------------------------------
|
| Forty guides written from the hand-written sections, each section a
| heading of its own. Guides overlap the way a real help center's do:
| the warehouse handbook and the onboarding guide both explain weights,
| one guide still carries the legacy section on presets beside the
| current one on rules, and some sections promise detail they never
| give. Searching them is as hard as searching a real one.
|
*/

package stowline

import "strings"

/*
guide is one guide: its section, title, introduction and sections.
*/
type guide struct {
	collection, title, intro string
	sections                 []string
}

/*
guides lists every guide.
*/
func guides() []guide {
	return []guide{
		{
			"getting-started", "The Stowline handbook", "Everything a new team needs, in one long page, from the first label to peak season.",
			[]string{"why-weight", "dim-weight", "saved-packages", "rules-basics", "rules-cheapest", "rules-testing", "connect-carrier", "usps-builtin", "balance-basics", "balance-refunds", "void-windows", "void-how", "adjustments", "thermal-setup", "thermal-trouble", "print-helper", "customs-basics", "duties", "returns-setup", "returns-resolutions", "tracking-gaps", "claims", "batches", "pickups", "manifests", "peak", "support-hours"},
		},
		{
			"getting-started", "Onboarding guide for new stores", "The first week with Stowline, step by step.",
			[]string{"warehouses", "saved-packages", "why-weight", "connect-carrier", "usps-builtin", "shopify-connect", "rules-basics", "thermal-setup", "print-helper", "balance-basics", "support-hours"},
		},
		{
			"getting-started", "Moving to Stowline from another shipping tool", "How to bring your setup over and switch without losing a day of shipping.",
			[]string{"migration-import", "migration-cutover", "rules-basics", "rules-legacy-presets", "saved-packages", "connect-carrier", "connect-carrier-legacy", "shopify-connect", "woo-connect", "woo-legacy", "thermal-setup", "plans"},
		},
		{
			"getting-started", "Warehouse operations handbook", "For the people packing and printing: weights, boxes, printers, batches and pickups.",
			[]string{"why-weight", "dim-weight", "dim-weight-legacy", "saved-packages", "thermal-setup", "thermal-trouble", "print-helper", "batches", "pickups", "manifests", "void-how", "address-validation", "peak", "peak-partial"},
		},
		{
			"shipping-labels", "Cutting shipping costs", "Where shipping money goes and how stores bring it down.",
			[]string{"dim-weight", "why-weight", "saved-packages", "rules-cheapest", "adjustments", "insurance-vs-declared", "usps-builtin", "duties", "plans"},
		},
		{
			"shipping-labels", "Everything about voids and refunds", "Voiding labels, when the money comes back and where it goes.",
			[]string{"void-how", "void-windows", "balance-refunds", "balance-basics", "adjustments", "adjustments-legacy", "support-hours"},
		},
		{
			"shipping-labels", "Printing labels: the complete guide", "Printers, formats, the print helper and every printing problem we see.",
			[]string{"thermal-setup", "thermal-trouble", "print-helper", "batches", "support-hours"},
		},
		{
			"shipping-labels", "Automation rules in depth", "How rules decide, how to test them and the mistakes teams make.",
			[]string{"rules-basics", "rules-cheapest", "rules-testing", "rules-legacy-presets", "saved-packages", "connect-carrier"},
		},
		{
			"shipping-labels", "Batch shipping playbook", "Running hundreds of labels a day without stalls.",
			[]string{"batches", "rules-basics", "balance-basics", "failed-payment", "status-page", "print-helper", "pickups", "manifests"},
		},
		{
			"carriers", "Choosing carriers and services", "How to decide which carriers to connect and which services to use.",
			[]string{"usps-builtin", "connect-carrier", "dim-weight", "rules-cheapest", "insurance-vs-declared", "pickups", "duties"},
		},
		{
			"carriers", "Carrier accounts: connecting and troubleshooting", "Connecting your own carrier accounts and fixing them when they break.",
			[]string{"connect-carrier", "connect-carrier-legacy", "usps-builtin", "status-page", "address-validation", "support-hours"},
		},
		{
			"carriers", "Pickups, manifests and handover", "Getting parcels from your door to the carrier.",
			[]string{"pickups", "manifests", "tracking-gaps", "peak", "peak-partial"},
		},
		{
			"integrations", "Shopify complete guide", "Connecting Shopify, what syncs, and fixing missing orders.",
			[]string{"shopify-connect", "shopify-missing", "rules-basics", "rules-testing", "returns-resolutions", "warehouses"},
		},
		{
			"integrations", "WooCommerce complete guide", "Connecting WooCommerce with a REST key and fixing connection errors.",
			[]string{"woo-connect", "woo-legacy", "rules-testing", "returns-resolutions", "address-validation"},
		},
		{
			"integrations", "Selling on marketplaces", "Amazon, eBay and Etsy with Stowline.",
			[]string{"marketplaces", "rules-testing", "customs-basics", "tracking-gaps"},
		},
		{
			"integrations", "Multi-channel selling guide", "Running several stores and marketplaces from one Stowline account.",
			[]string{"shopify-connect", "woo-connect", "marketplaces", "warehouses", "rules-basics", "rules-testing", "migration-cutover"},
		},
		{
			"tracking-returns", "Returns program guide", "Designing a returns policy and running it in Stowline.",
			[]string{"returns-setup", "returns-resolutions", "returns-legacy", "claims", "duties"},
		},
		{
			"tracking-returns", "When parcels go missing", "Tracking gaps, delivered-but-not-received, traces and claims.",
			[]string{"tracking-gaps", "delivered-not-received", "claims", "insurance-vs-declared", "status-page"},
		},
		{
			"tracking-returns", "Tracking and customer notifications", "What your customers see after you buy a label.",
			[]string{"tracking-gaps", "delivered-not-received", "webhooks-basics", "shopify-connect"},
		},
		{
			"billing", "Billing handbook", "Plans, balance, top-ups, invoices, adjustments and refunds in one place.",
			[]string{"plans", "plans-legacy", "balance-basics", "balance-refunds", "failed-payment", "adjustments", "adjustments-legacy", "void-windows", "delete-account"},
		},
		{
			"billing", "Understanding your shipping bill", "Why a label cost what it did, and what came later.",
			[]string{"dim-weight", "dim-weight-legacy", "adjustments", "insurance-vs-declared", "balance-basics", "duties"},
		},
		{
			"billing", "Plans for growing stores", "Choosing a plan as volume grows.",
			[]string{"plans", "plans-legacy", "batches", "rules-basics", "returns-setup", "sso"},
		},
		{
			"account-security", "Security and compliance overview", "How Stowline protects your account and your customers' data.",
			[]string{"security-data", "security-partial", "sso", "two-factor", "team-roles", "data-export", "delete-account"},
		},
		{
			"account-security", "Administering a large team", "Roles, sign-in policies and keeping the account tidy.",
			[]string{"team-roles", "sso", "two-factor", "api-auth", "data-export", "support-hours"},
		},
		{
			"account-security", "Your data and privacy", "What we store, for how long, and how to get it out.",
			[]string{"security-data", "data-export", "delete-account", "security-partial"},
		},
		{
			"api", "Developer guide: building a shipping integration", "The concepts every integration needs, before the reference.",
			[]string{"api-auth", "api-idempotency", "api-limits", "webhooks-basics", "webhooks-signatures", "address-validation", "customs-basics", "void-how", "data-export"},
		},
		{
			"api", "Webhooks guide", "Receiving, verifying and processing events reliably.",
			[]string{"webhooks-basics", "webhooks-signatures", "api-idempotency", "tracking-gaps"},
		},
		{
			"api", "API reliability guide", "Retries, idempotency, limits and outages.",
			[]string{"api-idempotency", "api-limits", "status-page", "api-auth", "batches"},
		},
		{
			"troubleshooting", "Troubleshooting guide: labels and printing", "Every label and printing problem we see, and the fix.",
			[]string{"thermal-trouble", "thermal-setup", "print-helper", "address-validation", "address-partial", "void-windows", "status-page"},
		},
		{
			"troubleshooting", "Troubleshooting guide: rates and costs", "When prices look wrong.",
			[]string{"dim-weight", "dim-weight-legacy", "why-weight", "adjustments", "rules-cheapest", "connect-carrier"},
		},
		{
			"troubleshooting", "Troubleshooting guide: integrations", "Orders not importing, tracking not going back, duplicates.",
			[]string{"shopify-missing", "woo-connect", "woo-legacy", "marketplaces", "rules-testing", "migration-cutover"},
		},
		{
			"troubleshooting", "Troubleshooting guide: billing", "Declined cards, missing refunds and unexpected charges.",
			[]string{"failed-payment", "balance-refunds", "void-windows", "adjustments", "plans-legacy"},
		},
		{
			"troubleshooting", "Troubleshooting guide: international", "Customs holds, duties disputes and refused parcels.",
			[]string{"customs-basics", "duties", "duties-partial", "address-partial", "tracking-gaps", "claims"},
		},
		{
			"troubleshooting", "Troubleshooting guide: account access", "Locked out, lost phone, SSO broken.",
			[]string{"two-factor", "sso", "team-roles", "support-hours"},
		},
		{
			"shipping-labels", "International shipping guide", "Customs, duties, carriers and returns across borders.",
			[]string{"customs-basics", "duties", "duties-partial", "connect-carrier", "dim-weight", "returns-setup", "tracking-gaps", "claims"},
		},
		{
			"shipping-labels", "Peak season guide", "Preparing for Black Friday to New Year.",
			[]string{"peak", "peak-partial", "balance-basics", "batches", "pickups", "status-page", "print-helper", "thermal-trouble"},
		},
		{
			"getting-started", "Glossary of shipping terms", "The words carriers and this help center use, explained in context.",
			[]string{"dim-weight", "manifests", "duties", "adjustments", "void-windows", "rules-legacy-presets", "insurance-vs-declared"},
		},
		{
			"getting-started", "Frequently asked questions", "The questions we answer most, collected.",
			[]string{"void-windows", "tracking-gaps", "thermal-trouble", "plans", "failed-payment", "address-validation", "shopify-missing", "customs-basics", "support-hours"},
		},
		{
			"getting-started", "Stowline for small shops", "Shipping from home with one person and a printer.",
			[]string{"usps-builtin", "thermal-setup", "why-weight", "plans", "plans-legacy", "returns-legacy", "support-hours"},
		},
		{
			"getting-started", "Stowline for enterprise brands", "Many warehouses, many people, strict controls.",
			[]string{"warehouses", "connect-carrier", "team-roles", "sso", "security-data", "security-partial", "api-auth", "data-export", "support-hours"},
		},
	}
}

/*
guideArticles renders the guides.
*/
func guideArticles() []Article {
	sections := guideSections()
	out := make([]Article, 0, len(guides()))
	for _, g := range guides() {
		var b strings.Builder
		b.WriteString(g.intro + "\n\n")
		for _, key := range g.sections {
			s := sections[key]
			b.WriteString("## " + s[0] + "\n\n" + s[1] + "\n\n")
		}
		out = append(out, Article{Collection: g.collection, Title: g.title, Description: g.intro, Body: b.String()})
	}
	return out
}
