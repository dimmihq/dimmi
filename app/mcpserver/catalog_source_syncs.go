/*
|--------------------------------------------------------------------------
| Tool Catalog: Knowledge Source Syncs And Documents
|--------------------------------------------------------------------------
|
| The entries for trying a source's settings before it is added, for
| its syncs, and for the documents it read: listed, read again, set
| aside or let back in, and turned into help center articles.
|
*/

package mcpserver

/*
sourceSetupDocs returns the entries that try a source's settings
before it is added. They save nothing.
*/
func sourceSetupDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.sources.previews.store": {
			Title: "Preview a knowledge source",
			Use: "Tries settings against the real service before the source is added: the account they reach, " +
				"how many documents the first page lists and how many the patterns keep. Nothing is saved.",
			Returns: "the account, the counts and a sample of documents by name.",
			Next:    []string{"dimmi_knowledge_sources_create: add it once it looks right"},
			Risk:    new(RiskRead),
			Params:  sourceParams(),
		},
		"knowledge.sources.trees.store": {
			Title: "List a new source's items",
			Use: "Lists the items, such as spaces, folders or pages, that a source being added could read, " +
				"from the connected account alone. Nothing is saved.",
			Returns: "the tree of items to choose from.",
			Risk:    new(RiskRead),
			Params:  sourceParams(),
		},
		"knowledge.sources.options.store": {
			Title: "List a source setting's values",
			Use: "Lists the values one setting of a source being added can take, read from the service with " +
				"the settings chosen so far, such as a repository's branches. Nothing is saved.",
			Returns: "the values to choose from.",
			Risk:    new(RiskRead),
			Params: map[string]string{
				"type":     "Source type, from dimmi_knowledge_source_types_list.",
				"field":    "Name of the setting whose values to list.",
				"settings": "The settings chosen so far, secrets included.",
			},
			Hint: "knowledge_source_options_unsupported means this setting is typed in, not picked.",
		},
	}
}

/*
syncDocs returns the entries for a source's syncs.
*/
func syncDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.sources.syncs.index": {
			Title:   "List a knowledge source's syncs",
			Use:     "Lists a source's latest syncs and what each read, changed or failed on.",
			Returns: "the syncs with their trigger, status, stats and timing.",
		},
		"knowledge.sources.syncs.store": {
			Title:   "Sync a knowledge source",
			Use:     "Starts reading a source again now, so the agents answer from what it holds today.",
			Returns: "the sync that started.",
			Risk:    new(RiskWrite),
		},
		"knowledge.sources.syncs.destroy": {
			Title:   "Cancel a knowledge source sync",
			Use:     "Stops a running sync. What it already read stays.",
			Returns: "the sync, now stopped.",
			Risk:    new(RiskWrite),
			Params: map[string]string{
				"run": "Id of the sync, starting with ksr_. Find it with dimmi_knowledge_sources_syncs_list.",
			},
		},
	}
}

/*
sourceDocumentDocs returns the entries for the documents a source read.
*/
func sourceDocumentDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.sources.documents.index": {
			Title:   "List a knowledge source's documents",
			Use:     "Lists the documents a source read, by status or words in the title or path.",
			Returns: "a page of documents with their title, path, status and error.",
			Params: map[string]string{
				"status": "active, error, excluded, pending or removed.",
				"after":  "Leave out for the first page; pass the cursor the previous page returned for the next.",
			},
		},
		"knowledge.sources.documents.syncs.store": {
			Title:   "Read a document again",
			Use:     "Reads one document of a source again now, without walking the whole source.",
			Returns: "the sync that reads it.",
			Risk:    new(RiskWrite),
		},
		"knowledge.sources.documents.exclusion.store": {
			Title: "Set a document aside",
			Use: "Leaves one document out of the knowledge for good: the agents stop answering from it, and " +
				"it stays out through every later sync until let back in.",
			Returns: "nothing once it is set aside.",
		},
		"knowledge.sources.documents.exclusion.destroy": {
			Title:   "Let a document back in",
			Use:     "Lets a set-aside document back into the knowledge and reads it again.",
			Returns: "the sync that reads it again.",
		},
		"knowledge.sources.imports.store": {
			Title: "Turn a source into articles",
			Use: "Turns a synced source's documents into articles of a help center, published unless the " +
				"document was a draft. A document imported before updates its article instead.",
			Returns: "nothing once the import is queued.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"help_center_id": "Help center (hc_...) the articles go to. Find it with dimmi_help_centers_list.",
			},
			Hint: "knowledge_source_nothing_to_import means it must sync first.",
		},
	}
}
