/*
|--------------------------------------------------------------------------
| Tool Catalog: Attributes And Events
|--------------------------------------------------------------------------
|
| The entries for the custom attributes contacts and companies carry and
| the events your product reports about contacts: the facts audience
| rules select people by.
|
*/

package mcpserver

import "maps"

/*
peopleDocs3 returns the entries for custom attributes and events.
*/
func peopleDocs3() map[string]Doc {
	out := attributeDocs()
	for _, part := range []map[string]Doc{
		attributeCleanupDocs(), eventDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
attributeDocs returns the entries that list, define and change custom
attributes.
*/
func attributeDocs() map[string]Doc {
	return map[string]Doc{
		"attribute-definitions.index": {
			Title: "List custom attributes",
			Use: "Lists the custom attributes contacts and companies carry, each with its key, type and list " +
				"values. Keys already in use without a definition are added as observed. Audience rules read " +
				"them as contact.custom.<key> and company.custom.<key>.",
			Returns: "the definitions, with status active or archived and whether browsers may write them.",
			Params:  map[string]string{"filter[object]": "contact or company; leave out for both."},
		},
		"attribute-definitions.store": {
			Title: "Define a custom attribute",
			Use: "Defines a custom attribute so its values are typed and checked, and audience rules and filters " +
				"can offer it. Its key and type never change afterwards. Values are set with " +
				"dimmi_contacts_update or dimmi_companies_update.",
			Returns: "the definition.",
			Params: map[string]string{
				"object": "contact or company.",
				"key": "Lowercase letters, digits and underscores, starting with a letter, 64 at most. " +
					"It cannot shadow a standard field.",
				"data_type": "string, number, boolean, date or list.",
				"options":   "For a list only: 1 to 200 different values.",
			},
			Hint: "attribute_exists: the key is already defined; update that definition instead.",
		},
		"attribute-definitions.update": {
			Title: "Update a custom attribute",
			Use: "Changes a custom attribute's label, description or list values, archives it to stop new " +
				"values while keeping old ones, or lets browsers write it. Archiving is refused while an " +
				"audience or workflow reads it.",
			Returns: "the definition as it now is.",
			Params: map[string]string{
				"status": "active or archived.",
				"client_writable": "Contacts only: true lets the messenger set it from the browser, so its " +
					"values are claimed, not trusted. 20 attributes at most.",
			},
			Hint: "attribute_in_use: change the audiences or workflows that read it first.",
		},
	}
}

/*
attributeCleanupDocs returns the entries that delete custom attributes
and list the values they dropped.
*/
func attributeCleanupDocs() map[string]Doc {
	return map[string]Doc{
		"attribute-definitions.destroy": {
			Title: "Delete a custom attribute",
			Use: "Deletes an archived custom attribute and removes its value from every contact or company " +
				"for good. Archive it with dimmi_attribute_definitions_update first.",
			Hint: "attribute_active: archive the attribute before deleting it.",
		},
		"attribute-problems.index": {
			Title: "List dropped attribute values",
			Use: "Lists custom attribute values dimmi refused in the last 30 days: a wrong type, a value not " +
				"in the list, a write to an archived attribute or past the key limit. Use it when values " +
				"seem missing.",
			Returns: "the problems by attribute and code, with counts, a sample value and when last seen.",
		},
	}
}

/*
eventDocs returns the entries for recording events and naming them.
*/
func eventDocs() map[string]Doc {
	return map[string]Doc{
		"contact-events.store": {
			Title: "Record a contact event",
			Use: "Records something a person did in your product, such as signed_up or upgraded, one event or " +
				"up to 100 under events. Audience rules see it at once as event.<name>.count, first_at and " +
				"last_at. A new name is defined on its first event.",
			Returns: "accepted, and per event its status: accepted, duplicate, contact_not_found, archived or " +
				"event_limit_reached.",
			Params: map[string]string{
				"name":            "The event's name, 64 characters at most, such as signed_up.",
				"contact_id":      "The person by contact id (ctc_...); or give external_id or email instead.",
				"occurred_at":     "RFC 3339 time, up to 30 days ago; earlier or future times are clamped.",
				"metadata":        "An object of details kept with the person's last event of this name.",
				"idempotency_key": "A key to send the same event again safely; repeats within 48 hours count once.",
				"events":          "Up to 100 events with the same fields, instead of one at the top level.",
			},
		},
		"event-definitions.index": {
			Title:   "List event names",
			Use:     "Lists the names of the events your product reports, with how many arrived and when last.",
			Returns: "the event definitions, with status active or archived.",
		},
		"event-definitions.update": {
			Title: "Update an event name",
			Use: "Changes an event name's label or description, or archives it to stop recording new events " +
				"of it. Counts already kept still answer audience rules.",
			Returns: "the event definition.",
			Params:  map[string]string{"status": "active or archived."},
		},
	}
}
