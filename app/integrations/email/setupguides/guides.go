/*
|--------------------------------------------------------------------------
| Forwarding Setup Guides
|--------------------------------------------------------------------------
|
| Forwarding is set up in the mail service the public address lives in,
| not here, so each address carries the steps for every service people
| use, the one that fits its domain first. A wrong guess costs nothing:
| every guide is listed and the generic one works anywhere. Steps and
| warnings are language keys; every guide ends by running the test.
|
*/

package setupguides

import (
	"slices"
	"strconv"
	"strings"
)

/*
The guide keys.
*/
const (
	Gmail             = "gmail"
	GoogleWorkspace   = "google_workspace"
	GoogleGroup       = "google_group"
	Microsoft365      = "microsoft_365"
	OutlookCom        = "outlook_com"
	Fastmail          = "fastmail"
	ICloud            = "icloud"
	Yahoo             = "yahoo"
	Zoho              = "zoho"
	CPanel            = "cpanel"
	CloudflareRouting = "cloudflare_routing"
	Generic           = "generic"
)

/*
Guide is how to forward mail in one mail service. Name is the service's
own name. DeepLink opens the right settings screen, empty when there is
none worth linking. Steps and Warnings are language keys under
email_setup.guides.<key>, filled with forwarding_address, address and
domain.
*/
type Guide struct {
	Key      string
	Name     string
	DeepLink string
	Steps    []string
	Warnings []string
}

/*
Guides returns every guide, the generic one last.
*/
func Guides() []Guide {
	return []Guide{
		guide(Guide{Key: Gmail, Name: "Gmail", DeepLink: "https://mail.google.com/mail/u/0/#settings/fwdandpop"}, 4, "sent_folder"),
		guide(Guide{Key: GoogleWorkspace, Name: "Google Workspace", DeepLink: "https://admin.google.com/ac/apps/gmail/routing"}, 7,
			"delay", "not_admin"),
		guide(Guide{Key: GoogleGroup, Name: "Google Group", DeepLink: "https://admin.google.com/ac/apps/gmail/routing"}, 3, "via_sender"),
		guide(Guide{Key: Microsoft365, Name: "Microsoft 365", DeepLink: "https://admin.exchange.microsoft.com/#/mailboxes"}, 3,
			"outbound_policy", "shared_mailbox"),
		guide(Guide{Key: OutlookCom, Name: "Outlook.com", DeepLink: "https://outlook.live.com/mail/0/options/mail/forwarding"}, 3),
		guide(Guide{Key: Fastmail, Name: "Fastmail", DeepLink: "https://app.fastmail.com/settings/filters"}, 3),
		guide(Guide{Key: ICloud, Name: "iCloud Mail", DeepLink: "https://www.icloud.com/mail/"}, 3, "whole_mailbox"),
		guide(Guide{Key: Yahoo, Name: "Yahoo Mail", DeepLink: "https://mail.yahoo.com/n/settings/mailboxes"}, 3, "paid_plan"),
		guide(Guide{Key: Zoho, Name: "Zoho Mail", DeepLink: "https://mail.zoho.com/zm/#settings/all/mailforward"}, 3),
		guide(Guide{Key: CPanel, Name: "cPanel", DeepLink: ""}, 3),
		guide(Guide{Key: CloudflareRouting, Name: "Cloudflare Email Routing", DeepLink: "https://dash.cloudflare.com/?to=/:account/{domain}/email"}, 3),
		guide(Guide{Key: Generic, Name: "Other mail service", DeepLink: ""}, 2, "no_alias"),
	}
}

/*
guide lists a guide's numbered steps and its warnings as language keys,
then the step every guide shares: running the test.
*/
func guide(g Guide, steps int, warnings ...string) Guide {
	prefix := "email_setup.guides." + g.Key + "."

	g.Steps = make([]string, 0, steps+1)
	for n := range steps {
		g.Steps = append(g.Steps, prefix+"step_"+strconv.Itoa(n+1))
	}

	g.Steps = append(g.Steps, "email_setup.guides.common.run_test")

	g.Warnings = make([]string, 0, len(warnings))
	for _, warning := range warnings {
		g.Warnings = append(g.Warnings, prefix+"warning_"+warning)
	}

	return g
}

/*
Ordered returns every guide with the recommended one first and the rest
in their usual order.
*/
func Ordered(recommended string) []Guide {
	guides := Guides()

	index := slices.IndexFunc(guides, func(g Guide) bool { return g.Key == recommended })
	if index <= 0 {
		return guides
	}

	first := guides[index]

	return append([]Guide{first}, slices.Delete(guides, index, index+1)...)
}

/*
ForDomain returns the guide of a public mail service the domain belongs
to, such as gmail.com, and false for any other domain.
*/
func ForDomain(domain string) (string, bool) {
	switch strings.ToLower(domain) {
	case "gmail.com", "googlemail.com":
		return Gmail, true
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return OutlookCom, true
	case "icloud.com", "me.com", "mac.com":
		return ICloud, true
	case "yahoo.com", "ymail.com":
		return Yahoo, true
	}

	return "", false
}

/*
mxGuides maps a part of a mail exchanger's name to the guide of the
service behind it.
*/
func mxGuides() []struct{ part, guide string } {
	return []struct{ part, guide string }{
		{"google.com", GoogleWorkspace},
		{"googlemail.com", GoogleWorkspace},
		{"protection.outlook.com", Microsoft365},
		{"fastmail", Fastmail},
		{"zoho", Zoho},
		{"icloud.com", ICloud},
		{"cloudflare.net", CloudflareRouting},
	}
}

/*
ForExchangers returns the guide of the service the mail exchangers
belong to, the generic one when none is known.
*/
func ForExchangers(hosts []string) string {
	for _, known := range mxGuides() {
		if slices.ContainsFunc(hosts, func(host string) bool {
			return strings.Contains(strings.ToLower(host), known.part)
		}) {
			return known.guide
		}
	}

	return Generic
}
