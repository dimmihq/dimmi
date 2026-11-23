/*
|--------------------------------------------------------------------------
| Written Schema Texts: SeriesStep To SnoozeLists
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts120 returns the texts from SeriesStep on.
*/
func schemaTexts120() map[string]map[string]string {
	return map[string]map[string]string{
		"SeriesStep": {
			"entered_at": "When the traveler reached the step.",
			"kind": "Kind of step, such as entry, email, chat, post, slack, wait, wait_for, condition, " +
				"split, ai_path, tag, untag, set_attribute, topic, webhook or exit.",
			"left_at": "When the traveler left the step; null while they are still on it.",
			"node_id": "Id of the step in the series' definition.",
			"outcome": "What happened at the step, as a short code; null when nothing was recorded.",
			"path_id": "Id of the path the traveler left the step by, such as next, failed, matched, " +
				"timed_out, else, fallback, no_date or a path the step names; null if they have not left by " +
				"one.",
			"status": "How the step went: done, waiting, skipped or failed.",
		},
		"SeriesStepReport": {
			"message": "For a step that sends a message, the delivery counts of the broadcast it sends " +
				"through, as a JSON object. Left out for other steps.",
			"outcomes": "How many travelers left the step with each outcome, keyed by outcome code.",
			"paths": "How many travelers left the step by each path, keyed by path id such as next or " +
				"else.",
			"reached": "How many travelers reached the step.",
			"variants": "For a split, each variant with how many took it and how many of them reached " +
				"the goal. Left out for other steps.",
			"waiting": "How many travelers wait on the step now.",
		},
		"SeriesTotals": {
			"active":   "How many travelers are on the series now.",
			"entered":  "How many travelers entered the series.",
			"exited":   "How many travelers left the series before finishing.",
			"finished": "How many travelers reached the end of the series.",
			"goal":     "How many travelers reached the series' goal.",
		},
		"SeriesTrace": {
			"company_id": "Id of the company traveling the series, such as cmp_..., for a series of " +
				"companies; null for a series of contacts.",
		},
	}
}

/*
schemaTexts121 returns the texts from SeriesTrace on.
*/
func schemaTexts121() map[string]map[string]string {
	return map[string]map[string]string{
		"SeriesTrace": {
			"company_name": "Name of the company traveling. Only present when journeys are listed with " +
				"who travels them, and left out when empty.",
			"contact_email": "Email of the contact traveling. Only present when journeys are listed " +
				"with who travels them, and left out when empty.",
			"contact_id": "Id of the contact traveling the series, such as ctc_..., for a series of " +
				"contacts; null for a series of companies.",
			"contact_name": "Name of the contact traveling. Only present when journeys are listed with " +
				"who travels them, and left out when empty.",
			"end_reason": "Why the journey ended, a short lowercase code such as series_stopped or " +
				"node_removed. Null while it has not ended or when no reason was recorded.",
			"entered_at": "When the traveler entered the series.",
			"entered_by": "How the traveler entered: rules, event, manual, api or test.",
			"goal_at":    "When the traveler reached the series' goal; null if they have not.",
			"left_at":    "When the journey ended; null while the traveler is still on the series.",
			"node_id": "Id of the step in the series' definition where the traveler stands now, or " +
				"stood when the journey ended.",
			"series_id": "Id of the series this journey belongs to, such as srs_.... Send it wherever " +
				"a series is asked for.",
			"series_name": "Name of the series. Left out when the response does not fill it in.",
			"status":      "Where the journey stands: active, waiting, finished or exited.",
			"steps": "Every step the journey reached, each with the path it left by, what happened and " +
				"when.",
			"test":    "True for a test journey, which never counts in the series' report.",
			"wake_at": "When a waiting journey moves on; null when it is not waiting on a time.",
		},
		"SeriesVariant": {
			"entered": "How many travelers took this variant.",
			"goal":    "How many of them reached the series' goal.",
			"path":    "Id of the split's path this variant is.",
		},
		"SeriesVersion": {
			"note": "Note written when the version was published, up to 280 characters; empty when " +
				"none.",
			"number": "The version's number within its series.",
		},
	}
}

/*
schemaTexts122 returns the texts from SeriesVersion on.
*/
func schemaTexts122() map[string]map[string]string {
	return map[string]map[string]string{
		"SeriesVersion": {
			"published_by": "Id of the teammate who published the version, such as mem_...; null when " +
				"unknown.",
			"spec": "The series' definition as it was published, its steps and the paths joining them, " +
				"as JSON. It never changes.",
		},
		"ServiceProvider": {
			"acs_url": "SAML only: dimmi's address the identity provider posts sign-in answers to. " +
				"Give it to the identity provider.",
			"entity_id": "SAML only: dimmi's entity id for this connection. Give it to the identity " +
				"provider.",
			"metadata_url": "SAML only: address of dimmi's metadata for this connection, which the " +
				"identity provider can read its settings from.",
			"redirect_uri": "OpenID Connect only: dimmi's address sign-ins come back to. Register it " +
				"with the identity provider.",
		},
		"Session": {
			"app_version": "Version of the user's own app the session belongs to; null for a browser.",
			"auth_method": "How the session signed in, such as password, magic_link, email_code, " +
				"passkey, sso or google.",
			"auth_methods": "Every method used to sign the session in, such as password and totp; " +
				"empty when none is recorded.",
			"authenticated_at": "When the user last proved who they are on this session.",
			"client":           "Name of the user's own app the session belongs to; null for a browser.",
			"country":          "Country the session was last used from; null when unknown.",
			"current":          "True for the session making this request.",
			"device_label": "Readable name of the device or browser, such as Chrome on macOS; null " +
				"when unknown.",
			"expires_at":       "When the session ends.",
			"ip":               "IP address the session was last used from; null when unknown.",
			"kind":             "Kind of session, such as browser.",
			"last_activity_at": "When the session was last used.",
			"platform":         "Platform of the user's own app the session belongs to; null for a browser.",
			"two_factor_used":  "True when a second factor was used to sign the session in.",
		},
	}
}

/*
schemaTexts123 returns the texts from Settings on.
*/
func schemaTexts123() map[string]map[string]string {
	return map[string]map[string]string{
		"Settings": {
			"card_numbers": "True when payment card numbers are redacted from messages; on by default.",
			"us_ssn": "True when US social security numbers are redacted from messages; off by " +
				"default.",
		},
		"Setup": {
			"options": "For a passkey: the options the browser's credential API creates the passkey " +
				"with. Left out for other factors.",
			"otpauth_url": "For an authenticator app: the otpauth:// address that adds the account to " +
				"the app. Left out for other factors.",
			"qr_svg": "For an authenticator app: the otpauth address drawn as a QR code, as SVG " +
				"markup. Left out for other factors.",
			"secret": "For an authenticator app: the secret to type into the app by hand. Shown once; " +
				"left out for other factors.",
		},
		"SetupGuide": {
			"deep_link": "Link to the right settings screen in the mail service; null when there is " +
				"none.",
			"key": "Name of the mail service the guide is for, such as gmail, google_workspace or " +
				"microsoft_365.",
			"name":        "Name of the mail service as people read it, such as Google Workspace.",
			"provider":    "Same value as key, kept for older readers. Use key.",
			"recommended": "True for the guide that fits the address's domain; it is listed first.",
			"steps": "The steps to set forwarding up, in order, with this address's own addresses " +
				"filled in.",
			"warnings": "What tends to go wrong in this mail service; empty for none.",
		},
		"ShowWhen": {
			"equals": "Values of the other field under which this one shows. Left out when any value " +
				"is enough.",
			"field": "Key of the other ticket field that must have a value for this one to show.",
		},
		"SignedOutOutputBody": {
			"sessions_revoked": "How many of the contact's messenger sessions were ended.",
		},
	}
}

/*
schemaTexts124 returns the texts from SimHandoff on.
*/
func schemaTexts124() map[string]map[string]string {
	return map[string]map[string]string{
		"SimHandoff": {
			"note":     "Note the agent left for the team at the handover. Left out when there is none.",
			"priority": "Priority the handover set on the conversation. Left out when it set none.",
			"reason": "Why the simulated turn handed over, such as customer_requested, unknown, " +
				"frustration, safety or max_replies.",
			"team_id": "Id of the team the handover sent the conversation to, such as team_.... Left " +
				"out when it chose none.",
		},
		"SimPending": {
			"action_id": "Id of the action whose write is waiting, such as aac_....",
			"args":      "The arguments the action was called with, as an object.",
			"state": "What the write waits on: awaiting_confirmation for the customer's yes, " +
				"pending_approval for a teammate's approval.",
			"summary": "One line saying what the write would do.",
			"tool":    "Name of the tool the agent called for the action.",
		},
		"SimState": {
			"branches": "Branches of the procedure taken so far in the simulated conversation, as an " +
				"object of strings. Send the whole state back with the next turn.",
			"called_action_ids": "Ids of the actions called so far in the simulated conversation. Send " +
				"the whole state back with the next turn.",
			"completed_step_ids": "Ids of the procedure steps completed so far. Send the whole state " +
				"back with the next turn.",
			"pending": "Writes that wait on the customer's confirmation or a teammate's approval; " +
				"empty for none.",
			"procedure_id": "Id of the procedure the simulated conversation is running, such as " +
				"prc_.... Left out when it runs none.",
			"status": "Where the procedure stands: active, completed, handed_off or exited. Left out " +
				"when no procedure runs.",
			"step_outputs": "What the procedure's steps produced so far, as an object. Send the whole " +
				"state back with the next turn.",
		},
		"SkippedTool": {
			"reason": "Why the tool could not be turned into an action.",
		},
	}
}

/*
schemaTexts125 returns the texts from SkippedTool on.
*/
func schemaTexts125() map[string]map[string]string {
	return map[string]map[string]string{
		"SkippedTool": {
			"tool": "Name of the MCP server's tool that the sync skipped.",
		},
		"SlackChannel": {
			"external_id": "Slack's id of the workspace (team) the app is installed in; null until it " +
				"is connected.",
			"external_name": "Name of the Slack workspace the app is installed in; empty until it is " +
				"known.",
			"inbound_transport": "How Slack delivers events to dimmi: webhook or socket. Empty until " +
				"it is known.",
			"last_error": "Why the connection last failed; null when it is healthy.",
			"mode": "How the connection was made: custom for the workspace's own Slack app, shared for " +
				"the install's shared app. Empty until it is known.",
			"name":   "The connection's name in dimmi.",
			"status": "Where the connection stands: pending, active, error or disconnected.",
			"type":   "Kind of channel; slack for a Slack connection.",
		},
		"SlackPostable": {
			"is_private": "True for a private Slack channel.",
			"name":       "The Slack channel's name.",
		},
		"SlackRoom": {
			"channel_id":         "Id of the Slack connection that watches the channel, such as chn_....",
			"external_id":        "Slack's own id of the channel, such as C0123ABCD.",
			"is_shared_external": "True when the Slack channel is shared with another organization.",
			"last_error":         "Why watching the channel last failed; null when it is healthy.",
			"last_seen_event_at": "When an event from the channel last arrived; null if none has.",
			"mode": "Who counts as the customer in the channel: shared (the other organization's " +
				"people), community (everyone but linked teammates), helpdesk (the team's colleagues) or " +
				"internal (nobody).",
			"name": "The Slack channel's name.",
			"removed_at": "When the app was removed from the channel; null while it is still in it. A " +
				"removed room keeps its settings.",
			"settings": "The settings an admin chose for the channel, as an object; empty for none.",
			"status":   "Whether the channel is watched: active, paused, removed or error.",
		},
	}
}

/*
schemaTexts126 returns the texts from SlackRoom on.
*/
func schemaTexts126() map[string]map[string]string {
	return map[string]map[string]string{
		"SlackRoom": {
			"trigger": "When a message in the channel becomes a conversation: author_window, " +
				"every_message, emoji, mention, ai, auto or off.",
		},
		"SlackSetup": {
			"complete": "True once the connection is active, so its Slack app needs no more setting " +
				"up.",
			"create_app_url":   "Address that opens Slack's app creation with the manifest filled in.",
			"interactions_url": "dimmi's address Slack sends button clicks and other interactions to.",
			"manifest": "The Slack app manifest that sets the app up, as an object to paste into " +
				"Slack.",
			"request_url": "dimmi's address Slack delivers events to.",
		},
		"SnoozeChoice": {
			"at": "Time of day the snooze ends, as HH:MM such as 09:00, for the kinds day_at and " +
				"weekday_at. Left out otherwise.",
			"days_ahead": "How many days ahead the snooze ends, for the kind day_at. Left out " +
				"otherwise.",
			"key": "Stable name of the choice within its list. Send it to pick this choice.",
			"kind": "How the end is worked out: preset (a built-in preset), in (minutes from now), " +
				"day_at (days ahead at a time) or weekday_at (next such weekday at a time).",
			"label": "Name of the choice as people read it, such as Later today.",
			"minutes": "How many minutes from now the snooze ends, for the kind in. Left out " +
				"otherwise.",
			"preset": "Built-in preset the choice uses, for the kind preset: later_today, tomorrow, " +
				"next_week or one_week. Left out otherwise.",
			"until": "The moment this choice would end the snooze if picked now.",
			"weekday": "Weekday the snooze ends on, 1 for Monday to 7 for Sunday, for the kind " +
				"weekday_at. Left out otherwise.",
		},
		"SnoozeLists": {
			"defaults": "The built-in presets offered when no list is set: later today, tomorrow, next " +
				"week and in one week.",
		},
	}
}
