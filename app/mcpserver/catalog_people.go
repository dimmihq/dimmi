/*
|--------------------------------------------------------------------------
| Tool Catalog: People
|--------------------------------------------------------------------------
|
| The entries for contacts and companies: the customers and leads the
| team talks to, how they appear on channels, and the companies they
| belong to.
|
*/

package mcpserver

import "maps"

/*
peopleDocs returns the entries for contacts, companies, attributes,
events, audiences, exports and erasures.
*/
func peopleDocs() map[string]Doc {
	out := contactReadDocs()
	for _, part := range []map[string]Doc{
		contactWriteDocs(), contactRemovalDocs(), contactInsightDocs(), companyDocs(), companyChangeDocs(),
		exportErasureAudienceDocs(), attributeEventDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
contactReadDocs returns the entries that find and read contacts.
*/
func contactReadDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.index": {
			Title: "List contacts",
			Use: "Lists the workspace's customers (users, who have an external id) and leads, most recently seen " +
				"first. Blocked contacts stay out unless filter[blocked]=true. To find people by a saved rule, " +
				"pass conditions with an audience, or use dimmi_audience_previews_create to only count them.",
			Returns: "a page of contacts with their attributes, tags and company, and next_cursor for more.",
			Next:    []string{"dimmi_contacts_get: read one contact in full"},
			Params: map[string]string{
				"filter[kind]": "user or lead.",
				"filter[custom.<key>]": "A custom attribute's value, such as filter[custom.plan]=pro. " +
					"true, false and numbers are read as such. Three at most.",
				"conditions": "A JSON list of {field, op, value or values}. Fields: name, email, phone, user_id, " +
					"kind, tags, company, last_seen, created, custom.<key>, and audience with op in_any or " +
					"in_none and values of audience ids (audn_...). Ten at most.",
				"sort":  "last_seen_at (default), created_at or name, with a leading minus for descending.",
				"page":  "A numbered page from 1, instead of a cursor.",
				"total": "true to count every matching contact, up to a cap.",
			},
		},
		"contacts.show": {
			Title:   "Get a contact",
			Use:     "Reads one customer or lead: details, custom attributes, tags, company and notes.",
			Returns: "the contact.",
			Next: []string{
				"dimmi_contacts_conversations_list: see what they wrote about",
				"dimmi_contacts_event_summaries_list: see what they did in your product",
			},
		},
		"contacts.conversations.index": {
			Title:   "List a contact's conversations",
			Use:     "Lists the conversations of one customer that you may see, newest first.",
			Returns: "a page of conversations, with next_cursor for more.",
			Next:    []string{"dimmi_conversations_messages_list: read one of them"},
		},
	}
}

/*
contactWriteDocs returns the entries that add and change contacts.
*/
func contactWriteDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.store": {
			Title: "Add a contact",
			Use: "Adds a customer or lead. With upsert=true it finds the contact by external id, then by a proven " +
				"email, and updates it instead, so it is safe to repeat. Typed-in lead emails are never merged; " +
				"they come back as possible duplicates.",
			Returns: "the contact, and with upsert meta.upserted (created or updated) and possible_duplicates.",
			Params: map[string]string{
				"upsert":      "true to find or add by external id, then email. Only this mode accepts company.",
				"external_id": "Your product's id for this person. A contact with one is a user, without a lead.",
				"custom_attributes": "An object of custom attribute values by key. Keys and types follow " +
					"dimmi_attribute_definitions_list; a value of the wrong type is dropped and listed by " +
					"dimmi_attribute_problems_list.",
				"company": "The person's company by external_id, name or domain; found or added, then linked.",
			},
			Hint: "contact_exists or contact_email_exists means someone already has it: retry with upsert=true.",
		},
		"contacts.update": {
			Title: "Update a contact",
			Use: "Changes a contact's details, custom attributes, notes, tags, company or blocked state. Only sent " +
				"fields change. Setting an external id turns a lead into a user; an existing one never changes.",
			Returns: "the contact as it now is.",
			Params: map[string]string{
				"custom_attributes": "Values by key, merged into what the contact has. null removes a key.",
				"blocked":           "true blocks the contact, which also hides them from the list; false unblocks.",
				"notes":             "Team-only notes, 10,000 characters at most, replacing the current notes.",
				"company_id":        "Id of the company (cmp_...) to link, or null to unlink.",
				"add_tags":          "Tag ids or names to add; a new name becomes a tag. 50 at most.",
				"remove_tags":       "Ids of tags (tag_...) to remove.",
			},
			Hint: "external_id_immutable: the contact already has another external id and keeps it.",
		},
	}
}

/*
contactRemovalDocs returns the entries that delete or erase a contact
and detach its identities.
*/
func contactRemovalDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.destroy": {
			Title: "Delete or erase a contact",
			Use: "Without erase, removes a contact added by mistake and only if it has no conversations. With " +
				"erase=true, starts a privacy erasure: the person vanishes from lists now, and after a grace " +
				"period every message, file and record of them is wiped for good. Use only when the person asked.",
			Returns: "nothing for a plain delete, or the erasure request with its scheduled_for time.",
			Next:    []string{"dimmi_contact_erasures_update: withdraw the erasure during its grace period"},
			Params: map[string]string{
				"erase": "true to erase the person and everything they wrote, after a grace period (7 days by " +
					"default).",
				"immediate": "With erase, true skips the grace period so nothing can be withdrawn. Needs a " +
					"recently confirmed password.",
			},
			Hint: "contact_has_conversations: a contact with history can only be erased, with erase=true.",
		},
		"contacts.identities.index": {
			Title: "List a contact's identities",
			Use: "Lists how a customer appears on channels: email addresses, phone numbers, chat app users. " +
				"Messages from any of them land in this contact's history.",
			Returns: "the identities, with their ids.",
			Next:    []string{"dimmi_contacts_identities_delete: detach one attached by mistake"},
		},
		"contacts.identities.destroy": {
			Title: "Detach a contact identity",
			Use: "Detaches an address or account attached to the wrong customer. The next message from it " +
				"starts a new lead instead of joining this contact's history; past messages stay where they are.",
			Params: map[string]string{
				"identity": "Id of the identity, from dimmi_contacts_identities_list.",
			},
		},
	}
}

/*
contactInsightDocs returns the entries on a contact's duplicates, events
and messenger sessions.
*/
func contactInsightDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.duplicates.index": {
			Title: "Find duplicate contacts",
			Use: "Lists contacts that look like the same person as this one, such as a lead who typed the " +
				"address of an existing user, with the reasons. Nothing is merged; a teammate decides.",
			Returns: "up to 20 likely duplicates with their reasons, strongest first.",
		},
		"contacts.event-summaries.index": {
			Title: "Summarize a contact's events",
			Use: "Says what one person did in your product: for each event name, how many times overall and " +
				"in the last 7 and 30 days, and when first and last. Audience rules read the same counts.",
			Returns: "one summary per event name, with the last event's metadata.",
		},
		"contacts.widget-sessions.destroy": {
			Title: "Sign a contact out of the messenger",
			Use: "Ends every messenger session the contact has on your sites, such as after an account " +
				"compromise. They must sign in again with a fresh identity token to keep chatting.",
			Returns: "sessions_revoked, how many sessions ended.",
		},
	}
}

/*
companyDocs returns the entries that find, read and add companies.
*/
func companyDocs() map[string]Doc {
	return map[string]Doc{
		"companies.index": {
			Title:   "List companies",
			Use:     "Lists the companies your customers belong to. q searches by name or domain.",
			Returns: "a page of companies, with next_cursor for more.",
			Next:    []string{"dimmi_companies_get: read one company in full"},
			Params: map[string]string{
				"filter[tag]": "Id of a tag (tag_...) the companies carry.",
				"page":        "A numbered page from 1, instead of a cursor.",
				"total":       "true to count every matching company, up to a cap.",
			},
		},
		"companies.show": {
			Title:   "Get a company",
			Use:     "Reads one company: name, domain, external id, custom attributes, tags and notes.",
			Returns: "the company.",
			Next: []string{
				"dimmi_companies_contacts_list: see its people",
				"dimmi_companies_conversations_list: see what its people wrote about",
			},
		},
		"companies.store": {
			Title: "Add a company",
			Use: "Adds a company. With upsert=true it finds the company by external id, then domain, and " +
				"updates it instead, so it is safe to repeat. Link people to it with dimmi_contacts_update.",
			Returns: "the company, and with upsert meta.upserted (created or updated).",
			Params: map[string]string{
				"upsert": "true to find or add by external id, then domain. A plain create needs a name.",
				"custom_attributes": "An object of custom attribute values by key, as " +
					"dimmi_attribute_definitions_list defines them for companies.",
			},
			Hint: "company_exists: another company has this external id; retry with upsert=true.",
		},
	}
}

/*
companyChangeDocs returns the entries that change and delete companies
and list their people and conversations.
*/
func companyChangeDocs() map[string]Doc {
	return map[string]Doc{
		"companies.update": {
			Title:   "Update a company",
			Use:     "Changes a company's details, custom attributes, notes or tags. Only sent fields change.",
			Returns: "the company as it now is.",
			Params: map[string]string{
				"custom_attributes": "Values by key, merged into what the company has. null removes a key.",
				"notes":             "Team-only notes, 10,000 characters at most, replacing the current notes.",
				"add_tags":          "Tag ids or names to add; a new name becomes a tag. 50 at most.",
				"remove_tags":       "Ids of tags (tag_...) to remove.",
			},
		},
		"companies.destroy": {
			Title: "Delete a company",
			Use: "Deletes a company. Its people stay as contacts without a company, and their conversations " +
				"are untouched.",
		},
		"companies.contacts.index": {
			Title:   "List a company's people",
			Use:     "Lists the contacts linked to one company, newest first.",
			Returns: "a page of contacts, with next_cursor for more.",
		},
		"companies.conversations.index": {
			Title:   "List a company's conversations",
			Use:     "Lists the conversations of all the company's people that you may see, newest first.",
			Returns: "a page of conversations, with next_cursor for more.",
			Next:    []string{"dimmi_conversations_messages_list: read one of them"},
		},
	}
}
