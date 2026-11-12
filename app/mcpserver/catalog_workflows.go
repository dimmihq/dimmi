/*
|--------------------------------------------------------------------------
| Tool Catalog: Workflows
|--------------------------------------------------------------------------
|
| The entries for workflows: the automation that runs on conversations,
| drafted, published as versions, paused, ordered and archived.
|
*/

package mcpserver

/*
workflowDocs returns the entries for reading and drafting workflows.
*/
func workflowDocs() map[string]Doc {
	return map[string]Doc{
		"workflows.index": {
			Title: "List workflows",
			Use: "Lists the workflows in priority order: when several want the same conversation, the first wins. " +
				"Each runs on a trigger, such as a first message or an SLA breach.",
			Returns: "every workflow with its trigger, status, position and whether its draft differs from live.",
			Params:  map[string]string{"archived": "true lists the archived workflows instead."},
		},
		"workflows.show": {
			Title:   "Get a workflow",
			Use:     "Reads one workflow with its working copy and the report of problems that would block publishing.",
			Returns: "the workflow with its spec, report, lock_version and draft_hash.",
		},
		"workflows.versions.index": {
			Title:   "List a workflow's versions",
			Use:     "Lists every published version of a workflow, newest first.",
			Returns: "the versions with their number, note, who published them and their spec.",
			Next:    []string{"dimmi_workflows_update: roll back with restore_version_id"},
		},
		"workflows.store": {
			Title: "Create a workflow",
			Use: "Creates a draft workflow at the end of the list, from a trigger or a whole definition. The " +
				"trigger never changes later. Nothing runs until it is published.",
			Returns: "the draft workflow.",
			Next: []string{
				"dimmi_workflows_update: build its steps",
				"dimmi_workflows_publications_create: make it live",
			},
			Params: map[string]string{
				"trigger": "conversation.opener, conversation.first_message, conversation.customer_reopened, " +
					"message.customer, conversation.customer_quiet, conversation.rated, message.teammate, " +
					"note.created, conversation.assignment_changed, conversation.status_changed, " +
					"conversation.teammate_quiet, dimmi.handed_over, dimmi.resolved, sla.warning, sla.breached " +
					"or reusable (called from other workflows).",
				"spec": "A whole definition to start from, in the shape dimmi_workflows_get returns.",
			},
		},
		"workflows.destroy": {
			Title: "Archive a workflow",
			Use: "Archives a workflow: it stops matching at once and leaves the list. Its versions stay, and " +
				"dimmi_workflows_unarchive_create brings it back.",
			Risk: new(RiskReach),
			Hint: "workflow_in_use means a live workflow still calls it; change that one first.",
		},
		"workflows.unarchive.store": {
			Title:   "Unarchive a workflow",
			Use:     "Brings an archived workflow back at the end of the list. One that was live comes back paused.",
			Returns: "the workflow.",
		},
	}
}

/*
workflowChangeDocs returns the entries for saving, publishing,
ordering and archiving workflows.
*/
func workflowChangeDocs() map[string]Doc {
	return map[string]Doc{
		"workflows.update": {
			Title: "Change a workflow",
			Use: "Saves the working copy, which never changes what runs; or sets status live or paused; or " +
				"restores a version, published at once with publish=true. Status and publish change what " +
				"customers get immediately.",
			Returns: "the workflow as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"spec": "The whole working copy, in the shape dimmi_workflows_get returns; its trigger must " +
					"match the workflow's.",
				"lock_version":               "The lock_version you read; 0 skips the check and the last save wins.",
				"status":                     "live or paused. Making it live needs a published version.",
				"restore_version_id":         "Version (wfv_...) to copy into the working copy.",
				"publish":                    "With restore_version_id, publishes the restored copy at once.",
				"acknowledge_warnings":       "With publish, true after the person saw the warnings.",
				"test_on_test_conversations": "true runs the workflow on test conversations only.",
			},
			Hint: "edit_conflict means someone saved since you read it: read it again and redo the change.",
		},
		"workflows.publications.store": {
			Title: "Publish a workflow",
			Use: "Freezes the working copy as a new version and makes it live: new events run it for real " +
				"customers, while conversations already running keep their version. Every error must be fixed " +
				"first; warnings need the person's yes.",
			Returns: "the workflow and the new version.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"note": "What changed, for the history, up to 280 characters.",
				"acknowledge_warnings": "true publishes despite warnings; set it only after showing them to the " +
					"person.",
				"draft_hash": "The draft_hash you reviewed, so a copy changed since is not published.",
			},
			Hint: "warnings_not_acknowledged lists the warnings: show them, then send acknowledge_warnings=true.",
		},
		"workflows.update-many": {
			Title: "Reorder workflows",
			Use: "Sets the priority order of the workflows; the first wins when several want a conversation, " +
				"so this changes what live workflows do.",
			Returns: "the workflows in their new order.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"order": "Every workflow id (wf_...) that is not archived, exactly once."},
			Hint:    "order_stale means the list changed: read it again with dimmi_workflows_list.",
		},
	}
}
