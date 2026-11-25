/*
|--------------------------------------------------------------------------
| Written Schema Texts: Auth To CatalogEntry
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromAuth returns the texts from Auth on.
*/
func schemaTextsFromAuth() map[string]map[string]string {
	return map[string]map[string]string{
		"Auth": {
			"type": "The kind of credential making the request: session, personal_access_token or " +
				"oauth_access_token.",
		},
		"AuthConfig": {
			"api_version": "The version of the API this server speaks.",
			"capabilities": "Optional sign-in features this server serves: magic_link and email_code " +
				"while mailed sign-in is on, otherwise empty.",
			"edition": "The kind of install: cloud or self_host.",
			"factors": "The second-factor types people may enroll, each with its type and name.",
			"locales": "The languages people may pick, the default first, each with its code and its " +
				"name in that language.",
			"magic_link":      "Whether signing in by a mailed link is switched on.",
			"mail_configured": "Whether the server can send email.",
			"min_client_versions": "The oldest app version this server accepts for each platform (ios, " +
				"android, cli). Null for a platform means any version.",
			"password_login":      "Whether signing in with a password is offered. Always true.",
			"password_min_length": "The fewest characters a password must have.",
			"providers": "The sign-in providers offered, such as Google, each with its key, name, " +
				"protocol and the path where its sign-in starts.",
			"region": "Where the install's data lives. \"self\" on a self-hosted install with no " +
				"region set.",
			"server_version": "The version of dimmi this server runs.",
			"setup_required": "Whether the install still needs first-time setup. The server currently " +
				"always answers false.",
			"signup_mode":   "Who may sign up: open or invite_only.",
			"sso_available": "Whether single sign-on is available on this server.",
		},
		"AuthFactor": {
			"codes": "The new recovery codes, shown once in the answer that made them. Left out " +
				"everywhere else.",
			"confirmed_at": "When the factor was confirmed and became active, in UTC. Null while it is " +
				"pending.",
			"last_used_at": "When the factor was last used to sign in, in UTC. Null when never.",
			"name":         "The name the person gave the factor, such as Work phone.",
		},
	}
}

/*
schemaTextsFromAuthFactor returns the texts from AuthFactor on.
*/
func schemaTextsFromAuthFactor() map[string]map[string]string {
	return map[string]map[string]string{
		"AuthFactor": {
			"preferred": "True when this is the factor asked for first at sign-in.",
			"remaining": "How many recovery codes are still unused. Present on recovery codes only.",
			"status":    "pending until the person confirms the factor, then active.",
			"total":     "How many recovery codes the set was made with. Present on recovery codes only.",
			"type":      "The kind of factor: totp (authenticator app), passkey or recovery_code.",
		},
		"Author": {
			"avatar_url": "Link to the author's picture. Null when they have none.",
			"name":       "The author's name as shown on the entry.",
			"type":       "Who wrote the entry: contact, member, agent (the AI), system or integration.",
		},
		"Background": {
			"fade": "Whether the background softens into the page below it.",
			"image_asset_id": "Id of the uploaded image used as the background. Left out unless type " +
				"is image.",
			"type": "What fills the messenger's home hero and header: solid, gradient or image.",
		},
		"Branding": {
			"locked": "True when the workspace may not hide the dimmi badge: on the cloud edition " +
				"until it has paid.",
			"show": "Whether the dimmi badge shows on the messenger.",
			"url":  "Where the badge links, such as https://trydimmi.com/?ref=<workspace>.",
		},
		"Broadcast": {
			"ai_replies": "Whether the AI agent may answer replies to this broadcast.",
			"reply_agent_id": "The AI agent that answers replies when ai_replies is true, such as aia_.... Null " +
				"leaves it to the agent placed on the channel the reply comes in on.",
			"audience_id": "Id of the saved audience the broadcast goes to, such as audn_... Null when " +
				"it uses its own rules or hand-picked Slack channels.",
			"body": "The message's text, up to 50000 characters.",
			"channel": "How the broadcast goes out: email, chat or post (both in the messenger; a post " +
				"shows as a card), slack, or conversation (inside each person's existing conversation).",
		},
	}
}

/*
schemaTextsFromBroadcast returns the texts from Broadcast on.
*/
func schemaTextsFromBroadcast() map[string]map[string]string {
	return map[string]map[string]string{
		"Broadcast": {
			"counts": "The latest tally of recipients as an object: total, queued, sent, delivered, " +
				"opened, clicked, replied, reactions, unsubscribed, converted, failed, bounced, skipped and " +
				"more. Empty before sending.",
			"email_address_id": "Id of the connected email address the broadcast is sent from, such as " +
				"ema_... Null when none is chosen.",
			"finished_at": "When sending finished. Null until then.",
			"goal": "What the broadcast hopes recipients do: an object with event (an event they send) " +
				"and days (within how many days). Null when no goal is set.",
			"holds": "Who a conversation broadcast leaves out: customer_waiting (still waiting on an " +
				"answer), team_replied (already answered since the tracker moved). Empty for none.",
			"name": "The team's name for the broadcast, 1 to 120 characters. Recipients do not see it.",
			"pause_reason": "Why sending was paused by itself: bounce_rate or complaint_rate. Null " +
				"when not paused for such a reason.",
			"preview": "The preview line an email shows after its subject, up to 200 characters. Empty " +
				"when none.",
			"reply_member_id": "Id of the teammate replies are assigned to, such as mem_... Null when " +
				"none is set.",
			"reply_team_id": "Id of the team replies are assigned to, such as team_... Null when none " +
				"is set.",
			"room_ids": "Ids of the Slack channels picked by hand to receive the broadcast, such as " +
				"chr_... Empty when none.",
			"rules": "The audience rule used when no saved audience is named, in the same shape as a " +
				"saved audience's rules. Null when none.",
			"schedule_kind": "When it sends: now, at (at scheduled_at) or local (at scheduled_at's " +
				"clock time in each person's own time zone).",
			"scheduled_at": "The time the broadcast is set to send. Null when schedule_kind is now.",
			"sender_id": "Id of the teammate the broadcast is sent as, such as mem_... Null when none " +
				"is chosen.",
			"service": "True for a service message, which also reaches people who unsubscribed or left " +
				"the topic and ignores the frequency cap.",
			"skip_frequency_cap": "True when the broadcast goes out even to people who recently got " +
				"another one.",
		},
	}
}

/*
schemaTextsFromBroadcastAndBroadcastCompany returns the texts from Broadcast on.
*/
func schemaTextsFromBroadcastAndBroadcastCompany() map[string]map[string]string {
	return map[string]map[string]string{
		"Broadcast": {
			"slack_as":   "Who a Slack broadcast posts as: bot or member (the sender).",
			"started_at": "When sending started. Null until then.",
			"status":     "Where the broadcast stands: draft, scheduled, sending, paused, sent or stopped.",
			"subject": "The subject line of an email, or the heading of a post. Up to 200 characters; " +
				"empty when none.",
			"ticket_id": "Id of the ticket a conversation broadcast was started from, such as tkt_...; " +
				"it reaches only conversations linked to it. Null otherwise.",
			"topic_id": "Id of the topic the broadcast is sent under, such as bctp_...; only people " +
				"subscribed to it get it. Null when it has no topic.",
			"track_clicks": "Whether clicks on the message's links are counted. On by default.",
		},
		"BroadcastCompany": {
			"clicked":    "How many of the company's recipients clicked a link.",
			"company_id": "Id of the company, such as cmp_... Empty for recipients with no company.",
			"delivered":  "How many of the company's recipients the message was delivered to.",
			"failed":     "How many sends to the company's recipients failed.",
			"name":       "The company's name. Empty when it has none.",
			"opened":     "How many of the company's recipients opened the message.",
			"reactions":  "How many reactions the company's recipients left.",
			"replied":    "How many of the company's recipients replied.",
			"total":      "How many recipients the broadcast had at this company.",
		},
		"BroadcastEstimate": {
			"capped": "True when counting stopped at the 50000-contact limit, so the real numbers may " +
				"be higher.",
			"reach": "How many people or Slack channels the broadcast would reach if sent now.",
			"skipped": "How many would be left out for each reason, such as {\"no_email\": 12, " +
				"\"unsubscribed\": 3}. Empty when nobody would be.",
		},
		"BroadcastLink": {
			"clicks": "How many times the link was clicked.",
			"n":      "The link's number within the broadcast.",
			"url":    "The address the link points to.",
		},
	}
}

/*
schemaTextsFromBroadcastPreferences returns the texts from
BroadcastPreferences on.
*/
func schemaTextsFromBroadcastPreferences() map[string]map[string]string {
	return map[string]map[string]string{
		"BroadcastPreferences": {
			"email":            "The email address these preferences belong to.",
			"topics":           "Each topic the person can choose, with whether they get it.",
			"unsubscribed_all": "True when the person left every broadcast from this workspace.",
			"workspace":        "The name of the workspace that sends the broadcasts.",
		},
		"BroadcastReason": {
			"count": "How many recipients this reason applies to.",
			"reason": "Why recipients were left out or wait, such as no_email, suppressed, " +
				"unsubscribed, not_subscribed, frequency_cap, duplicate_address or blocked.",
		},
		"BroadcastRecipient": {
			"address": "The email address the message went to. Empty for a recipient with no address, " +
				"or one who was left out.",
			"clicked_at":   "When the recipient first clicked a link. Null when they have not.",
			"company_id":   "Id of the recipient's company, such as cmp_... Null when there is none.",
			"company_name": "The company's name. Empty when there is none.",
			"contact_email": "The contact's email address. Empty when not known or the recipient is a " +
				"Slack channel.",
			"contact_id": "Id of the contact the message went to, such as ctc_... Null for a Slack " +
				"channel or a contact since removed.",
			"contact_name": "The contact's name. Empty when not known.",
			"conversation_id": "Id of the conversation the message or its reply lives in, such as " +
				"conv_... Null when there is none.",
			"converted_at": "When the recipient did what the broadcast's goal hoped for. Null when " +
				"they have not.",
			"delivered_at": "When the message was delivered. Null when it has not been.",
			"opened_at":    "When the recipient first opened the message. Null when they have not.",
			"reactions":    "How many reactions the recipient left on the message.",
			"reason": "Why the recipient was left out or the send failed, such as no_email, " +
				"suppressed, unsubscribed, frequency_cap or not_delivered. Null when nothing went wrong.",
			"replied_at": "When the recipient replied. Null when they have not.",
			"room_id": "Id of the Slack channel the message went to, such as chr_... Null when the " +
				"recipient is a person.",
		},
	}
}

/*
schemaTextsFromBroadcastRecipient returns the texts from BroadcastRecipient on.
*/
func schemaTextsFromBroadcastRecipient() map[string]map[string]string {
	return map[string]map[string]string{
		"BroadcastRecipient": {
			"room_name": "The Slack channel's name. Empty when the recipient is a person.",
			"sent_at":   "When the message was sent to this recipient. Null when it has not been.",
			"status": "What happened to this recipient: queued, sent, delivered, failed, bounced, " +
				"complained, suppressed, skipped or recalled.",
			"unsubscribed_at": "When the recipient unsubscribed from this message. Null when they have " +
				"not.",
		},
		"BroadcastReport": {
			"companies": "Results per company the broadcast reached: recipients, delivered, opened, " +
				"clicked, replied, reactions and failed. Empty when no recipient belongs to a company.",
			"links": "Each tracked link in the broadcast with its number, URL and click count. Empty " +
				"when the broadcast has no counted links.",
			"reasons": "Why recipients were skipped, suppressed, failed, bounced or complained, each " +
				"reason with how many people it covers, largest first.",
			"waiting": "Why recipients still queued are being held back from sending, each reason with " +
				"how many people it covers. Empty when nothing is held.",
		},
		"BroadcastTopic": {
			"broadcast_count": "How many broadcasts were sent under this topic.",
			"consent": "How people join the topic: opt_in reaches only those who said yes, opt_out " +
				"reaches everyone who has not said no.",
			"description": "The text explaining what the topic covers, shown to people choosing their " +
				"subscriptions. Empty when none was written.",
			"name":               "The topic's name, such as Product updates.",
			"subscribed_count":   "How many people the topic reaches now.",
			"unsubscribed_count": "How many people have left this topic.",
		},
		"BroadcastTopicPick": {
			"description": "The text explaining what the topic covers. Empty when none was written.",
			"name":        "The topic's name, such as Product updates.",
			"subscribed":  "True when this person currently receives broadcasts under the topic.",
		},
	}
}

/*
schemaTextsFromBulkChangesOutputBody returns the texts from
BulkChangesOutputBody on.
*/
func schemaTextsFromBulkChangesOutputBody() map[string]map[string]string {
	return map[string]map[string]string{
		"BulkChangesOutputBody": {
			"data": "One result per conversation, in the order the ids were sent: whether it went " +
				"through, the fields that changed, or the error code that stopped it.",
		},
		"BulkError": {
			"code": "The machine-readable code of the error that stopped this member's change, such as " +
				"not_found.",
			"message": "A sentence explaining why the change failed, in the caller's language when a " +
				"translation exists.",
		},
		"BulkMembersOutputBody": {
			"data": "One result per member id sent, in the order sent: its status and, when it failed, " +
				"the error.",
		},
		"BulkMeta": {
			"failed": "How many members' changes failed.",
			"skipped": "How many members were left alone because they already had the values asked " +
				"for.",
			"updated": "How many members were changed.",
		},
		"BulkResult": {
			"status": "What came of this member's change: updated, skipped (already had the values " +
				"asked for) or failed (see error).",
		},
		"Captcha": {
			"provider": "The captcha service asked for on sign-up and when mailing sign-in links. Only " +
				"turnstile is offered.",
			"site_key": "The public site key the browser uses to show the captcha challenge.",
		},
		"CatalogEntry": {
			"context_tokens": "The size of the model's context window, in tokens.",
			"display_name":   "The model's name as shown to people in settings.",
			"family":         "The model's family: anthropic, openai or openweight.",
		},
	}
}
