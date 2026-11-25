/*
|--------------------------------------------------------------------------
| Written Schema Texts: Workflow To WorkspaceSummary
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromWorkflow returns the texts from Workflow on.
*/
func schemaTextsFromWorkflow() map[string]map[string]string {
	return map[string]map[string]string{
		"Workflow": {
			"position": "The workflow's place in the list, which is also its priority when several " +
				"workflows want the same conversation; lower first.",
			"published_at": "When the workflow was last published. Null when it never was.",
			"spec": "The workflow's working copy: its steps and settings as an object. Present only " +
				"when one workflow is read, left out in lists.",
			"status": "Where the workflow stands: draft (never live), live (running on conversations) " +
				"or paused.",
			"test_on_test_conversations": "The workflow's setting for running on test conversations, " +
				"true or false.",
			"trigger": "The event that starts the workflow, fixed for its life, such as " +
				"conversation.first_message, message.customer or ticket.created.",
		},
		"WorkflowRun": {
			"conversation_id": "Id of the conversation the run worked on, such as conv_.... Null when " +
				"it has none.",
			"end_reason": "Why the run ended, such as completed, conversation_closed, " +
				"teammate_replied, no_action, expired or internal_error. Null while it has not ended.",
			"ended_at": "When the run ended. Null while it is running or waiting.",
			"parent_run_id": "Id of the run that called this one from a run-workflow step, such as " +
				"wfr_.... Null when nothing called it.",
			"started_at": "When the run started.",
			"status": "Where the run stands: running, waiting, completed, passed, exited, canceled, " +
				"failed or expired.",
			"steps":   "Every step the run executed, in order, with the path it took and its outcome.",
			"trigger": "The event that started the run, such as message.customer or ticket.created.",
		},
		"WorkflowRunStep": {
			"detail": "A few small facts the step kept, such as the member it picked, as an object. " +
				"Never holds message text. Empty object when there are none.",
			"finished_at": "When the step finished. Null while it has not.",
			"kind": "What the step does, such as message, branch, wait, assign, tag, close, " +
				"create_ticket or http_request.",
			"outcome": "What the step resulted in, as a short word. Null when it records none.",
		},
	}
}

/*
schemaTextsFromWorkflowRunStep returns the texts from WorkflowRunStep on.
*/
func schemaTextsFromWorkflowRunStep() map[string]map[string]string {
	return map[string]map[string]string{
		"WorkflowRunStep": {
			"path_id": "Id of the path the run took out of this step, as named in the workflow's " +
				"definition. Null when the step has no paths.",
			"started_at": "When the step started.",
			"status":     "How the step went: completed, waiting, skipped, failed or interrupted.",
			"step_id":    "The step's id within the workflow's definition.",
		},
		"WorkflowVersion": {
			"kind": "Whether this version talks to the customer or works behind the scenes: " +
				"customer_facing or background.",
			"note": "A note the publisher left about this version, up to 280 characters. Null when " +
				"none was left.",
			"number": "The version's number within its workflow, counting up from 1.",
			"published_by_member_id": "Id of the member who published this version, such as mem_.... " +
				"Null when unknown or the member is gone.",
			"published_via": "How the version was published, such as session for a person signed in. " +
				"Null when not recorded.",
			"restored_from_version_id": "Id of the older version this one was restored from, such as " +
				"wfv_.... Null when it was not a restore.",
			"spec": "The frozen workflow definition this version holds, as an object. It never " +
				"changes.",
			"workflow_id": "Id of the workflow this version belongs to, such as wf_....",
		},
		"Workspace": {
			"customer_facing_name": "The name customers see for the workspace. Null when they see the " +
				"workspace's own name.",
			"deleted_at": "When the workspace was moved to the trash. Null while it is not deleted.",
			"gif_picker": "True when teammates may search GIFs from the composer.",
			"locale":     "The workspace's language, such as en.",
			"logo_url":   "Web address of the workspace's logo on this server. Null when it has none.",
			"name":       "The workspace's name, 1 to 80 characters.",
			"purge_after": "When a deleted workspace is removed for good unless restored. Null while " +
				"it is not deleted.",
			"require_two_factor": "True when every member must have a second sign-in factor to work in " +
				"the workspace.",
		},
	}
}

/*
schemaTextsFromWorkspace returns the texts from Workspace on.
*/
func schemaTextsFromWorkspace() map[string]map[string]string {
	return map[string]map[string]string{
		"Workspace": {
			"seen_receipt_shows_name": "True when the customer's messenger names the teammate who read " +
				"their message.",
			"seen_receipts": "True when the customer's messenger is told that the team read their " +
				"message.",
			"show_viewers_on_list":  "True when inbox lists show who is viewing each conversation.",
			"slug":                  "The workspace's short name used in its web addresses.",
			"team_mentions_enabled": "True when a note may mention a whole team.",
			"test_mode": "True when the workspace is a test-mode workspace. It is fixed and cannot be " +
				"changed.",
			"timezone": "The workspace's time zone, as an IANA name such as UTC or Europe/Paris.",
			"undo_send_seconds": "How many seconds a reply waits before it leaves, during which its " +
				"writer can undo it. 5 unless the workspace changed it.",
		},
		"WorkspaceMember": {
			"access_team_ids": "Ids of the teams the member's conversation access names, such as " +
				"team_...; used by the teams and all_except_teams modes. Empty otherwise.",
			"assign_on_reply": "The member's own rule for taking a conversation by replying to it: " +
				"inherit (follow the workspace), always or never.",
			"availability": "What the member shows teammates and customers: online or away.",
			"away_reassign": "True when customers' answers go back to the pool while the member is " +
				"away.",
			"away_since": "Since when the member has been away. Null while they are online.",
			"capacity_by_channel": "The most open conversations the member holds per channel type, " +
				"such as {\"email\": 10, \"widget\": 5}. A channel left out has no limit of its own.",
			"conversation_access": "Which conversations the member sees: all, assigned (only their " +
				"own), teams (their own plus the listed teams') or all_except_teams.",
			"conversation_access_unassigned": "True when a narrowed access mode also shows the member " +
				"conversations nobody is assigned to.",
			"conversation_capacity": "The most open conversations the member holds overall. Null for " +
				"no limit.",
			"deactivated_at": "When the member was deactivated. Null while they are active.",
			"joined_via": "How the member arrived in the workspace, such as created, cli or an " +
				"invitation. Null when not recorded.",
			"last_active_at": "When the member last used the workspace. Null when never, and for " +
				"callers who do not manage members.",
			"role": "The member's role in the workspace: owner, admin or member.",
		},
	}
}

/*
schemaTextsFromWorkspaceMember returns the texts from WorkspaceMember on.
*/
func schemaTextsFromWorkspaceMember() map[string]map[string]string {
	return map[string]map[string]string{
		"WorkspaceMember": {
			"sign_in_locked": "True when failed sign-in guesses locked the member's account. Null for " +
				"callers who do not manage members.",
			"status": "Whether the member can work in the workspace: active or deactivated.",
			"teams": "The teams the member belongs to. Present only when the request asked for " +
				"members' teams.",
			"two_factor_enabled": "True when the member's account has a second sign-in factor. Null " +
				"for callers who do not manage members.",
		},
		"WorkspaceSummary": {
			"name": "The workspace's name.",
			"previous_slugs": "Slugs the workspace gave up and still holds, each until a set time, so " +
				"links naming an old slug still find it.",
			"role": "The caller's role in this workspace: owner, admin or member.",
			"slug": "The workspace's short name used in its web addresses.",
		},
	}
}
