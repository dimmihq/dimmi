/*
|--------------------------------------------------------------------------
| Tool Catalog: Help Center Sites
|--------------------------------------------------------------------------
|
| The entries for adding and deleting help centers, the custom domain a
| help center answers on, and what its readers read and searched for.
|
*/

package mcpserver

/*
helpCenterSiteDocs returns the entries for help centers' lifecycle,
domains and analytics.
*/
func helpCenterSiteDocs() map[string]Doc {
	return map[string]Doc{
		"help-centers.store": {
			Title: "Add a help center",
			Use: "Adds another help center to the workspace, such as one per product or language. It starts " +
				"empty and active at its own address.",
			Returns: "the help center with its public url.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"name":      "The help center's name, 1 to 80 characters.",
				"subdomain": "Its address; taken from the name, made unique, when left out.",
				"locale":    "Language code such as en or fr; the workspace's language when left out.",
			},
		},
		"help-centers.destroy": {
			Title: "Delete a help center",
			Use: "Deletes a help center that is not the default, with its articles and custom domain. Messengers " +
				"that showed it show the default one. Only owners may; confirm must be its name exactly.",
			Risk:   new(RiskDanger),
			Params: map[string]string{"confirm": "The help center's name, typed exactly."},
		},
	}
}

/*
helpCenterDomainDocs returns the entries for help centers' custom
domains and analytics.
*/
func helpCenterDomainDocs() map[string]Doc {
	return map[string]Doc{
		"help-centers.domain.show": {
			Title: "Get a help center's custom domain",
			Use:   "Reads the domain a help center answers on, the DNS records to create and where its check stands.",
			Returns: "the domain with status pending, verified or failed, the reason it failed, and its records; " +
				"null data when it has none.",
		},
		"help-centers.domain.update": {
			Title: "Add a custom domain to a help center",
			Use: "Claims a subdomain the team owns, such as help.example.com, for a help center. It answers there " +
				"once the CNAME and TXT records it returns exist and the background check verifies them.",
			Returns: "the domain with the CNAME and TXT records to create.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"hostname": "A subdomain the team owns, such as help.example.com."},
			Next:    []string{"dimmi_help_centers_domain_checks_create: check again after creating the records"},
		},
		"help-centers.domain.checks.store": {
			Title:   "Check a help center's custom domain now",
			Use:     "Starts the domain's checks over, due at once, after the team created or fixed its records.",
			Returns: "the domain; its status changes once the check ran, within a minute.",
		},
		"help-centers.domain.destroy": {
			Title: "Remove a help center's custom domain",
			Use:   "Stops a help center answering on its custom domain; it keeps its own address.",
			Risk:  new(RiskReach),
		},
		"help-centers.analytics.show": {
			Title: "Read help center analytics",
			Use: "Reads what readers did over the last days: article views per day, the most read articles with " +
				"how helpful readers found them, and searches that found nothing, which are articles to write.",
			Returns: "daily views, top articles, helpful percent and content gaps.",
			Params:  map[string]string{"days": "How many days back, 1 to 90; 30 when left out."},
		},
	}
}
