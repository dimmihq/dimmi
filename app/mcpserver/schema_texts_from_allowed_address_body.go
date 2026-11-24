/*
|--------------------------------------------------------------------------
| Written Schema Texts: AllowedAddressBody To Auth
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromAllowedAddressBody returns the texts from AllowedAddressBody on.
*/
func schemaTextsFromAllowedAddressBody() map[string]map[string]string {
	return map[string]map[string]string{
		"AllowedAddressBody": {
			"cidr": "An address range in CIDR form, or a single IP address, such as 203.0.113.0/24. Up " +
				"to 64 characters.",
			"label": "A name for this range, such as Office. Up to 60 characters.",
		},
		"Appearance": {
			"button_color": "Color of the messenger's buttons as #RRGGBB, such as #1f2328. Null when " +
				"not set.",
			"font":         "The messenger's font: system, or inherit to use the font of the page it sits on.",
			"header_color": "Color of the messenger's header as #RRGGBB. Null when not set.",
			"logo_asset_id": "Id of the uploaded image shown as the messenger's logo. Null when there " +
				"is no logo.",
			"padding_x": "How far the launcher sits from the page's edge sideways, in pixels, from 0 " +
				"to 200. Defaults to 20.",
			"padding_y": "How far the launcher sits from the page's edge up and down, in pixels, from " +
				"0 to 200. Defaults to 20.",
			"position":      "Which side of the page the launcher sits on: right or left.",
			"primary_color": "The messenger's main color as #RRGGBB. Defaults to #1f2328.",
			"reply_sound": "The sound the messenger plays when a reply arrives: none, pop, chime, bubble, " +
				"swoosh or tick. Defaults to pop.",
			"sent_sound": "The sound the messenger plays when the customer sends a message: none, pop, " +
				"chime, bubble, swoosh or tick. Defaults to none.",
			"sound": "Whether the messenger plays a sound. On by default.",
			"theme": "The messenger's color scheme: light, dark or auto.",
		},
		"Article": {
			"audience_id": "Id of the saved audience set on the article, such as audn_... Null when " +
				"none is set.",
			"body_markdown": "The article's working copy as Markdown. Left out in lists; read one " +
				"article to get it.",
			"collection_id": "Id of the collection the article sits in, such as col_... Null when it " +
				"is in none.",
			"description": "Short summary of the article. Null when none was written.",
			"has_unpublished_changes": "True when the article is published and its working copy has " +
				"edits that are not live yet.",
			"help_center_id": "Id of the help center the article belongs to, such as hc_...",
			"lock_version": "Counter that goes up with each save. Send the value you read when " +
				"updating so an edit made in between is not overwritten.",
		},
	}
}

/*
schemaTextsFromArticle returns the texts from Article on.
*/
func schemaTextsFromArticle() map[string]map[string]string {
	return map[string]map[string]string{
		"Article": {
			"published_at": "When the article was published. Null when it never was.",
			"slug": "The article's part of its web address: up to 120 lowercase letters, digits and " +
				"hyphens, such as reset-your-password.",
			"status": "Where the article stands: draft, published or archived.",
			"title":  "The article's title.",
			"visibility": "Who can read the article: public (anyone in the help center) or agent_only " +
				"(not shown to the public).",
			"word_count": "How many words the article's body has.",
		},
		"ArticleFeedNote": {
			"comment": "The note a reader left after saying the article did not help.",
		},
		"ArticleFeedback": {
			"comments": "How many readers left a note with their answer.",
			"helpful":  "How many readers said the article helped.",
			"recent": "The latest notes readers left after saying no, each with the contact when the " +
				"reader was signed in.",
			"unhelpful": "How many readers said the article did not help.",
		},
		"ArticleVersion": {
			"kind":   "Why the version was kept: published, snapshot, restored or suggestion.",
			"number": "The version's number within its article.",
			"title":  "The article's title as it was in this version.",
		},
		"AssignmentPoolMember": {
			"availability": "Whether the member is online or away.",
			"capacity": "The most open conversations the member may hold at once. Null when there is " +
				"no limit.",
			"channel_capacity": "The most open conversations of this channel type the member may hold. " +
				"Null when there is no limit for the channel.",
			"eligible":   "True when the member may be given the next conversation.",
			"open_count": "How many open conversations the member holds now.",
			"why_not": "Why the member cannot take the next conversation: away, at_capacity or " +
				"at_channel_capacity. Null when they can.",
		},
	}
}

/*
schemaTextsFromAssignmentStatus returns the texts from AssignmentStatus on.
*/
func schemaTextsFromAssignmentStatus() map[string]map[string]string {
	return map[string]map[string]string{
		"AssignmentStatus": {
			"channel_type": "The channel type the members were judged for, such as widget, email, " +
				"slack or whatsapp. Defaults to widget.",
			"members": "The members of the inbox's assignment pool, each with what they hold and " +
				"whether they may take the next conversation.",
			"mode": "How the inbox assigns new conversations: manual, round_robin or balanced.",
		},
		"AssignmentWait": {
			"reason": "Why the conversation waits for an assignee: no_available_member or " +
				"all_at_capacity.",
			"since": "When the conversation started waiting for an assignee.",
		},
		"AssignmentWaiting": {
			"count":     "How many conversations in the inbox wait for an assignee.",
			"oldest_at": "When the longest-waiting conversation started waiting. Null when none waits.",
			"reasons": "How many conversations wait for each reason, such as {\"no_available_member\": " +
				"2, \"all_at_capacity\": 1}. Empty when none waits.",
		},
		"Attachment": {
			"blocked": "True when the malware scan refused the file, so it cannot be downloaded.",
			"content_id": "The id an email's text uses to show this picture in place, written there as " +
				"cid:<content id>. Null for an ordinary attachment.",
			"content_type": "The file's media type, such as image/png or application/pdf.",
			"filename":     "The file's name, such as invoice.pdf.",
			"height": "Height of an image in pixels. Null when it is not known or the file is not an " +
				"image.",
			"scanned": "True when the malware scan found the file clean. A file with a url that is not " +
				"scanned was never checked.",
			"size_bytes": "The file's size in bytes.",
			"url": "A link that reads the file for a few minutes; fetch it again when it lapses. Null " +
				"until the malware scan lets the file through.",
			"width": "Width of an image in pixels. Null when it is not known or the file is not an " +
				"image.",
		},
	}
}

/*
schemaTextsFromAttribute returns the texts from Attribute on.
*/
func schemaTextsFromAttribute() map[string]map[string]string {
	return map[string]map[string]string{
		"Attribute": {
			"client_writable": "Whether browsers (the messenger on a customer's page) may write this " +
				"attribute. At most 20 attributes can be.",
			"data_type": "The type of the attribute's values: string, number, boolean, date or list. " +
				"Dates are kept as Unix seconds.",
			"description": "What the attribute is for, up to 500 characters. Null when none was " +
				"written.",
			"key": "The attribute's name in data: a lowercase letter then up to 63 lowercase letters, " +
				"digits or underscores, such as plan_tier.",
			"label": "The name people see for the attribute, 1 to 80 characters, such as Plan tier.",
			"last_value_at": "When a value was last written for this attribute. Null when none ever " +
				"was.",
			"of": "Whose attribute it is: contact or company.",
			"options": "The allowed values of a list attribute, as an array of up to 200 strings. An " +
				"empty array for other types.",
			"origin": "How the definition came to be: created by the team, or observed on the first " +
				"verified write of a new key.",
			"status": "active, or archived: an archived attribute keeps its values but takes no new " +
				"writes.",
		},
		"AttributeMap": {
			"display_name": "Name of the identity provider's attribute that carries a person's full " +
				"name.",
			"email": "Name of the identity provider's attribute that carries a person's email address.",
			"first_name": "Name of the identity provider's attribute that carries a person's first " +
				"name.",
			"groups":    "Name of the identity provider's attribute that carries a person's groups.",
			"last_name": "Name of the identity provider's attribute that carries a person's last name.",
		},
		"AttributeProblem": {
			"code": "Why the writes were dropped: type_mismatch, invalid_option, archived_write or " +
				"key_limit_reached.",
			"count": "How many writes were dropped for this key and reason.",
		},
	}
}

/*
schemaTextsFromAttributeProblem returns the texts from AttributeProblem on.
*/
func schemaTextsFromAttributeProblem() map[string]map[string]string {
	return map[string]map[string]string{
		"AttributeProblem": {
			"key":     "The attribute key the dropped writes were for, such as plan_tier.",
			"last_at": "When a write was last dropped for this key and reason.",
			"of":      "Whose attribute the writes were for: contact or company.",
			"sample":  "The shape of a dropped value, never the value itself. Null when none was kept.",
		},
		"Audience": {
			"description": "What the audience is for. Null when none was written.",
			"last_estimate": "The last preview count kept for the audience: an object with count, " +
				"pages, approximate and at (when it was counted). Null when never previewed.",
			"lock_version": "Counter that goes up with each save. Send the value you read when " +
				"updating so an edit made in between is not overwritten.",
			"name": "The name the team gave the audience.",
			"rules": "Who is in the audience: an object with visitors (everyone, identified, leads, " +
				"anonymous or anonymous_and_leads) and a rules tree of all/any/not groups over field, op and " +
				"value.",
			"used_by": "What names this audience, such as workflows, broadcasts, articles or other " +
				"audiences. Listed only when one audience is read; null in lists.",
		},
		"AudienceEvaluation": {
			"leaves": "Each condition of the rule with the value the person actually has, how far that " +
				"value is trusted and whether the condition held.",
			"result": "Whether the person is in the audience: match, no_match, or depends_on_page when " +
				"conditions about the page they are on decide it.",
		},
		"AudienceField": {
			"custom": "True when the field is one of the workspace's own custom attributes rather than " +
				"a built-in field.",
			"key": "The field's full name as used in a rule's field, such as contact.email or " +
				"contact.custom.plan_tier.",
			"label": "The name people see for the field. Left out when it has none.",
			"ops": "The operators a rule may use on this field, such as eq, neq, contains, exists, gt, " +
				"before or within_last_days.",
		},
	}
}

/*
schemaTextsFromAudienceField returns the texts from AudienceField on.
*/
func schemaTextsFromAudienceField() map[string]map[string]string {
	return map[string]map[string]string{
		"AudienceField": {
			"options": "The fixed values the field can take, for a field with a set list of values. " +
				"Left out otherwise.",
			"trust": "How far the field's value can be believed: verified, observed, claimed or by_key " +
				"(trusted only when the server recorded it from the team's own systems).",
			"type": "The type of the field's values: string, number, bool, date, list, set, url, id, " +
				"auto or saved_audience.",
			"where": "Where the field's value is known: server, client (the visitor's browser) or " +
				"conversation.",
			"window": "True for an event count, which a rule may narrow to the last days with " +
				"within_days. Left out otherwise.",
		},
		"AudiencePerson": {
			"email": "The person's email address. Null when not known.",
			"kind":  "Whether the contact is a user or a lead.",
			"name":  "The person's name. Null when not known.",
		},
		"AudiencePreview": {
			"approximate": "True when the counts were scaled up from a sample rather than counted in " +
				"full.",
			"count": "How many people the rule matches.",
			"pages": "How many more people match only on some pages, because the rule has conditions " +
				"about the page they are on.",
			"sample": "Some of the people who match. Empty for a caller who may not read contacts.",
			"total_contacts": "How many contacts the workspace has in all, to compare the count " +
				"against.",
			"warnings": "Rules that are allowed but probably wrong, each with a code such as " +
				"negative_or or impossible_and, a path and a message.",
		},
		"AudienceUser": {
			"live": "True when a live version of that thing runs on this audience.",
			"name": "The name of the thing that uses the audience.",
		},
	}
}

/*
schemaTextsFromAuditActor returns the texts from AuditActor on.
*/
func schemaTextsFromAuditActor() map[string]map[string]string {
	return map[string]map[string]string{
		"AuditActor": {
			"label": "The actor's name as it was when the entry was written, so it still reads right " +
				"after a member or token is gone.",
			"type": "The kind of actor: member for a person, system for work dimmi did on its own.",
		},
		"AuditLog": {
			"changes": "What changed, as a JSON object. An empty object when the event carries no " +
				"details.",
			"event": "The name of what happened.",
			"ip": "The IP address the change came from. Null when not known, as for work the system " +
				"did.",
			"request_id": "Id of the API request that made the change, for matching against logs. Null " +
				"when there was none.",
			"user_agent": "The browser or client that made the change. Null when not known.",
		},
		"AuditSubject": {
			"label": "The subject's name as it was when the entry was written. Null when it has none.",
			"type": "The kind of thing the change was about, such as member, team, invitation or " +
				"token.",
		},
		"AuditVia": {
			"kind": "The kind of credential the actor used: session, app, personal_token, oauth_client " +
				"or cli.",
		},
		"Auth": {
			"abilities": "The abilities the credential itself was given.",
			"auth_methods": "The ways the person signed in for this session, such as password, " +
				"magic_link, google, sso, totp or passkey. Left out for tokens.",
			"authenticated_at": "When the person signed in for this session, in UTC. Left out for " +
				"tokens.",
			"effective_abilities": "What the credential can actually do right now. Check this list, " +
				"not abilities, before trying something.",
			"session_id":      "Id of the session, such as ses_... Left out for tokens.",
			"two_factor_used": "True when the sign-in used a second factor. Left out for tokens.",
		},
	}
}
