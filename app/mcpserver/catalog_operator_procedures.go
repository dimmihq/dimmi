/*
|--------------------------------------------------------------------------
| Tool Catalog: Operator And Procedures
|--------------------------------------------------------------------------
|
| The entries for Operator, the team's own agent, with its threads,
| changesets, memories, settings and usage, and for the procedures the
| AI agent follows.
|
*/

package mcpserver

import "maps"

/*
operatorProcedureDocs returns the entries for Operator, procedures and
workflows.
*/
func operatorProcedureDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		operatorThreadDocs(), operatorChangesetDocs(), operatorSettingsDocs(), procedureDocs(),
		procedurePublishDocs(), workflowDocs(), workflowChangeDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
operatorThreadDocs returns the entries for talking with Operator.
*/
func operatorThreadDocs() map[string]Doc {
	return map[string]Doc{
		"operator.threads.index": {
			Title:   "List Operator threads",
			Use:     "Lists your own threads with Operator, the team's agent, latest first. Threads are private.",
			Returns: "the threads with their title, conversation and last message time.",
		},
		"operator.threads.store": {
			Title: "Open an Operator thread",
			Use: "Opens your latest thread with Operator about a conversation, or starts one; without a " +
				"conversation it starts a thread about the whole workspace. Opening spends nothing.",
			Returns: "the thread with its messages, oldest first, and the changes Operator proposed in it.",
			Next:    []string{"dimmi_operator_threads_messages_create: ask Operator something on it"},
			Params: map[string]string{
				"conversation_id": "Conversation (conv_...) the thread is about; leave out for the workspace.",
				"fresh":           "true starts a new thread even when one about the conversation exists.",
			},
		},
		"operator.threads.show": {
			Title:   "Get an Operator thread",
			Use:     "Reads one of your threads with Operator.",
			Returns: "the thread with its messages, oldest first, and the changesets proposed in it.",
		},
		"operator.threads.destroy": {
			Title: "Delete an Operator thread",
			Use:   "Deletes one of your own threads with Operator and its messages. Changes already applied stay.",
			Risk:  new(RiskWrite),
		},
		"operator.threads.messages.store": {
			Title: "Ask Operator",
			Use: "Asks Operator a question or gives it a task on a thread and waits for its answer. Every " +
				"question runs real models and spends the workspace's AI budget. Operator proposes changes as " +
				"changesets to apply; groups set to auto in its settings apply at once.",
			Returns: "your message, Operator's answer and the changesets it proposed.",
			Next: []string{
				"dimmi_operator_changesets_applications_create: apply a proposed change the person approved",
				"dimmi_operator_changesets_declines_create: turn a proposed change down",
			},
			Params: map[string]string{"body": "What to ask, in plain words, up to 8000 characters."},
			Hint:   "A teammate may ask at most 30 questions a minute; wait and try again.",
		},
		"operator.usage.show": {
			Title: "Get Operator usage",
			Use: "Reads one month of Operator's runs: cost, tokens, failures, by purpose, teammate and model, " +
				"and what became of its changesets. Months are counted in UTC; the spending cap is shared with " +
				"the AI agent.",
			Returns: "the month's cost in micro-dollars, counts, daily series, breakdowns and latest runs.",
			Params:  map[string]string{"period": "The month as YYYY-MM. Leave out for the current month."},
		},
	}
}

/*
operatorChangesetDocs returns the entries for Operator's proposed
changes and memories.
*/
func operatorChangesetDocs() map[string]Doc {
	return map[string]Doc{
		"operator.changesets.index": {
			Title: "List Operator changesets",
			Use: "Lists the changes Operator proposed that you may review, newest first: pending, applied, " +
				"declined, reverted or failed.",
			Returns: "the changesets with their title, reason, status and the operations each holds.",
		},
		"operator.changesets.applications.store": {
			Title: "Apply an Operator changeset",
			Use: "Applies every operation of a changeset as you, in one go: it changes real data, such as tags, " +
				"assignments, statuses, notes, contacts, article drafts, and can run AI actions in the " +
				"workspace's systems. Show the operations and apply only when the person agrees.",
			Returns: "the changeset as applied, or failed with why and nothing changed.",
			Next:    []string{"dimmi_operator_changesets_reversions_create: undo it if it was wrong"},
			Risk:    new(RiskDanger),
			Hint:    "A failed changeset changed nothing; fix the cause and apply again, or decline it.",
		},
		"operator.changesets.declines.store": {
			Title:   "Decline an Operator changeset",
			Use:     "Turns down a changeset Operator proposed. Nothing changes.",
			Returns: "the changeset, declined.",
		},
		"operator.changesets.reversions.store": {
			Title: "Revert an Operator changeset",
			Use: "Undoes an applied changeset, newest operation first. A field someone changed since is left " +
				"as they set it.",
			Returns: "the changeset, reverted.",
		},
		"operator.memories.index": {
			Title:   "List Operator memories",
			Use:     "Lists what you asked Operator to remember. Memories are yours alone.",
			Returns: "your memories, newest first.",
		},
		"operator.memories.destroy": {
			Title: "Forget an Operator memory",
			Use:   "Makes Operator forget one of your memories.",
			Risk:  new(RiskWrite),
		},
	}
}

/*
operatorSettingsDocs returns the entries for Operator's settings.
*/
func operatorSettingsDocs() map[string]Doc {
	return map[string]Doc{
		"operator.settings.show": {
			Title: "Get Operator settings",
			Use: "Reads how Operator is set up for the workspace: on or off, briefs, triage, instructions and " +
				"policies.",
			Returns: "the settings with each group's policy and the groups that may apply at once.",
		},
		"operator.settings.update": {
			Title: "Change Operator settings",
			Use: "Changes Operator for the whole workspace, including which changes it may apply without asking. " +
				"Only the fields sent change; policies merge by group.",
			Returns: "the settings as they now are.",
			Risk:    new(RiskDanger),
			Params: map[string]string{
				"policies": "Group to policy: groups are conversations, notes, contacts, articles, procedures " +
					"and actions; policies are auto (apply at once), ask (wait for a teammate) or off. Articles, " +
					"procedures and actions always ask.",
				"brief_on_open":     "true has Operator brief a teammate when they open a conversation.",
				"triage_on_arrival": "true has Operator triage new conversations as they arrive.",
				"instructions":      "How the team wants Operator to work, up to 8000 characters.",
				"model":             "A model the agent's provider key offers; empty uses the agent's own.",
			},
		},
	}
}

/*
procedureDocs returns the entries for reading and editing procedures.
*/
func procedureDocs() map[string]Doc {
	return map[string]Doc{
		"procedures.index": {
			Title:   "List procedures",
			Use:     "Lists the procedures, the step-by-step playbooks the AI agent follows for kinds of requests.",
			Returns: "every procedure with its status and how many conversations used it in 30 days.",
		},
		"procedures.show": {
			Title:   "Get a procedure",
			Use:     "Reads one procedure's working copy and whether it differs from the live one.",
			Returns: "the procedure with its definition, status and lock_version.",
		},
		"procedures.store": {
			Title:   "Create a procedure",
			Use:     "Creates an empty draft procedure. The agent does not follow it until published.",
			Returns: "the draft procedure.",
			Next:    []string{"dimmi_procedures_update: write its steps"},
			Params:  map[string]string{"name": "A name unique in the workspace, 1 to 80 characters."},
		},
		"procedures.update": {
			Title: "Edit a procedure",
			Use: "Saves the name or the working copy of a procedure. Customers see no change until it is " +
				"published.",
			Returns: "the procedure as saved, with its new lock_version.",
			Next:    []string{"dimmi_procedures_publications_create: make it live"},
			Params: map[string]string{
				"definition": "JSON with description (when it applies, up to 500 characters), steps (1 to 30, " +
					"each with id, kind instruction, condition or action, text, and action_id for an action) " +
					"and handover (up to 10 cases to hand to a person).",
				"lock_version": "The lock_version you read, so a save never overwrites someone else's.",
			},
			Hint: "edit_conflict means someone saved since you read it: read it again and redo the change.",
		},
		"procedures.destroy": {
			Title: "Delete a procedure",
			Use:   "Deletes a procedure. The agent stops following it at once if it was live.",
		},
	}
}

/*
procedurePublishDocs returns the entries for publishing procedures.
*/
func procedurePublishDocs() map[string]Doc {
	return map[string]Doc{
		"procedures.publications.store": {
			Title: "Publish a procedure",
			Use: "Makes the working copy live: the AI agent follows it with customers from the next message. " +
				"Every action it takes must exist and be published.",
			Returns: "the procedure, published.",
			Risk:    new(RiskReach),
			Hint:    "procedure_invalid lists the problems; fix them with dimmi_procedures_update.",
		},
		"procedures.publications.destroy": {
			Title:   "Unpublish a procedure",
			Use:     "Takes a procedure out of the AI agent's hands at once. The working copy stays.",
			Returns: "the procedure, unpublished.",
			Risk:    new(RiskReach),
		},
	}
}
