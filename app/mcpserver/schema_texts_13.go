/*
|--------------------------------------------------------------------------
| Written Schema Texts: NotificationDelivery To OperatorRecentSuggestion
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts85 returns the texts from NotificationDelivery on.
*/
func schemaTexts85() map[string]map[string]string {
	return map[string]map[string]string{
		"NotificationDelivery": {
			"member_id": "Id of the teammate the alert was for (starts with mem_). Null when the line " +
				"is not for a teammate, as for a Slack destination.",
			"occurred_at": "When the thing the alert is about happened, as an ISO 8601 time.",
			"reason": "Why it was skipped or failed, such as preference_off, no_route, quiet_hours, " +
				"paused, suppressed, ai_handled, already_seen, no_longer_needed, not_visible, cooldown, " +
				"rejected or error. Null when sent.",
			"status":  "How the delivery ended: sent, failed or skipped.",
			"title":   "Headline of the alert that was delivered.",
			"trigger": "What the alert was about, such as customer_replied or mentioned.",
		},
		"NotificationDeliveryListOutputBody": {
			"data": "The deliveries of this one notification, one line per channel it was tried on.",
		},
		"NotificationDestination": {
			"digest_time": "Time of day the digests post, as HH:MM in 24-hour form in the workspace's " +
				"time zone, such as 09:00.",
			"digests": "Digests this destination gets: any of daily_standup, daily_summary and " +
				"weekly_themes. Empty for none.",
			"inbox_ids": "Ids of the inboxes it hears about (each starts with inbox_). Null means " +
				"every inbox.",
			"include_ai_handled": "True when it also hears about conversations the AI agent is " +
				"answering.",
			"integration_id":    "Id of the Slack installation it posts through (starts with itg_).",
			"last_delivered_at": "When dimmi last posted to it successfully. Null when it never has.",
			"last_error":        "Message of the last failure to post. Null when there was none.",
			"last_error_code": "Code of the last failure to post, such as integration_disconnected or " +
				"slack_refused. Null when there was none.",
			"name":           "Name the destination is shown by in settings.",
			"next_digest_at": "When its next digest is due. Null when it gets no digests.",
			"scope": "Whose destination it is: workspace for a shared Slack channel, member for one " +
				"teammate's own direct message.",
			"show_previews": "True when its posts show the customers' words; false keeps message text " +
				"out of Slack.",
			"slack_channel_id": "Slack's id of the channel it posts in. Null for a direct message.",
		},
	}
}

/*
schemaTexts86 returns the texts from NotificationDestination on.
*/
func schemaTexts86() map[string]map[string]string {
	return map[string]map[string]string{
		"NotificationDestination": {
			"slack_channel_name": "Name of the Slack channel it posts in. Null for a direct message.",
			"status":             "Whether it works: active, or error after a failed post (see last_error).",
			"triggers": "Keys of the triggers it hears about, such as new_conversation or " +
				"ai_handoff_requested. Empty for none.",
			"type": "Kind of destination: slack_channel or slack_dm.",
		},
		"NotificationDestinationTest": {
			"destination_id": "Id of the destination that was tested (starts with ndst_).",
			"error":          "Why the test post failed, in words. Null when it was sent.",
			"error_code": "Code of the failure, such as integration_disconnected or slack_refused. " +
				"Null when it was sent.",
			"status": "How the test post ended: sent or failed.",
		},
		"NotificationPreferences": {
			"cells": "One entry per trigger and channel pair, saying whether the member gets it, its " +
				"default and whether it is locked on.",
			"channels": "The channels a notification can go on, in order: in_app, email, push, slack.",
			"triggers": "Every thing a teammate can be told about, in the order settings show them, " +
				"each with its key, label and description.",
		},
		"NotificationSettings": {
			"include_ai_handled": "True when the member is also told about conversations the AI agent " +
				"is answering.",
			"paused_until": "Notifications are paused until this time. Null when not paused.",
			"quiet_hours_bypass": "Trigger keys that still notify during quiet hours, such as " +
				"[\"mentioned\"]. Empty for none.",
			"quiet_hours_days": "Days quiet hours apply, as a bitmask from 0 to 127: Monday is 1, " +
				"Tuesday 2, Wednesday 4, up to Sunday 64. 127 is every day.",
			"quiet_hours_enabled": "True when quiet hours are on.",
			"quiet_hours_end": "When quiet hours end, as HH:MM in 24-hour form in the member's time " +
				"zone, such as 08:00. An end before the start runs past midnight.",
			"quiet_hours_start": "When quiet hours begin, as HH:MM in 24-hour form in the member's " +
				"time zone, such as 22:00.",
		},
	}
}

/*
schemaTexts87 returns the texts from NotificationSettings on.
*/
func schemaTexts87() map[string]map[string]string {
	return map[string]map[string]string{
		"NotificationSettings": {
			"show_previews": "True when notifications show the customers' words; false keeps message " +
				"text out of them.",
			"timezone": "The member's time zone for quiet hours, such as Europe/Paris. Null when they " +
				"have not set one.",
		},
		"NotificationTrigger": {
			"description": "A sentence explaining when this trigger fires, such as \"A teammate " +
				"mentions you in a note.\"",
			"key": "The trigger's key, such as mentioned or customer_replied; use it wherever a " +
				"trigger is named.",
			"label": "Short name settings show for the trigger, such as \"Mentions\".",
		},
		"NotificationUnsubscribe": {
			"email": "The email address the unsubscribe link is for.",
			"label": "Name of the kind of email the link turns off, such as \"Customer replies\".",
			"settings_url": "Web address of the notification settings in the app, for changing more " +
				"than this one email.",
			"subscribed": "True while this kind of email is still on for the member; false once " +
				"unsubscribed.",
			"trigger":   "Key of the trigger whose emails the link is about, such as customer_replied.",
			"workspace": "Name of the workspace the emails come from.",
		},
		"NotificationsOutputBody": {
			"data": "One page of the caller's notifications as the bell shows them.",
		},
		"OAuthAuthorization": {
			"client_id":     "Id of the app asking for access.",
			"client_name":   "Name of the app asking for access, as it gave it.",
			"loopback":      "True when the app answers on the person's own computer rather than a website.",
			"redirect_host": "Host the person is sent back to after deciding, such as app.example.com.",
			"scopes":        "The access the app asks for, as scope names.",
		},
	}
}

/*
schemaTexts88 returns the texts from OAuthAuthorization on.
*/
func schemaTexts88() map[string]map[string]string {
	return map[string]map[string]string{
		"OAuthAuthorization": {
			"self_registered": "True when the app registered itself, so dimmi cannot vouch for who " +
				"made it.",
			"workspaces": "The workspaces the person may connect the app to, each with its id, name " +
				"and the person's role there.",
		},
		"OAuthDecision": {
			"redirect_to": "Web address the browser must go to now that the person approved or denied " +
				"the app.",
		},
		"OAuthIdentity": {
			"avatar_url": "Picture of the account as the provider last reported it. Null when it gave " +
				"none.",
			"email": "Email of the account at the provider, as last reported. Null when it gave none.",
			"last_login_at": "When this account was last used to sign in, in UTC. Null until it has " +
				"been.",
			"provider": "Key of the sign-in provider the account is at, such as google or github.",
			"username": "Username of the account at the provider, as last reported. Null when it gave " +
				"none.",
		},
		"OAuthWorkspaceOption": {
			"name": "Name of a workspace the person may connect the app to.",
			"role": "The person's role in that workspace, such as owner, admin or member.",
		},
		"ObservedCall": {
			"action_id": "Id of the workspace action the simulation called (starts with aac_).",
			"args": "The arguments the action was called with, as a JSON object. Left out when there " +
				"were none.",
			"blocked": "True when the call was stopped before it ran; a blocked call does not count as " +
				"called in a test's assertions. Left out when false.",
			"succeeded": "True when the mocked call answered with success. Nothing real is run in a " +
				"simulation.",
			"tool": "The action's tool name, the name the AI calls it by.",
		},
	}
}

/*
schemaTexts89 returns the texts from Onboarding on.
*/
func schemaTexts89() map[string]map[string]string {
	return map[string]map[string]string{
		"Onboarding": {
			"completed_at": "When the workspace finished setup. Null while setup is still under way.",
			"step": "The setup step the workspace is on: workspace, logo, channels, tags, team, " +
				"knowledge, test_conversations or done.",
		},
		"OperatorAnswer": {
			"changesets": "The changes Operator proposed in this answer, each a changeset a teammate " +
				"reviews. Empty when it proposed none.",
		},
		"OperatorChangeset": {
			"applied_at": "When the changes were applied. Null while not applied.",
			"auto": "True when Operator applied it on its own under an auto policy, without a " +
				"teammate's approval.",
			"conversation_id": "Id of the conversation the changeset was proposed about (starts with " +
				"conv_). Null when it is not tied to one.",
			"decided_by": "Id of the teammate who applied or declined it (starts with mem_). Null when " +
				"no teammate has decided it.",
			"dedup_key": "Key that keeps the same suggestion from being made twice. Null when it has " +
				"none.",
			"error":      "Why applying it failed. Null unless status is failed.",
			"expires_at": "When a routine's suggestion counts as stale. Null when it does not expire.",
			"message_id": "Id of the Operator message that proposed it (starts with opm_). Null when " +
				"no message did.",
			"operations": "The changes in the set, one entry each, with its kind, a one-line label and " +
				"what it wrote or will write.",
			"preview": "True for a dry run's suggestion, which is shown only and never applied.",
			"proposed_to": "Id of the teammate it was proposed to (starts with mem_). Null when " +
				"Operator proposed it on its own, as a routine does.",
			"reason":      "Operator's explanation of why it proposes these changes. May be empty.",
			"reverted_at": "When the applied changes were undone. Null when they were not.",
			"routine_id": "Id of the routine that suggested it (starts with ort_). Null when it came " +
				"from a chat.",
			"routine_run_id": "Id of the routine run that suggested it (starts with orr_). Null when " +
				"it came from a chat.",
		},
	}
}

/*
schemaTexts90 returns the texts from OperatorChangeset on.
*/
func schemaTexts90() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorChangeset": {
			"status": "Where it stands: pending (waiting for review), applied, declined, reverted " +
				"(undone) or failed.",
			"thread_id": "Id of the Operator thread it was proposed in (starts with opt_). Null when " +
				"it has none.",
			"title": "Short name of what the changeset does, up to 200 characters.",
		},
		"OperatorKnowledgePassage": {
			"article_id": "Id of the help article the passage is from (starts with art_); send it " +
				"wherever an article is asked for. Null when it comes from a connected source.",
			"text":  "The words of the passage, as the AI agent reads them when it answers.",
			"title": "Title of the article or document the passage is from.",
			"url":   "Web address of the article or document the passage is from.",
		},
		"OperatorMemory": {
			"body": "What the caller asked Operator to remember, in their words, up to 1000 " +
				"characters.",
		},
		"OperatorMessage": {
			"body":   "Text of the turn: the teammate's question or Operator's answer.",
			"role":   "Who said it: user for the teammate, assistant for Operator.",
			"status": "How Operator's run for this turn ended: done, stopped or failed.",
			"steps": "The tool calls Operator made to answer, in order, as a JSON array; each has " +
				"tool, summary, ok and, when set, draft and changeset_id. Empty for a teammate's turn.",
		},
		"OperatorNothingToChange": {
			"message": "A sentence saying what happened, to read back.",
			"recorded": "True when the run was recorded as having found nothing to change. False for a " +
				"dry run, which records nothing.",
			"routine_id": "Id of the routine whose run this is (starts with ort_).",
		},
		"OperatorOperation": {
			"after": "The value after the change, as JSON. Filled when the operation is applied; left " +
				"out before.",
		},
	}
}

/*
schemaTexts91 returns the texts from OperatorOperation on.
*/
func schemaTexts91() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorOperation": {
			"article_id": "Id of the help article the operation changes (starts with art_). Left out " +
				"for other kinds.",
			"before": "The value before the change, as JSON, kept so it can be undone. Filled when the " +
				"operation is applied; left out before.",
			"conversation_number": "Number of the conversation the operation changes. Left out when it " +
				"does not change one.",
			"created_id": "Id of what the operation made once applied: the note (starts with msg_) or " +
				"the article (starts with art_). Left out otherwise.",
			"help_center_id": "Id of the help center a new article goes in (starts with hc_). Left out " +
				"for other kinds.",
			"kind": "What it does: conversation.tag, .untag, .assign, .priority, .status, .title or " +
				".note, contact.update, article.create_draft, article.edit_draft, procedure.edit_draft or " +
				"action.run.",
			"label": "One line saying what the operation does, for a reviewer to read.",
			"outcome": "What became of it: applied, reverted or skipped. Left out until the changeset " +
				"is decided.",
			"procedure_id": "Id of the procedure the operation changes (starts with prc_). Left out " +
				"for other kinds.",
			"text": "The full text it writes, such as a note, an article body or a procedure's working " +
				"copy, to read before applying. Left out when the label says it all.",
			"title":      "Title of the article it creates or retitles. Left out for other kinds.",
			"visibility": "Whether a new article is public or agent_only. Left out for other kinds.",
		},
		"OperatorRecentSuggestion": {
			"changes": "One line per change the suggestion holds, saying what it does.",
			"dedup_key": "Key that keeps the same suggestion from being made twice; reuse it to avoid " +
				"repeating it. Null when it has none.",
			"routine_id": "Id of the routine that made the suggestion (starts with ort_). Null when it " +
				"came from a chat.",
			"status":           "Where the suggestion stands: pending, applied, declined, reverted or failed.",
			"suggested_on":     "The UTC day it was made, as YYYY-MM-DD, such as 2026-11-20.",
			"suggestion_title": "Short name of what the suggestion does.",
		},
	}
}
