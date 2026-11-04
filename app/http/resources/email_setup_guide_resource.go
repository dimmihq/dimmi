/*
|--------------------------------------------------------------------------
| Forwarding Setup Guides
|--------------------------------------------------------------------------
|
| Forwarding is set up in the mail service the public address lives in,
| not here, so each address carries short steps for the services people
| use, its own addresses filled in. Gmail's guide says where its
| confirmation code shows up, since that mail arrives here.
|
*/

package resources

/*
SetupGuide is how to forward mail to the address in one mail service.
*/
type SetupGuide struct {
	Provider string   `json:"provider"`
	Name     string   `json:"name"`
	Steps    []string `json:"steps"`
}

/*
setupGuides returns the forwarding steps of the common mail services.
*/
func setupGuides(address, forwarding string) []SetupGuide {
	return []SetupGuide{
		{Provider: "gmail", Name: "Gmail", Steps: []string{
			"Open Gmail settings for " + address + " and go to Forwarding and POP/IMAP.",
			"Choose Add a forwarding address and enter " + forwarding + ".",
			"Gmail sends a confirmation code here. It shows on this page within a minute; enter it in Gmail.",
			"Select Forward a copy of incoming mail to " + forwarding + " and save.",
		}},
		{Provider: "google_workspace", Name: "Google Workspace", Steps: []string{
			"In the Google Admin console, open Apps, Google Workspace, Gmail, Routing.",
			"Add a routing rule for " + address + " that also delivers to " + forwarding + ".",
			"Save. No confirmation code is needed for admin routing.",
		}},
		{Provider: "microsoft_365", Name: "Microsoft 365", Steps: []string{
			"In the Exchange admin center, open the mailbox " + address + " and choose Manage mail flow settings.",
			"Turn on email forwarding to " + forwarding + " and keep a copy in the mailbox.",
			"If mail does not arrive, allow automatic forwarding in the outbound spam filter policy.",
		}},
		{Provider: "fastmail", Name: "Fastmail", Steps: []string{
			"Open Settings, Filters & Rules, and add a rule for mail to " + address + ".",
			"Choose Send a copy to " + forwarding + " and save.",
		}},
		{Provider: "other", Name: "Other", Steps: []string{
			"In your mail service, forward every message sent to " + address + " to " + forwarding + ".",
			"Keep a copy in the original mailbox if you like; replies leave from dimmi either way.",
			"Send a test email to check the forward works.",
		}},
	}
}
