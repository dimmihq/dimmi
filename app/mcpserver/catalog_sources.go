/*
|--------------------------------------------------------------------------
| Tool Catalog: Knowledge Sources
|--------------------------------------------------------------------------
|
| The entries for the knowledge the AI agent and Operator answer from
| beyond the help center: sources connected from other tools, the
| embedding model searches use and what the knowledge costs. A source's
| settings hold secrets, so changing one asks the person first.
|
*/

package mcpserver

import "maps"

/*
sourceDocs returns the entries for knowledge sources, their syncs and
documents, and the knowledge settings.
*/
func sourceDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		knowledgeSettingDocs(), sourceReadDocs(), sourceChangeDocs(), sourceSetupDocs(), syncDocs(),
		sourceDocumentDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
knowledgeSettingDocs returns the entries for the embedding model and
what the knowledge costs.
*/
func knowledgeSettingDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.settings.show": {
			Title:   "Get knowledge settings",
			Use:     "Reads the embedding model knowledge search uses, and one being built to replace it.",
			Returns: "the active model, its provider key and the progress of a model being built.",
		},
		"knowledge.settings.update": {
			Title: "Change the embedding model",
			Use: "Picks the model knowledge search uses. It is checked with one tiny embedding, then built in " +
				"the background while searches stay on the old one, and switched in once it covers " +
				"everything. Building spends on the provider key.",
			Returns: "the settings with the model being built.",
			Risk:    new(RiskDanger),
			Params: map[string]string{
				"embedding_key_id": "Provider key (aik_...) to embed with; null or left out for the built-in " +
					"embedder. Find it with dimmi_ai_keys_list.",
				"model": "Embedding model id on that key.",
				"dims":  "Vector size the model returns, 64 to 4096.",
			},
		},
		"knowledge.usage.show": {
			Title:   "Get knowledge usage",
			Use:     "Reads what the workspace's knowledge cost this month, per source and kind of work.",
			Returns: "the month's start, its total cost in micro-dollars and a line per source and kind.",
		},
	}
}

/*
sourceReadDocs returns the entries for listing and reading sources and
the kinds that can be added.
*/
func sourceReadDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.source-types.index": {
			Title: "List knowledge source types",
			Use: "Lists the tools a knowledge source can be added from, with the settings each needs, the " +
				"modes it supports, what it can feed and how often it may sync.",
			Returns: "every source type with its settings schema and abilities.",
			Next:    []string{"dimmi_knowledge_sources_previews_create: try settings before adding"},
		},
		"knowledge.sources.index": {
			Title:   "List knowledge sources",
			Use:     "Lists the workspace's knowledge sources and how each one's syncing goes.",
			Returns: "every source with its type, mode, targets, schedule, status, counts and last sync.",
		},
		"knowledge.sources.show": {
			Title:   "Get a knowledge source",
			Use:     "Reads one source. Secrets are never sent back; secrets_set names the ones stored.",
			Returns: "the source with its settings, patterns, schedule, status, counts and last sync.",
			Next: []string{
				"dimmi_knowledge_sources_documents_list: what it read",
				"dimmi_knowledge_sources_syncs_list: its recent syncs",
			},
		},
		"knowledge.sources.tree.show": {
			Title:   "Get a knowledge source's tree",
			Use:     "Lists the items a saved source can choose from, such as spaces, folders or pages, as a tree.",
			Returns: "the tree of items with which are chosen.",
			Hint:    "knowledge_source_tree_unsupported means this type has no items to choose from.",
		},
	}
}

/*
sourceChangeDocs returns the entries for adding, changing and removing
sources.
*/
func sourceChangeDocs() map[string]Doc {
	return map[string]Doc{
		"knowledge.sources.store": {
			Title: "Add a knowledge source",
			Use: "Connects a tool as a knowledge source. Its settings are checked against the real service " +
				"before it is saved, so a wrong key fails at once. Only use credentials the person gave you " +
				"for this.",
			Returns: "the source.",
			Next:    []string{"dimmi_knowledge_sources_syncs_create: read it now"},
			Params:  sourceParams(),
		},
		"knowledge.sources.update": {
			Title: "Change a knowledge source",
			Use: "Changes a source's name, settings, patterns, targets or schedule. A change of what it reads " +
				"starts a sync at once. Type and mode never change.",
			Returns: "the source as it now is.",
			Params:  sourceParams(),
		},
		"knowledge.sources.destroy": {
			Title:   "Remove a knowledge source",
			Use:     "Removes a source with every document it read; the agents stop answering from them.",
			Returns: "nothing once it is removed.",
		},
	}
}

/*
sourceParams explains the fields of a knowledge source.
*/
func sourceParams() map[string]string {
	return map[string]string{
		"type": "Source type, from dimmi_knowledge_source_types_list. Read on create only.",
		"name": "A name for the team, up to 80 characters.",
		"mode": "sync keeps reading it for the agents; import turns it once into help center articles. " +
			"Read on create only.",
		"targets": "What it feeds: agent, operator and help_center. Defaults to agent and operator for sync, " +
			"help_center for import.",
		"help_center_id": "Help center (hc_...) an import or a help_center target writes to. Find it with " +
			"dimmi_help_centers_list.",
		"settings": "The object the type's settings schema describes, secrets included.",
		"include":  "Patterns of documents to read, up to 50; empty reads everything.",
		"exclude":  "Patterns of documents to leave out, up to 50.",
		"schedule": "manual, hourly, daily or weekly, as the type allows.",
		"sync_now": "true starts a sync as soon as it is saved.",
	}
}
