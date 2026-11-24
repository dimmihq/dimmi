/*
|--------------------------------------------------------------------------
| Written Schema Texts: SLAPolicy To SeriesStep
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromSLAPolicy returns the texts from SLAPolicy on.
*/
func schemaTextsFromSLAPolicy() map[string]map[string]string {
	return map[string]map[string]string{
		"SLAPolicy": {
			"resolution_seconds": "Seconds allowed to resolve the work; null when the policy sets no " +
				"such target.",
			"schedule_id": "Id of the schedule whose open hours the targets count in, such as sch_...; " +
				"null when the policy has none.",
			"ticket_type_ids": "Ids of the ticket types the policy applies to new tickets of, such as " +
				"tktt_...; empty when it names none.",
			"warn_before_seconds": "How many seconds before a target is due the team is warned.",
		},
		"SSOAttributeMap": {
			"display_name": "Name of the identity provider attribute that carries a person's full " +
				"name. Left out when not set.",
			"email": "Name of the identity provider attribute that carries a person's email address. " +
				"Left out when not set.",
			"first_name": "Name of the identity provider attribute that carries a person's first name. " +
				"Left out when not set.",
			"groups": "Name of the identity provider attribute that carries a person's groups. Left " +
				"out when not set.",
			"last_name": "Name of the identity provider attribute that carries a person's last name. " +
				"Left out when not set.",
		},
		"SSOConfig": {
			"client_id":        "OpenID Connect only: the client id the identity provider issued for dimmi.",
			"idp_certificates": "SAML only: the identity provider's signing certificates, up to 5.",
			"idp_entity_id":    "SAML only: the identity provider's entity id.",
			"idp_metadata_url": "SAML only: address of the identity provider's metadata, which its " +
				"settings are refreshed from.",
			"idp_sso_url": "SAML only: the identity provider's address that sign-ins are sent to.",
			"issuer": "OpenID Connect only: the identity provider's issuer address, such as " +
				"https://login.example.com.",
			"name_id_format": "SAML only: the name id format asked for, such as " +
				"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress.",
			"scopes": "OpenID Connect only: the scopes asked for at sign-in, such as openid, email and " +
				"profile.",
		},
	}
}

/*
schemaTextsFromSSOConnection returns the texts from SSOConnection on.
*/
func schemaTextsFromSSOConnection() map[string]map[string]string {
	return map[string]map[string]string{
		"SSOConnection": {
			"default_custom_role_id": "Id of the custom role given to people who match no group, such " +
				"as rol_...; null to use default_role alone.",
			"default_role": "Built-in role given to people who match no group: admin or member.",
			"default_team_ids": "Ids of the teams people created at sign-in are added to, such as " +
				"team_...; empty for none.",
			"enforced": "True when the company requires its people to sign in through this connection. " +
				"Only an active connection may be enforced.",
			"group_role_map": "Which identity provider groups get which role; empty when none are " +
				"mapped.",
			"has_secret": "True when a signing key or client secret is stored for the connection. The " +
				"secret itself is never returned.",
			"jit_enabled": "True when just-in-time provisioning is turned on for the connection.",
			"last_error": "The last failed sign-in through the connection, as an object with a code " +
				"and the time it happened (at); null when none is recorded.",
			"last_login_at": "When someone last signed in through the connection; null if nobody has.",
			"metadata_error": "Why the last refresh from the identity provider's metadata address " +
				"failed; null when it worked or none was tried.",
			"name":     "The connection's name, shown to people signing in.",
			"protocol": "How the connection signs people in: saml or oidc.",
			"status": "Where the connection stands: draft, tested, active or disabled. Changing its " +
				"protocol settings sends it back to draft.",
			"tested_at": "When a test sign-in last succeeded; null if none has.",
		},
		"SSOConnectionTest": {
			"url": "Address to open in the browser to sign in at the identity provider and test the " +
				"connection.",
		},
		"SSOGroupRole": {
			"custom_role_id": "Id of the custom role the group's members get, such as rol_.... Left " +
				"out when the group gets a built-in role only.",
			"group": "Name of the identity provider group, as the provider sends it.",
			"role": "Built-in role the group's members get: admin or member. May be left out when a " +
				"custom role is given.",
		},
	}
}

/*
schemaTextsFromSSOLookup returns the texts from SSOLookup on.
*/
func schemaTextsFromSSOLookup() map[string]map[string]string {
	return map[string]map[string]string{
		"SSOLookup": {
			"enforced": "True when the company requires this email address to sign in through single " +
				"sign-on.",
			"name": "Name of the single sign-on connection the email address signs in through; null " +
				"when there is none.",
			"redirect_url": "Address to send the browser to so the person signs in at their identity " +
				"provider; null when the email address has no connection.",
		},
		"SavedReply": {
			"body": "The text of the reply.",
			"owner_id": "Id of the teammate who owns a personal reply, such as mem_...; null for a " +
				"reply shared with the workspace.",
			"shared": "True when the whole workspace can use the reply, false when it is the owner's " +
				"own.",
			"shortcut": "Short word typed to insert the reply, stored without a leading # or /, such " +
				"as refund; null when it has none.",
			"title": "The reply's name in the list of saved replies.",
		},
		"SavedReplyActions": {
			"assignee_id": "Id of the teammate the conversation is assigned to when the reply is sent, " +
				"such as mem_.... Left out when it assigns nobody.",
			"priority": "Priority set when the reply is sent: none, low, medium, high or urgent. Left " +
				"out when it sets none.",
			"snooze_preset": "How long to snooze when status is snoozed: later_today, tomorrow, " +
				"next_week or one_week. Left out otherwise.",
			"status": "Status the conversation moves to when the reply is sent: open, closed or " +
				"snoozed. Left out when it changes none.",
			"tag_ids": "Ids of the tags added to the conversation when the reply is sent, such as " +
				"tag_...; empty for none.",
			"team_id": "Id of the team the conversation is assigned to when the reply is sent, such as " +
				"team_.... Left out when it assigns none.",
		},
		"Schedule": {
			"holidays": "Days closed all day, each an object with date (such as 2026-12-25), an " +
				"optional name, and yearly true when it repeats every year.",
		},
	}
}

/*
schemaTextsFromSchedule returns the texts from Schedule on.
*/
func schemaTextsFromSchedule() map[string]map[string]string {
	return map[string]map[string]string{
		"Schedule": {
			"is_default": "True for the workspace's default schedule.",
			"name":       "The schedule's name.",
			"timezone":   "Timezone the hours are read in, such as Europe/Paris.",
			"weekly": "Open hours by weekday, mon to sun, each a list of up to 4 [start, end] ranges " +
				"as HH:MM, such as {\"mon\": [[\"09:00\", \"17:00\"]]}; 24:00 ends at midnight.",
		},
		"ScheduleStatus": {
			"next_close": "When the schedule next closes, while it is open; otherwise null.",
			"next_open": "When the schedule next opens, while it is closed; null while it is open or " +
				"when it never opens.",
			"open": "True when the schedule is open at the moment it was read.",
		},
		"SearchHit": {
			"highlight": "Excerpt of the matching message with <mark> and </mark> around the matched " +
				"words; null when the conversation matched on something other than a message.",
			"message_id": "Id of the message that matched, such as msg_...; null when the conversation " +
				"matched on something other than a message.",
		},
		"SearchResults": {
			"companies": "Companies the search found. Always an empty list for now.",
			"contacts":  "Contacts the search found; empty for none.",
			"conversations": "Conversations the search found, each with its contact and an excerpt of " +
				"the matching message; empty for none.",
		},
		"SecurityEvent": {
			"country": "Country the event came from; null when unknown.",
			"device_label": "Readable name of the device or browser the event came from; null when " +
				"unknown.",
			"ip": "IP address the event came from; null when unknown.",
			"metadata": "Further details of the event, as an object whose keys depend on the type; " +
				"empty when there are none.",
			"type": "What happened, as a short code naming the kind of security event.",
		},
	}
}

/*
schemaTextsFromSecurityEvent returns the texts from SecurityEvent on.
*/
func schemaTextsFromSecurityEvent() map[string]map[string]string {
	return map[string]map[string]string{
		"SecurityEvent": {
			"user_agent": "The browser's user agent string at the time; null when unknown.",
		},
		"SendingIdentity": {
			"domain_status": "Where the workspace's own sending domain stands for this address: " +
				"verified, degraded or unverified.",
			"from":     "The From line replies leave with, such as \"Acme Support\" <support@acme.com>.",
			"provider": "Name of the mail provider this dimmi install sends customer mail through.",
			"uses_default_sender": "True when replies leave from dimmi's default sender, false when a " +
				"verified domain of the workspace's own sends them.",
		},
		"Series": {
			"ai_replies": "True when the AI agent may answer replies to the series' messages.",
			"archived_at": "When the series was archived; null if it has not been. An archived series " +
				"can no longer change.",
			"counts":      "Counts stored on the series, as a JSON object; empty when none are stored.",
			"description": "What the series is for, up to 500 characters; empty when none was written.",
			"email_address_id": "Id of the email address the series' emails are sent from, such as " +
				"ema_...; null when none is chosen.",
			"has_unpublished_changes": "True when the working copy differs from the version that runs, " +
				"or nothing was published yet.",
			"live_version_id": "Id of the published version that runs, such as srsv_...; null when the " +
				"series was never published.",
			"lock_version": "Number that goes up on every save. Send it back when saving so a save " +
				"made against an older copy is refused.",
			"name":      "The series' name, up to 120 characters.",
			"paused_at": "When the series was paused; null while it is not paused.",
			"problems": "What the working copy still misses before it can go live, each with a code, a " +
				"message and the step and field it is about; empty when nothing is missing.",
			"published_at": "When the series was last published; null if it never was.",
			"reply_member_id": "Id of the teammate replies to the series are assigned to, such as " +
				"mem_...; null for none.",
		},
	}
}

/*
schemaTextsFromSeries returns the texts from Series on.
*/
func schemaTextsFromSeries() map[string]map[string]string {
	return map[string]map[string]string{
		"Series": {
			"reply_team_id": "Id of the team replies to the series are assigned to, such as team_...; " +
				"null for none.",
			"sender_id": "Id of the teammate the series' messages are sent as, such as mem_...; null " +
				"for none.",
			"service": "True when the series sends service notices, such as an outage or a security " +
				"notice, rather than marketing.",
			"slack_as": "Who the series' Slack messages are posted as: bot or member.",
			"spec": "The working copy of the series' definition, its steps and the paths joining them, " +
				"as the same JSON the builder saves.",
			"status":  "Where the series stands: draft, live, paused or archived.",
			"subject": "Who travels the series: contact or company.",
			"topic_id": "Id of the subscription topic the series sends under, such as bctp_...; null " +
				"for none.",
		},
		"SeriesCount": {
			"count": "How many journeys ended for this reason.",
			"reason": "Why journeys ended, a short lowercase code such as series_stopped or " +
				"node_removed.",
		},
		"SeriesDay": {
			"day":     "The day these numbers are for, as a date and time at the start of the day.",
			"entered": "How many travelers entered the series that day.",
			"goal":    "How many travelers reached the goal that day.",
		},
		"SeriesEnrollment": {
			"company_id": "Id of the company traveling the series, such as cmp_..., for a series of " +
				"companies; null for a series of contacts.",
			"company_name": "Name of the company traveling. Only present when journeys are listed with " +
				"who travels them, and left out when empty.",
			"contact_email": "Email of the contact traveling. Only present when journeys are listed " +
				"with who travels them, and left out when empty.",
			"contact_id": "Id of the contact traveling the series, such as ctc_..., for a series of " +
				"contacts; null for a series of companies.",
		},
	}
}

/*
schemaTextsFromSeriesEnrollment returns the texts from SeriesEnrollment on.
*/
func schemaTextsFromSeriesEnrollment() map[string]map[string]string {
	return map[string]map[string]string{
		"SeriesEnrollment": {
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
			"test":        "True for a test journey, which never counts in the series' report.",
			"wake_at":     "When a waiting journey moves on; null when it is not waiting on a time.",
		},
		"SeriesProblems": {
			"problems": "What the series misses before it can go live, each with a code, a message and " +
				"the step and field it is about; empty when it is ready.",
		},
		"SeriesReport": {
			"active":   "How many travelers are on the series now.",
			"days":     "Entries and goals reached per day over the period of the report.",
			"entered":  "How many travelers entered the series.",
			"exited":   "How many travelers left the series before finishing.",
			"finished": "How many travelers reached the end of the series.",
			"goal":     "How many travelers reached the series' goal.",
			"reasons":  "Why travelers left, each reason with how many.",
			"steps":    "Each step's numbers, keyed by the step's id in the series' definition.",
		},
		"SeriesStep": {
			"detail": "Further details of what happened at the step, as a JSON object whose keys " +
				"depend on the kind of step; empty when there are none.",
		},
	}
}
