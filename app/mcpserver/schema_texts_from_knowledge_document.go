/*
|--------------------------------------------------------------------------
| Written Schema Texts: KnowledgeDocument To Member
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromKnowledgeDocument returns the texts from KnowledgeDocument on.
*/
func schemaTextsFromKnowledgeDocument() map[string]map[string]string {
	return map[string]map[string]string{
		"KnowledgeDocument": {
			"excluded_by": "Why the document is left out: member, robots, pattern, js_rendered, " +
				"duplicate, too_large, unsupported_type, empty or draft. Null when it is not excluded.",
			"external_id": "The document's own id at its source, such as its address for a website " +
				"page. Unique within the source.",
			"fetched_at": "When the document was last fetched from its source. Null when it never was.",
			"indexed_at": "When the document was last made searchable. Null when it is not indexed " +
				"yet.",
			"linked_article_id": "Id of the help center article this document was imported into, " +
				"starting with art_. Null when there is none.",
			"locale": "The document's language as a language code, such as en. Empty when unknown.",
			"path": "Where the document sits in its source, such as /docs/billing/refunds. Include and " +
				"exclude patterns match against it.",
			"status": "Where the document stands: pending, active (searchable), excluded, error or " +
				"removed.",
			"title": "The document's title. Empty when it has none.",
			"url":   "The document's address at its source. Null when it has none.",
		},
		"KnowledgeOptions": {
			"options": "The values the setting can take, each with the value to store, the name shown, " +
				"a short detail and whether it is preselected.",
		},
		"KnowledgeReembed": {
			"done":       "How many pieces of text have been embedded with the new model so far.",
			"failed":     "How many pieces of text the provider refused to embed.",
			"started_at": "When building the new model's vectors began. Null when no build has run.",
			"total":      "How many distinct pieces of text the new model has to embed in all.",
		},
		"KnowledgeSettings": {
			"active_model": "The embedding model searches use now, written provider:model@dims, such " +
				"as openai:text-embedding-3-small@1024. Null when none is set.",
		},
	}
}

/*
schemaTextsFromKnowledgeSettings returns the texts from KnowledgeSettings on.
*/
func schemaTextsFromKnowledgeSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"KnowledgeSettings": {
			"builtin_model": "The embedding model built into this install, written " +
				"provider:model@dims. Null when the install has none.",
			"embedding_key_id": "Id of the AI provider key the active model embeds with, starting with " +
				"aik_. Null means the built-in embedder.",
			"embeddings_error": "Why embeddings are not healthy, in words. Null when there is no " +
				"error.",
			"embeddings_status": "Where the workspace's embeddings stand: none, ready, building (a " +
				"switch is in progress) or degraded.",
			"pending_key_id": "Id of the AI provider key the pending model embeds with, starting with " +
				"aik_. Null for the built-in embedder or when no switch is under way.",
			"pending_model": "The model being built for a switch, written provider:model@dims. Null " +
				"when no switch is under way.",
			"previous_model": "The model used before the last switch, written provider:model@dims. " +
				"Null when there was none.",
			"switched_at": "When the workspace last switched to its active model. Null when it never " +
				"switched.",
		},
		"KnowledgeSource": {
			"counts": "How many of the source's documents are in each status, keyed by status, such as " +
				"{\"active\": 120, \"excluded\": 4}.",
			"document_count": "How many documents the source holds.",
			"exclude":        "Path patterns of documents to leave out; empty when none.",
			"help_center_id": "Id of the help center the source's documents go to, starting with hc_. " +
				"Set when the mode is import or help_center is a target; null otherwise.",
			"include":         "Path patterns of documents to keep; empty keeps everything.",
			"last_error":      "What went wrong in the last sync, in words. Null when there is no error.",
			"last_error_code": "Short code of the last sync's error. Null when there is no error.",
			"last_success_at": "When a sync last succeeded. Null when none has yet.",
			"last_synced_at": "When the source was last synced, whether or not it succeeded. Null when " +
				"it never was.",
			"mode": "How the source is used: sync keeps a read-only copy refreshed on a schedule; " +
				"import copies it once into editable help center articles.",
			"name": "The name the team gave this source.",
			"next_sync_at": "When the next scheduled sync is due. Null when none is planned, as with a " +
				"manual schedule.",
		},
	}
}

/*
schemaTextsFromKnowledgeSource returns the texts from KnowledgeSource on.
*/
func schemaTextsFromKnowledgeSource() map[string]map[string]string {
	return map[string]map[string]string{
		"KnowledgeSource": {
			"schedule": "How often the source is synced: manual, hourly, daily or weekly.",
			"secrets_set": "Names of the secret settings that have a value saved; the values " +
				"themselves are never shown. Empty when none.",
			"settings": "The source's own settings without its secrets, an object shaped by the " +
				"settings schema of its type, such as a website's start address.",
			"status": "Where the source stands: idle, queued, syncing, error (see last_error) or " +
				"paused.",
			"targets": "Who reads the source's documents: any of agent (the AI agent), operator and " +
				"help_center.",
			"type": "What kind of source it is, such as website, notion, github, markdown_upload, " +
				"zendesk_help_center or intercom_help_center.",
		},
		"KnowledgeSourcePreview": {
			"account": "The account reached at the source's service, as it names itself. Empty when " +
				"the source has no account.",
			"estimate": "The service's own estimate of how many documents the source holds; 0 when it " +
				"gives none.",
			"listed":  "How many documents the first page of the source listed.",
			"matched": "How many of the listed documents the include and exclude patterns keep.",
			"more":    "True when the source has more pages of documents beyond the first one listed.",
			"sample": "A few of the documents the source would read, each with its title, path and " +
				"address.",
		},
		"KnowledgeSourcePreviewSample": {
			"path":  "Where the sample document sits in its source, such as /docs/billing.",
			"title": "The sample document's title.",
			"url":   "The sample document's address at its source.",
		},
		"KnowledgeSourceType": {
			"auth": "How this kind of source signs in to its service: none or oauth.",
			"integration": "The connected service this kind of source reads through, such as notion or " +
				"github. Left out when it needs none.",
		},
	}
}

/*
schemaTextsFromKnowledgeSourceType returns the texts from
KnowledgeSourceType on.
*/
func schemaTextsFromKnowledgeSourceType() map[string]map[string]string {
	return map[string]map[string]string{
		"KnowledgeSourceType": {
			"logo":  "Key of the picture the dashboard shows for this kind of source, such as notion.",
			"modes": "The modes this kind of source supports: sync, import or both.",
			"name":  "The name people see for this kind of source, such as Website or Notion.",
			"schedules": "The sync schedules that make sense for this kind of source, the first being " +
				"the default, from manual, hourly, daily and weekly.",
			"schema": "The JSON Schema of this kind of source's settings; properties marked x-secret " +
				"are stored sealed and never shown again.",
			"summary": "One line saying what this kind of source reads.",
			"targets": "Who may read documents of this kind of source: any of agent, operator and " +
				"help_center.",
			"type": "The key of this kind of source, such as website or notion. Send it as the type " +
				"when adding a source.",
		},
		"KnowledgeSyncRun": {
			"error":       "Why the run failed, in words. Null when it did not.",
			"error_code":  "Short code of the run's error. Null when it did not fail.",
			"finished_at": "When the run ended. Null while it is queued or running.",
			"full_run": "True when the run reads the whole source and removes documents it no longer " +
				"finds; false for a partial run.",
			"source_id":  "Id of the knowledge source the run belongs to, starting with ks_.",
			"started_at": "When the run began. Null while it is still queued.",
			"stats": "The run's counters as an object of numbers, such as created, updated, unchanged " +
				"and removed documents.",
			"status":  "Where the run stands: queued, running, succeeded, partial, failed or canceled.",
			"trigger": "What started the run: create, manual, schedule, webhook, retry or document.",
		},
		"KnowledgeTree": {
			"nodes": "The source's top-level items, each a container or a document with its id, title, " +
				"kind and the items under it.",
		},
	}
}

/*
schemaTextsFromKnowledgeUpload returns the texts from KnowledgeUpload on.
*/
func schemaTextsFromKnowledgeUpload() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"KnowledgeUpload": {
			"key": "The key that names the uploaded archive. Put it in a source's settings to read " +
				"from this upload.",
			"name": "The uploaded file's name.",
		},
		"KnowledgeUsage": {
			"lines": "The month's usage per source and kind, the largest first.",
			"period_start": "The first day of the month the usage is for, in UTC, written YYYY-MM-DD, " +
				"such as 2026-11-01.",
		},
		"KnowledgeUsageLine": {
			"browser_ms": "Browser time spent reading pages for this line, in milliseconds.",
			"estimated_tokens": "How many of the input tokens were estimated rather than reported by " +
				"the provider.",
			"input_tokens": "How many tokens were embedded for this line.",
			"kind":         "What was used: embedding (turning text into vectors) or crawl (reading pages).",
			"requests":     "How many calls were made for this line.",
			"source_id": "Id of the knowledge source, starting with ks_. Null for the workspace's own " +
				"articles and conversations.",
			"source_name": "Name of the knowledge source the usage belongs to.",
			"source_type": "The kind of that source, such as website or notion.",
		},
		"Launcher": {
			"icon": "The picture on the messenger's launcher button: chat, help or wave.",
			"image_asset_id": "Id of an uploaded image shown on the launcher instead of the icon. Left " +
				"out when none is set.",
			"label": "The text on the launcher, at most 24 characters. Empty when it shows none.",
			"shape": "The launcher button's shape: circle or squircle.",
			"style": "How the launcher shows on the page: bubble, tab or hidden.",
		},
	}
}

/*
schemaTextsFromLeafResult returns the texts from LeafResult on.
*/
func schemaTextsFromLeafResult() map[string]map[string]string {
	return map[string]map[string]string{
		"LeafResult": {
			"actual": "The value the person actually has for the field. Null when nothing is known.",
			"field":  "The field the condition reads, such as page.path or a customer attribute's key.",
			"op":     "The condition's comparison operator.",
			"path": "Where the condition sits in the rule, written as a path through its all, any and " +
				"not groups.",
			"result": "Whether the condition held: true, false, or unknown when it cannot be decided " +
				"from what is known.",
			"trust": "How far the field's value can be believed: verified, observed, claimed or " +
				"by_key.",
			"value": "The value the condition compares against, as written in the rule. Left out when " +
				"the condition has none.",
		},
		"LinkSafety": {
			"allow_domains": "Domains whose links skip the warning; each covers the domain and every " +
				"name under it. Empty when none.",
			"block_domains": "Domains whose links cannot be clicked; each covers the domain and every " +
				"name under it. Empty when none.",
			"enabled": "True when links in messages are checked and warned about before opening.",
			"strip_inbound_email_links": "True when links in received email show as plain text until a " +
				"teammate reveals them.",
		},
		"LinkedTicket": {
			"assignee_id": "Id of the teammate the linked ticket is assigned to, starting with mem_. " +
				"Null when nobody holds it.",
			"kind":      "The linked ticket's kind: customer, back_office or tracker.",
			"reference": "The linked ticket's number as the team reads it, such as T-142.",
			"title":     "The linked ticket's title.",
			"type_name": "Name of the linked ticket's type.",
		},
		"ListedTag": {
			"color": "The tag's color as a hex code, such as #2563eb. Null when it has none.",
		},
	}
}

/*
schemaTextsFromListedTag returns the texts from ListedTag on.
*/
func schemaTextsFromListedTag() map[string]map[string]string {
	return map[string]map[string]string{
		"ListedTag": {
			"icon": "Name of the tag's icon from the app's set, in lowercase letters, digits and " +
				"dashes. Null when it has none.",
			"name": "The tag's name.",
		},
		"Locale": {
			"code": "The language's BCP 47 code, such as de or pt-BR. Send it wherever a locale is " +
				"asked for.",
			"name": "The language's name written in that language, such as Deutsch for de.",
		},
		"Mapping": {
			"column": "The position of the file's column, counted from 0.",
			"field": "The field the column fills: email, name, external_id, phone, domain, " +
				"company_name, company_domain, tags, or attribute.<key> for a custom attribute.",
		},
		"Me": {
			"locale": "The language the caller reads in, as a language code such as en: their own " +
				"choice, else the workspace's, else the request's.",
			"workspaces": "The workspaces the caller can switch to. Null for personal access tokens, " +
				"which cannot switch.",
		},
		"Member": {
			"access_team_ids": "Ids of the teams the conversation access names, each starting with " +
				"team_. Used when conversation_access is teams or all_except_teams.",
			"assign_on_reply": "Whether replying to a conversation nobody holds assigns it to this " +
				"member: inherit (follow the inbox), always or never.",
			"availability": "Whether the member is taking new conversations: online or away.",
			"away_reassign": "True when a customer's answer to this member's conversation goes back to " +
				"the pool while the member is away.",
			"away_since": "Since when the member has been away. Null when they are not away.",
			"capacity_by_channel": "The most open conversations the member holds per channel type, " +
				"such as {\"email\": 20}. Empty for no per-channel limit.",
			"conversation_access": "Which conversations the member sees: all, assigned (only their " +
				"own), teams (their own plus the listed teams') or all_except_teams.",
		},
	}
}
