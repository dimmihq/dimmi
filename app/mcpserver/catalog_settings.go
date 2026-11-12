/*
|--------------------------------------------------------------------------
| Tool Catalog: Settings
|--------------------------------------------------------------------------
|
| The entries for sending email, identity verification, redaction and
| retention, the messenger's settings and office hours.
|
*/

package mcpserver

import "maps"

/*
settingsDocs returns the entries for sending email, identity
verification, redaction and retention, the messenger's settings and
office hours.
*/
func settingsDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		emailAddressDocs(), emailAddressChangeDocs(), emailAddressMailDocs(), signatureDocs(),
		signatureChangeDocs(), emailDomainDocs(), emailDomainChangeDocs(), messengerPrivacyDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
emailAddressDocs returns the entries for listing, reading and
connecting the workspace's email addresses.
*/
func emailAddressDocs() map[string]Doc {
	return map[string]Doc{
		"email-addresses.index": {
			Title: "List email addresses",
			Use: "Lists the public addresses customers write to, such as support@company.com, the default first, " +
				"each with the forwarding address it must forward to, the inbox it feeds and what it sends as now.",
			Returns: "every address of the workspace with its forwarding address, sending state and setup guides.",
			Next:    []string{"dimmi_email_addresses_get: read one address in full"},
		},
		"email-addresses.show": {
			Title: "Get an email address",
			Use: "Reads one connected address: its forwarding address, reply settings, the inbox it feeds, whether " +
				"forwarding was confirmed and what it sends as (its own verified domain or the default sender).",
			Returns: "the address with its settings, sending state and setup guides.",
			Next: []string{
				"dimmi_email_addresses_tests_create: prove forwarding works",
				"dimmi_email_addresses_update: change how it replies",
			},
		},
		"email-addresses.store": {
			Title: "Connect an email address",
			Use: "Connects a public address the team receives mail at and gives it a forwarding address. Mail only " +
				"arrives once the person sets their mail provider to forward to that address. The workspace's first " +
				"address becomes its default.",
			Returns: "the new address with the forwarding address to set up and the setup guide that fits its domain.",
			Next: []string{
				"dimmi_email_addresses_tests_create: check forwarding once the person has set it up",
				"dimmi_email_domains_create: send from the address's own domain",
			},
			Params: map[string]string{
				"address":      "The public address, such as support@company.com. It cannot be on dimmi's own domain.",
				"display_name": "The name replies are sent under, at most 80 characters.",
				"inbox_id": "Id of the inbox (inbox_...) new conversations land in. Find it with " +
					"dimmi_inboxes_list.",
			},
			Hint: "An address already connected cannot be connected again; list the addresses to find it.",
		},
	}
}

/*
emailAddressChangeDocs returns the entries for changing and
disconnecting an email address.
*/
func emailAddressChangeDocs() map[string]Doc {
	return map[string]Doc{
		"email-addresses.update": {
			Title: "Update an email address",
			Use: "Changes how an address signs and styles replies customers receive, what happens to bulk mail, " +
				"which inbox it feeds, or makes it the default. Moving it to another inbox leaves earlier " +
				"conversations where they are. Only the fields sent change.",
			Returns: "the address as it now is.",
			Params: map[string]string{
				"reply_to_mode": "token (default) adds a Reply-To address that threads answers back into their " +
					"conversation; none leaves it out.",
				"from_name_style": "member_and_name names the teammate and the team; name_only names just the team.",
				"is_default": "true makes this the workspace's default address. The default cannot be turned off, " +
					"only moved to another address.",
				"inbox_id": "Id of the inbox (inbox_...) new conversations land in.",
				"settings": "quote_previous includes the earlier message in replies; bulk_mail_handling is close " +
					"(newsletters open closed conversations), open (open ones) or drop (no conversation); template " +
					"is plain or branded, brand_color a hex color like #1a73e8, footer_markdown at most 500 " +
					"characters.",
			},
		},
		"email-addresses.destroy": {
			Title:   "Disconnect an email address",
			Use:     "Disconnects an address: mail sent to its forwarding address no longer becomes conversations.",
			Returns: "nothing once it is disconnected.",
		},
	}
}

/*
emailAddressMailDocs returns the entries for the mail an address
received, its reply style and its forwarding tests.
*/
func emailAddressMailDocs() map[string]Doc {
	return map[string]Doc{
		"email-addresses.inbound-emails.index": {
			Title: "List emails received at an address",
			Use: "Lists what arrived at an address, newest first, with sender, subject, how each was handled and " +
				"the conversation it joined. Use it to find out why an email did not show up in the inbox.",
			Returns: "a page of received emails with status, sender, subject and any error, with next_cursor for more.",
			Risk:    new(RiskSensitive),
			Params: map[string]string{
				"filter[status]": "received, processing, processed, ignored, failed or quarantined.",
			},
		},
		"email-addresses.template-renderings.store": {
			Title: "Preview an address's reply style",
			Use: "Renders a sample reply from an address in a style before it is saved. Fields left out keep " +
				"what the address has saved. Nothing changes and nothing is sent.",
			Returns: "the sample reply as HTML and plain text.",
			Risk:    new(RiskRead),
			Next:    []string{"dimmi_email_addresses_update: save the style under settings"},
			Params: map[string]string{
				"template":        "plain or branded.",
				"brand_color":     "A six digit hex color, such as #1a73e8.",
				"footer_markdown": "A footer in Markdown, at most 500 characters.",
			},
		},
		"email-addresses.tests.store": {
			Title: "Send a forwarding test",
			Use: "Sends a test email to the address's own public address and waits for it to come back through " +
				"forwarding, proving the setup works. One test per address every thirty seconds, twenty a day.",
			Returns: "the test, waiting for its mail, with the address it was sent to.",
			Risk:    new(RiskWrite),
			Next:    []string{"dimmi_email_addresses_tests_get: read the result after a few seconds"},
			Hint: "When no mailer is available the test is still stored; ask the person to send the mail " +
				"themselves with the subject the error gives.",
		},
		"email-addresses.tests.show": {
			Title: "Get a forwarding test",
			Use: "Reads a forwarding test: still waiting, received, or failed, with findings that explain what " +
				"went wrong and how long the mail took.",
			Returns: "the test with its status, findings and latency.",
			Params: map[string]string{
				"test": "Id of the forwarding test, starting with eat_, from dimmi_email_addresses_tests_create.",
			},
		},
	}
}

/*
signatureDocs returns the entries for listing and
creating email signatures.
*/
func signatureDocs() map[string]Doc {
	return map[string]Doc{
		"email-signatures.index": {
			Title: "List email signatures",
			Use: "Lists the signatures that close email replies: personal ones per teammate and address ones per " +
				"address. Without manage_any you see only your own and the team's.",
			Returns: "the signatures with their kind, teammate, address, Markdown body and whether each is used.",
			Params: map[string]string{
				"filter[member]":  "A teammate's id (mem_...).",
				"filter[address]": "An email address id (ema_...).",
				"filter[kind]":    "personal or address.",
			},
		},
		"email-signatures.store": {
			Title: "Create an email signature",
			Use: "Adds a signature put under replies customers receive. A personal one is yours unless member_id " +
				"names another teammate, and covers every address unless email_address_id names one. One per " +
				"teammate and address.",
			Returns: "the signature as saved.",
			Risk:    new(RiskReach),
			Next:    []string{"dimmi_email_signatures_renderings_create: check how it reads before saving"},
			Params: map[string]string{
				"kind":             "personal (default) or address.",
				"member_id":        "Id of the teammate (mem_...) for another teammate's personal signature.",
				"email_address_id": "Id of the address (ema_...); required for an address signature.",
				"body": "Markdown with bold, italic, links and line breaks, at most 2000 characters. Placeholders: " +
					"{{member_name}}, {{member_title}}, {{address_name}}, {{company_name}}.",
				"enabled": "Whether replies use it; defaults to true.",
			},
			Hint: "Only the four listed placeholders are allowed; the error names any unknown one.",
		},
	}
}

/*
signatureChangeDocs returns the entries for changing,
removing and previewing email signatures.
*/
func signatureChangeDocs() map[string]Doc {
	return map[string]Doc{
		"email-signatures.update": {
			Title:   "Update an email signature",
			Use:     "Changes a signature's text or turns it on or off. The next replies customers receive use it.",
			Returns: "the signature as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"body": "Markdown, at most 2000 characters, with the placeholders {{member_name}}, {{member_title}}, " +
					"{{address_name}} and {{company_name}}.",
			},
		},
		"email-signatures.destroy": {
			Title:   "Delete an email signature",
			Use:     "Removes a signature. Replies sent from then on go out without it.",
			Returns: "nothing once it is removed.",
		},
		"email-signatures.renderings.store": {
			Title: "Preview an email signature",
			Use: "Shows how a signature text reads in a reply from you, placeholders filled in, before it is saved. " +
				"Nothing is stored.",
			Returns: "the signature as HTML and plain text.",
			Risk:    new(RiskRead),
			Params: map[string]string{
				"body":             "The signature in Markdown, at most 2000 characters.",
				"email_address_id": "Id of the address (ema_...) to fill {{address_name}} from; leave out for none.",
			},
		},
	}
}

/*
emailDomainDocs returns the entries for listing, reading and adding
the workspace's sending domains.
*/
func emailDomainDocs() map[string]Doc {
	return map[string]Doc{
		"email-domains.index": {
			Title:   "List sending domains",
			Use:     "Lists the domains the workspace sends email from, with whether each is verified.",
			Returns: "every sending domain with its status, DNS records and when it was last checked.",
		},
		"email-domains.show": {
			Title: "Get a sending domain",
			Use: "Reads one sending domain: the DNS records to publish and the state of each, the DNS host found " +
				"and its guide, and why verification failed if it did.",
			Returns: "the domain with its records, warnings and host guide, or a zone file with format=bind.",
			Params: map[string]string{
				"format": "json (default) or bind for the records as a zone file a DNS host can import.",
			},
		},
		"email-domains.store": {
			Title: "Add a sending domain",
			Use: "Adds a domain such as company.com so replies are sent from it rather than the default sender. " +
				"It starts pending: the person publishes the DNS records it returns, then it is checked.",
			Returns: "the pending domain with the DNS records to publish.",
			Next: []string{
				"dimmi_email_domains_dns_setups_create: let the DNS host add the records in one approval",
				"dimmi_email_domains_checks_create: check once the records are published",
			},
			Params: map[string]string{
				"domain":              "The domain, such as company.com. Mailbox domains like gmail.com are refused.",
				"mail_from_subdomain": "The return path label, such as bounce; one label, at most 63 characters.",
			},
		},
	}
}

/*
emailDomainChangeDocs returns the entries for changing,
removing and checking a sending domain.
*/
func emailDomainChangeDocs() map[string]Doc {
	return map[string]Doc{
		"email-domains.update": {
			Title: "Update a sending domain",
			Use: "Changes the return path label or the DNS host the person says they use. A new return path's " +
				"records are checked again; the other records keep their state.",
			Returns: "the domain as it now is.",
			Params: map[string]string{
				"mail_from_subdomain": "The return path label, such as bounce.",
				"dns_host":            "The DNS host the person uses; an empty string goes back to the detected one.",
			},
		},
		"email-domains.destroy": {
			Title: "Remove a sending domain",
			Use: "Removes a sending domain. A verified domain addresses still send from is refused unless force is " +
				"true; those addresses then send from the default sender.",
			Returns: "nothing once it is removed.",
			Params: map[string]string{
				"force": "true removes a domain addresses still send from.",
			},
			Hint: "A refusal lists the addresses still sending from the domain; ask before passing force=true.",
		},
		"email-domains.checks.store": {
			Title: "Check a sending domain's DNS",
			Use: "Queues a check of the domain's DNS records now instead of waiting for the next scheduled one. A " +
				"pending or failed domain starts a fresh three day verification window. Once every ten seconds.",
			Returns: "the domain, with the check queued.",
			Risk:    new(RiskWrite),
			Next:    []string{"dimmi_email_domains_get: read the records' state after the check"},
		},
		"email-domains.dns-setups.store": {
			Title: "Start automatic DNS setup",
			Use: "Gets a link the person opens to approve adding every record at their DNS host in one step, where " +
				"the host supports it. Nothing changes until they approve there; copying records by hand always works.",
			Returns: "the DNS host's name and the apply_url to give the person.",
			Risk:    new(RiskWrite),
			Hint:    "When the host does not support it, give the person the records from dimmi_email_domains_get.",
		},
		"email-domains.dkim-rotations.store": {
			Title: "Rotate a domain's signing key",
			Use: "Starts a new DKIM signing key for a domain dimmi signs for. The current key signs until the new " +
				"record is published and seen. Domains sent through SES rotate their own keys.",
			Returns: "the domain with the new record to publish.",
			Next:    []string{"dimmi_email_domains_checks_create: check once the new record is published"},
		},
	}
}
