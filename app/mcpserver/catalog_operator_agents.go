/*
|--------------------------------------------------------------------------
| Tool Catalog: Doing Operator's Work
|--------------------------------------------------------------------------
|
| The entries that let any connected agent do what Operator does:
| suggest changes a teammate reviews, each kind through a tool that
| requires what that kind needs, see what was suggested lately so
| nothing is suggested twice, keep a routine's state between runs, say
| a run found nothing, and read the skills Operator's own runs read.
|
*/

package mcpserver

/*
The words the suggestion tools share for the arguments they share.
*/
const (
	routineText = "The id of the routine this is for, such as ort_01jx2m8k4t. Leave out when no routine asked " +
		"for it."
	dryRunText = "true while trying a routine out: only a preview is left and nothing is kept. Needs routine_id."
	reasonText = "Why, in one or two short, plain sentences a teammate reads at a glance: what is wrong today " +
		"and what this changes. No tool names or ids."
	dedupText = "A stable key for what this suggests, such as stale:art_01jx2m8k4t. A suggestion with the same " +
		"key that still waits, or was declined in the last 30 days, is not made again."
	outcomeText = "made (false when the same suggestion was made before), a message saying what happened, and " +
		"the suggestion a teammate reviews."
)

/*
operatorSuggestionDocs returns the entries that suggest changes.
*/
func operatorSuggestionDocs() map[string]Doc {
	return map[string]Doc{
		"operator.suggestions.article-edits.store":   articleEditSuggestionDoc(),
		"operator.suggestions.new-articles.store":    newArticleSuggestionDoc(),
		"operator.suggestions.procedure-edits.store": procedureEditSuggestionDoc(),
		"operator.suggestions.changes.store":         changesSuggestionDoc(),
	}
}

/*
changesSuggestionDoc is the entry that suggests changes to
conversations and customers.
*/
func changesSuggestionDoc() Doc {
	return Doc{
		Name:  "propose_changes",
		Title: "Suggest changes to conversations",
		Use: "Suggests changes to conversations and customers as one suggestion a teammate reviews: tag or " +
			"untag, assign, set priority, open or close, rename, add an internal note, update a customer's " +
			"name, or add a regression test to a procedure. Articles and procedure edits have their own " +
			"tools: dimmi_propose_article_edit, dimmi_propose_new_article and dimmi_propose_procedure_edit.",
		Returns: outcomeText,
		Params: map[string]string{
			"operations": "The changes, each an object with kind and what that kind needs, such as " +
				"[{\"kind\": \"conversation.tag\", \"conversation_number\": 1042, \"tags\": [\"billing\"]}]. " +
				"Kinds: conversation.tag and conversation.untag (conversation_number, tags), " +
				"conversation.assign (conversation_number, member_id or team_id), conversation.priority " +
				"(conversation_number, priority), conversation.status (conversation_number, status open or " +
				"closed), conversation.title (conversation_number, new_subject), conversation.note " +
				"(conversation_number, note_text), contact.update (contact_id, customer_name), " +
				"procedure.create_test_case (procedure_id, test_case).",
			"suggestion_title": "What it does in a few everyday words, such as Route to Billing.",
			"reason":           reasonText, "dedup_key": dedupText, "routine_id": routineText, "dry_run": dryRunText,
		},
	}
}

/*
operatorAgentDocs returns the entries for what was suggested lately, a
routine's state, a quiet run and the skills.
*/
func operatorAgentDocs() map[string]Doc {
	return map[string]Doc{
		"operator.suggestions.recent.index": recentSuggestionsDoc(),
		"operator.routines.state.show": {
			Name:    "get_routine_state",
			Title:   "Get a routine's saved state",
			Use:     "Reads what a routine kept from its earlier runs, such as cursors and what it covered.",
			Returns: "the state as a JSON object, empty when nothing was saved.",
			Next:    []string{"dimmi_save_routine_state: save what the next run needs"},
		},
		"operator.routines.state.update":            saveRoutineStateDoc(),
		"operator.routines.nothing-to-change.store": nothingToChangeDoc(),
		"operator.knowledge.search.index":           searchKnowledgeDoc(),
		"operator.skills.index": {
			Name:    "list_skills",
			Title:   "List skills",
			Use:     "Lists the skills: the steps for doing each kind of routine work well, and when to read each.",
			Returns: "the skills with skill_name and when_to_use.",
			Next:    []string{"dimmi_get_skill_by_name: read a skill's steps before starting the work"},
		},
		"operator.skills.show": {
			Name:    "get_skill_by_name",
			Title:   "Read a skill",
			Use:     "Reads how to do one kind of routine work well, step by step, before starting it.",
			Returns: "the skill with its steps.",
			Params: map[string]string{
				"skill": "The skill's name, such as audit_articles. Find the names with dimmi_list_skills.",
			},
		},
	}
}

/*
articleEditSuggestionDoc is the entry that suggests a fix to an article.
*/
func articleEditSuggestionDoc() Doc {
	return Doc{
		Name:  "propose_article_edit",
		Title: "Suggest a fix to an article",
		Use: "Suggests a fix to one help article: the passages to replace, each quoted from the article " +
			"with what it should say instead. A teammate reviews it; the article changes only when they " +
			"approve. Read the article with dimmi_get_article_by_id first. Call it once per article.",
		Returns: outcomeText,
		Params: map[string]string{
			"article_id": "The id of the article to fix, such as art_01jx2m8k4t, as dimmi_get_article_by_id " +
				"and dimmi_list_help_center_articles show it. Never its title.",
			"edits": "The passages to replace, such as [{\"find\": \"Refunds take 5 days.\", \"replace\": " +
				"\"Refunds take 10 days.\"}]. find is the passage as the article has it now, copied word for " +
				"word from body_markdown: one whole sentence or list item is best; it is replaced everywhere " +
				"it appears. replace is what it should say instead, in Markdown; empty removes the passage.",
			"suggestion_title": "What the fix does in a few everyday words, such as Fix the refund window " +
				"in the returns guide. Not the article's title.",
			"reason": reasonText, "dedup_key": dedupText, "routine_id": routineText, "dry_run": dryRunText,
			"new_article_title": "Only to rename the article: its new title, such as Returns and refunds.",
		},
		Hint: "When a passage is not found, the reason quotes the closest passage: copy it into find.",
	}
}

/*
newArticleSuggestionDoc is the entry that suggests a new article.
*/
func newArticleSuggestionDoc() Doc {
	return Doc{
		Name:  "propose_new_article",
		Title: "Suggest a new article",
		Use: "Suggests a new help article as a draft a teammate reviews; nothing is created until they " +
			"approve. Call it once per article.",
		Returns: outcomeText,
		Params: map[string]string{
			"article_title": "The new article's title, as readers see it, such as Set up single sign-on.",
			"article_body":  "The whole article in Markdown, such as ## Before you start ...",
			"suggestion_title": "What the suggestion is in a few everyday words, such as Add a guide to " +
				"SSO setup.",
			"reason": reasonText, "dedup_key": dedupText, "routine_id": routineText, "dry_run": dryRunText,
			"visibility": "public for customers, or agent_only for a snippet only the AI agent reads. " +
				"public when left out.",
			"collection_id": "The id of the collection to file it in, such as col_01jx2m8k4t. Leave out " +
				"for none. Find it with dimmi_list_help_center_collections.",
			"help_center_id": "The id of the help center, such as hc_01jx2m8k4t. Leave out for the " +
				"default one. Find it with dimmi_list_help_centers.",
		},
	}
}

/*
procedureEditSuggestionDoc is the entry that suggests an edit to a procedure.
*/
func procedureEditSuggestionDoc() Doc {
	return Doc{
		Name:  "propose_procedure_edit",
		Title: "Suggest an edit to a procedure",
		Use: "Suggests an edit to one procedure's working copy. A teammate reviews it; the procedure " +
			"changes only when they approve. Read it with dimmi_get_procedure_by_id first. Call it once " +
			"per procedure.",
		Returns: outcomeText,
		Params: map[string]string{
			"procedure_id": "The id of the procedure to edit, such as prc_01jx2m8k4t, as " +
				"dimmi_list_procedures shows it. Never its name.",
			"procedure_definition": "The procedure's whole new working copy as a JSON object, as " +
				"dimmi_get_procedure_by_id shows it, with your changes made.",
			"suggestion_title": "What the edit does in a few everyday words, such as Ask for the order " +
				"number before refunding.",
			"reason": reasonText, "dedup_key": dedupText, "routine_id": routineText, "dry_run": dryRunText,
		},
	}
}

/*
saveRoutineStateDoc is the entry that saves a routine's state.
*/
func saveRoutineStateDoc() Doc {
	return Doc{
		Name:  "save_routine_state",
		Title: "Save a routine's state",
		Use: "Saves what the next run of a routine needs, as a JSON object of at most 64 KB. Keys given " +
			"replace the saved ones; every other saved key stays.",
		Returns: "the state as it now stands, and saved (false for a dry run).",
		Risk:    new(RiskWrite),
		Params: map[string]string{
			"state": "What to save, as a JSON object, such as {\"next_offset\": 15}.",
			"replace_everything": "true to drop every key saved before and keep only this object. Leave out " +
				"to keep them.",
			"dry_run": "true while trying a routine out: nothing is saved.",
		},
	}
}

/*
nothingToChangeDoc is the entry that reports a run that found nothing.
*/
func nothingToChangeDoc() Doc {
	return Doc{
		Name:  "report_nothing_to_change",
		Title: "Report that nothing needs changing",
		Use: "Says a run of a routine found nothing that needs the team's attention, so nobody is " +
			"notified. Call it at the end of a run that suggested nothing.",
		Returns: "recorded (false for a dry run) and a message.",
		Risk:    new(RiskWrite),
		Params: map[string]string{
			"reason": "One line on what you checked and why nothing needs changing, such as Read 15 " +
				"articles; all agree with each other.",
			"dry_run": "true while trying a routine out: nothing is recorded.",
		},
	}
}

/*
searchKnowledgeDoc is the entry that searches the knowledge.
*/
func searchKnowledgeDoc() Doc {
	return Doc{
		Name:  "search_knowledge",
		Title: "Search the knowledge",
		Use: "Searches the knowledge the AI agent answers customers from: published help articles and " +
			"connected sources. Use it to find what the help center already says about something.",
		Returns: "up to 8 passages, each with its title, article_id when it is from an article, url and text.",
		Next:    []string{"dimmi_get_article_by_id: read the whole article a passage is from"},
		Params:  map[string]string{"search_text": "What to look for, in a few words, such as refund window."},
	}
}

/*
recentSuggestionsDoc is the entry that lists what was suggested lately.
*/
func recentSuggestionsDoc() Doc {
	return Doc{
		Name:  "list_recent_suggestions",
		Title: "List recent suggestions",
		Use: "Lists the changes suggested lately, newest first, each with its status, dedup_key and what it " +
			"changes in a line. Call it before suggesting, so nothing is suggested twice.",
		Returns: "up to 50 suggestions with status, suggested_on, suggestion_title, dedup_key and changes.",
		Params: map[string]string{
			"days_back":  "How many days back to look, from 1 to 90, such as 30. 30 when left out.",
			"status":     "Only suggestions with this status, such as pending. Leave out for all.",
			"routine_id": "Only suggestions made for this routine, such as ort_01jx2m8k4t. Leave out for all.",
		},
	}
}
