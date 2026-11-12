/*
|--------------------------------------------------------------------------
| Catalog: Bulk Changes And Merges
|--------------------------------------------------------------------------
|
| The tools that change many conversations at once and that merge
| conversations or contacts. Merges cannot be taken back, so they are
| marked as dangerous.
|
*/

package mcpserver

/*
mergeDocs documents bulk changes and merges.
*/
func mergeDocs() map[string]Doc {
	return map[string]Doc{
		"conversation-bulk-changes.store": {
			Title: "Change many conversations at once",
			Use: "Applies one change to up to 200 conversations: the same fields as updating one conversation " +
				"under changes, or trash: true to move them all to the trash. Each conversation changes on its " +
				"own, so one that fails does not stop the rest.",
			Returns: "for each conversation, whether it changed, the fields that changed, or the error code.",
			Risk:    new(RiskWrite),
			Hint:    "Send exactly one of changes or trash.",
		},
		"conversations.merges.store": {
			Title: "Merge conversations into one",
			Use: "Moves every message of the source conversations into this one, in the order they happened. " +
				"The sources are closed and moved to the trash. Only conversations of the same contact, or of " +
				"contacts at the same company, can be merged.",
			Returns: "the merged conversation as it is now.",
			Params:  map[string]string{"source_ids": "Up to 20 conversations (conv_...) to fold into this one."},
			Risk:    new(RiskDanger),
		},
		"contacts.merges.store": {
			Title: "Merge a duplicate contact into this one",
			Use: "Folds source_id into this contact: conversations, identities, tags and event counts move over, " +
				"this contact's details win and the source only fills blanks. The source keeps pointing here. " +
				"Two users with different external ids cannot be merged.",
			Returns: "this contact as it is now.",
			Params:  map[string]string{"source_id": "The duplicate contact (ctc_...) to merge into this one."},
			Next:    []string{"dimmi_contacts_duplicates_list: find likely duplicates first"},
			Risk:    new(RiskDanger),
		},
	}
}
