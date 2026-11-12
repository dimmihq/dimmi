/*
|--------------------------------------------------------------------------
| Tool Catalog: Exports, Erasures And Audiences
|--------------------------------------------------------------------------
|
| The entries for data exports, a customer's own export and erasure, and
| the saved audiences that select contacts by rule.
|
*/

package mcpserver

import "maps"

/*
exportErasureAudienceDocs returns the entries for data exports, a
customer's export and erasure, and saved audiences.
*/
func exportErasureAudienceDocs() map[string]Doc {
	out := dataExportDocs()
	for _, part := range []map[string]Doc{
		contactExportDocs(), erasureDocs(), audienceDocs(), audienceCountDocs(), audienceTagDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
dataExportDocs returns the entries for members' data exports.
*/
func dataExportDocs() map[string]Doc {
	return map[string]Doc{
		"data-exports.store": {
			Title: "Export conversations or contacts",
			Use: "Starts building a file of conversations or contacts, in the background. Each export belongs to " +
				"you alone. For a privacy request about one customer, use dimmi_contacts_exports_create instead.",
			Returns: "the export with status pending; poll it until ready.",
			Next:    []string{"dimmi_data_exports_get: wait for status ready"},
			Params: map[string]string{
				"kind": "conversations or contacts.",
				"format": "Conversations: txt (readable transcripts, 1,000 at most), csv (50,000) or json " +
					"(10,000). Contacts: csv (50,000) or json (10,000).",
				"include_notes": "true to include the team's internal notes, only if you may moderate " +
					"conversations.",
				"selection": "What to export: ids (picked ids), or filter (the list's filter[...] keys and " +
					"values) with q, or view_id (a saved inbox view), or for contacts conditions, as " +
					"dimmi_contacts_list reads them. Empty means everything.",
			},
			Hint: "export_too_large: narrow the selection or pick a format that covers more rows.",
		},
		"data-exports.index": {
			Title:   "List my data exports",
			Use:     "Lists the exports you started, with where each stands. Other members' exports never show.",
			Returns: "your exports, newest first, with status pending, running, ready, failed or expired.",
		},
		"data-exports.show": {
			Title:   "Get a data export",
			Use:     "Reads one of your exports: its status, row count, warnings and, once ready, when it expires.",
			Returns: "the export.",
			Next:    []string{"dimmi_data_exports_download_get: fetch the file once status is ready"},
		},
		"data-exports.download.show": {
			Title: "Download a data export",
			Use: "Fetches a ready export's file: one file as it is, or several as a zip. It can hold many " +
				"customers' personal data, so fetch it only for the person and never pass it on.",
			Returns: "the file's content.",
			Risk:    new(RiskSensitive),
			Hint:    "export_not_ready: wait until dimmi_data_exports_get says ready; expired files are gone.",
		},
		"data-exports.destroy": {
			Title: "Delete a data export",
			Use:   "Deletes one of your exports and its file at once, instead of waiting for it to expire.",
			Risk:  new(RiskWrite),
		},
	}
}

/*
contactExportDocs returns the entries for one customer's export.
*/
func contactExportDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.exports.store": {
			Title: "Export a customer's data",
			Use: "Starts building a zip of everything dimmi holds about one customer, to answer their request " +
				"for a copy of their data. It is built in the background and kept for a day once ready.",
			Returns: "the export with status pending.",
			Next:    []string{"dimmi_contacts_exports_get: wait for status ready"},
			Params: map[string]string{
				"include_notes": "true to add the team's internal notes, which stay out by default.",
			},
			Hint: "export_in_progress: an export of this customer is already being built; check that one.",
		},
		"contacts.exports.show": {
			Title:   "Get a customer's export",
			Use:     "Reads where a customer's export stands: pending, running, ready, failed or expired.",
			Returns: "the export, with when it expires once ready.",
			Next:    []string{"dimmi_contacts_exports_download: fetch the zip once ready"},
			Params:  map[string]string{"export": "Id of the customer's export, from dimmi_contacts_exports_create."},
		},
		"contacts.exports.download": {
			Title: "Download a customer's export",
			Use: "Fetches the zip of a customer's data, to hand to that customer. It holds all their personal " +
				"data, so fetch it only for the person answering the request.",
			Returns: "the zip.",
			Risk:    new(RiskSensitive),
			Params:  map[string]string{"export": "Id of the customer's export, from dimmi_contacts_exports_create."},
			Hint:    "export_not_ready: wait until dimmi_contacts_exports_get says ready.",
		},
	}
}

/*
erasureDocs returns the entries for requests to erase customers.
*/
func erasureDocs() map[string]Doc {
	return map[string]Doc{
		"contact-erasures.index": {
			Title:   "List erasure requests",
			Use:     "Lists the workspace's requests to erase customers and where each stands.",
			Returns: "a page of erasure requests, newest first.",
			Params: map[string]string{
				"filter[status]": "pending (in its grace period), pending_hold (held by a retention rule), " +
					"completed or canceled.",
				"page":  "A numbered page from 1.",
				"total": "true to count every matching request.",
			},
		},
		"contact-erasures.show": {
			Title:   "Get an erasure request",
			Use:     "Reads one erasure request: its customer, status, when the wipe is due and what it removed.",
			Returns: "the erasure request.",
		},
		"contact-erasures.update": {
			Title: "Cancel an erasure",
			Use: "Withdraws an erasure still in its grace period: the customer comes back as they were. A " +
				"completed erasure cannot be undone.",
			Returns: "the request, now canceled, with identities_conflicted listing addresses someone else took " +
				"meanwhile, to merge by hand.",
			Params: map[string]string{"status": "Always canceled."},
			Hint:   "erasure_closed: the erasure already completed or was canceled.",
		},
	}
}

/*
audienceDocs returns the entries for saved audiences.
*/
func audienceDocs() map[string]Doc {
	return map[string]Doc{
		"audiences.index": {
			Title: "List audiences",
			Use: "Lists the saved audiences: named rules selecting contacts, used by workflows, the messenger, " +
				"help articles and the AI agent wherever dimmi asks who.",
			Returns: "the audiences with their rules and last estimated size.",
			Params: map[string]string{
				"page":  "A numbered page from 1; without it every audience comes back.",
				"total": "With page, true to count every audience.",
			},
		},
		"audiences.show": {
			Title:   "Get an audience",
			Use:     "Reads one saved audience with its rule and what uses it: workflows and other audiences.",
			Returns: "the audience with used_by and lock_version.",
			Next:    []string{"dimmi_audiences_previews_create: count who it matches now"},
		},
		"audiences.store": {
			Title: "Create an audience",
			Use: "Saves a named rule selecting contacts by their details, custom attributes, events, tags and " +
				"company. Membership is never stored: it always means who matches now. Check the rule with " +
				"dimmi_audience_previews_create first.",
			Returns: "the audience.",
			Params: map[string]string{
				"rules": audienceRules() + " Empty means everyone.",
				"name":  "Unique in the workspace, 80 characters at most.",
			},
			Hint: "audience_invalid: the problems say which leaf to fix; check fields with dimmi_audience_fields_list.",
		},
		"audiences.update": {
			Title: "Update an audience",
			Use: "Changes a saved audience's name, description or rule. Everything using it follows the change " +
				"at once, so check what uses it with dimmi_audiences_get first.",
			Returns: "the audience with its new lock_version.",
			Params: map[string]string{
				"rules":        audienceRules(),
				"lock_version": "The lock_version you read, so a teammate's newer save is never overwritten.",
			},
			Hint: "edit_conflict: someone saved it since you read it; read it again and reapply.",
		},
		"audiences.destroy": {
			Title: "Delete an audience",
			Use:   "Deletes a saved audience. Refused while a live workflow or another audience uses it.",
			Hint:  "audience_in_use: used_by names what to change first.",
		},
	}
}

/*
audienceCountDocs returns the entries that count and test audience rules.
*/
func audienceCountDocs() map[string]Doc {
	return map[string]Doc{
		"audiences.previews.store": {
			Title: "Count a saved audience",
			Use: "Counts who a saved audience matches right now and keeps the count as its last estimate. " +
				"Large workspaces are sampled and the count marked approximate.",
			Returns: "count, pages (people who match only on some pages), total_contacts and approximate.",
		},
		"audience-previews.store": {
			Title: "Preview an audience rule",
			Use: "Counts who a rule matches right now without saving it, to check a rule before saving it or " +
				"before a workflow or broadcast uses it. Nothing is written.",
			Returns: "count, pages (people who match only on some pages), total_contacts, approximate, a sample " +
				"of matching people and warnings.",
			Params: map[string]string{
				"rules":  audienceRules(),
				"sample": "How many matching people to list, 0 to 25.",
			},
		},
		"audience-evaluations.store": {
			Title: "Test a rule on one person",
			Use: "Answers whether one contact matches a rule, with every condition's actual value and " +
				"whether it held. Nothing is written. Use it to explain why someone did or did not see something.",
			Returns: "result true, false or unknown (depends on the page), and each condition's outcome.",
			Params: map[string]string{
				"rules":      audienceRules(),
				"contact_id": "Id of the contact (ctc_...) to test.",
				"context": "What the browser would report: page_url, locale, device (desktop, mobile or tablet) " +
					"and country (two letters). Page conditions without it stay unknown.",
			},
		},
		"audience-fields.index": {
			Title: "List audience fields",
			Use: "Lists every field a rule may read: contact and company details, custom attributes with their " +
				"types and options, and each active event's count, first and last time, with the operators each " +
				"field allows. Read it before writing a rule.",
			Returns: "the fields with key, type, options and ops.",
		},
	}
}

/*
audienceTagDocs returns the entries that tag contacts in bulk.
*/
func audienceTagDocs() map[string]Doc {
	return map[string]Doc{
		"audiences.freezes.store": {
			Title: "Tag everyone in an audience",
			Use: "Puts a tag on everyone a saved audience matches right now, so the group stays fixed while the " +
				"audience keeps changing. Later joiners are not tagged.",
			Returns: "the tag and how many people matched and were tagged.",
			Params:  map[string]string{"tag": "Name of the tag to put on them, 50 characters at most."},
			Hint:    "audience_too_large: workspaces over 100,000 people cannot be frozen at once.",
		},
	}
}

/*
audienceRules explains the rules argument every audience tool takes.
*/
func audienceRules() string {
	return "The audience as JSON: {\"visitors\": everyone, identified, leads, anonymous or " +
		"anonymous_and_leads, \"rules\": a tree of all, any and not groups over leaves {field, op, value}}. " +
		"Fields come from dimmi_audience_fields_list, such as contact.custom.plan or event.signed_up.count. " +
		"Four levels, 50 leaves, 8 KB at most."
}
