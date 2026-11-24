/*
|--------------------------------------------------------------------------
| Written Schema Texts: EmailDomain To HelpCenterAnalytics
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromEmailDomain returns the texts from EmailDomain on.
*/
func schemaTextsFromEmailDomain() map[string]map[string]string {
	return map[string]map[string]string{
		"EmailDomain": {
			"last_checked_at": "When the domain's DNS records were last checked. Null when never " +
				"checked.",
			"mail_from_domain": "The subdomain used as the return path for bounces, such as " +
				"bounce.acme.com. Null when none is set up.",
			"next_check_at": "When the DNS records will be checked again. Null when no check is " +
				"planned.",
			"provider_hint": "The key of the DNS host the setup instructions are written for, such as " +
				"cloudflare, route53 or godaddy, or other when unknown.",
			"provider_trusted": "True when the install's operator listed this domain as already set up " +
				"at the mail service.",
			"records": "The DNS records to publish for the domain, each with what it is for, where it " +
				"goes, its value and what the last check found.",
			"status": "Where the domain stands: pending, verifying, verified, degraded (was verified, " +
				"now failing checks) or failed.",
			"verified_at": "When the domain was verified for sending. Null while it is not verified.",
			"warnings": "Mistakes the last check found in the domain's DNS, each with a code, a " +
				"message and, where it helps, the host and the fixing value. Empty when none.",
		},
		"EmailDomainLinks": {
			"checks": "The API address of the domain's DNS checks.",
			"self":   "The API address of this domain, such as https://.../api/v1/email-domains/edm_...",
			"zone_file": "The API address that returns the domain's records as a BIND zone file, to " +
				"import at a DNS host.",
		},
		"EmailSignature": {
			"body": "The signature's text, in short Markdown with links and emphasis only, up to 2000 " +
				"characters.",
			"email_address_id": "Id of the email address the signature is for, starting with ema_. " +
				"Null for one used from every address.",
			"enabled": "True when the signature is added to replies.",
			"kind": "Whose signature it is: personal for a teammate's own, address for the team's on " +
				"an address.",
			"member_id": "Id of the teammate who owns a personal signature, starting with mem_. Null " +
				"for the team's signature.",
		},
	}
}

/*
schemaTextsFromEmailSignatureRendering returns the texts from
EmailSignatureRendering on.
*/
func schemaTextsFromEmailSignatureRendering() map[string]map[string]string {
	return map[string]map[string]string{
		"EmailSignatureRendering": {
			"html": "The signature as HTML, as a reply shows it.",
			"text": "The signature as plain text, as a reply's text part shows it.",
		},
		"EmailSuppression": {
			"email": "The email address that is no longer mailed.",
			"expires_at": "When the suppression lapses on its own, as soft bounces do. Null when it " +
				"lasts until removed.",
			"reason": "Why the address is suppressed: hard_bounce, soft_bounce, complaint, " +
				"unsubscribed, manual (blocked by hand) or invalid.",
			"scope": "Which mail is stopped: all, or only notifications, changelog or feedback mail.",
			"source": "What reported it: ses, provider, dsn (a bounce message), arf (a complaint " +
				"report), one_click, link, api or manual.",
		},
		"EmailTemplateRendering": {
			"html": "A sample reply in the address's style, as HTML.",
			"text": "The same sample reply as plain text.",
		},
		"EmailVerification": {
			"email":       "The email address that was verified.",
			"status":      "Always verified.",
			"user_id":     "Id of the user account the address belongs to, starting with usr_.",
			"verified_at": "When the address was first proven, as a UTC time.",
		},
		"EventDefinition": {
			"client_writable": "True when the messenger may send this event from the visitor's " +
				"browser; false when only the team's servers may report it.",
			"description": "Text explaining what the event means, up to 500 characters. Null when none " +
				"was written.",
			"label":        "The event's display name, 1 to 80 characters, such as Exported a report.",
			"last_seen_at": "When an event of this name last arrived. Null when none has.",
			"name": "The event's name as it is sent: 1 to 64 lowercase letters, digits and _ . : -, " +
				"such as report.exported. Unique in the workspace.",
		},
	}
}

/*
schemaTextsFromEventDefinition returns the texts from EventDefinition on.
*/
func schemaTextsFromEventDefinition() map[string]map[string]string {
	return map[string]map[string]string{
		"EventDefinition": {
			"origin": "How the name came to be: created by the team, or observed on the first trusted " +
				"event of that name.",
			"status": "Whether the name takes events: active, or archived, which takes no new events " +
				"while existing summaries are kept.",
			"total_count": "How many events of this name have arrived in all.",
		},
		"EventSettings": {
			"messenger_new_events": "True when the messenger may send any event name from the browser, " +
				"new names included; false when it may send only names the team opened to it.",
		},
		"EventSummary": {
			"count":         "How many times this person did the event in all.",
			"first_at":      "When this person first did the event.",
			"label":         "The event's display name, such as Exported a report.",
			"last_30_days":  "How many times this person did the event in the last 30 days.",
			"last_7_days":   "How many times this person did the event in the last 7 days.",
			"last_at":       "When this person last did the event.",
			"last_metadata": "The details sent with this person's most recent event, as JSON.",
			"name":          "The event's name as it is sent, such as report.exported.",
		},
		"Factor": {
			"name": "The second factor's display name: Authenticator app, Passkey or Recovery codes.",
			"type": "The kind of second factor people may enroll: totp, passkey or recovery_code.",
		},
		"FeedbackBoard": {
			"approval_required": "True when requests customers post on this board wait for the team's " +
				"approval before showing.",
			"description": "Text describing the board, up to 500 characters. Empty when none was " +
				"written.",
			"help_center_id": "Id of the help center the board belongs to, starting with hc_.",
			"name":           "The board's name, 1 to 60 characters, such as Feature requests.",
			"position":       "The board's place in the team's order; lower comes first.",
		},
	}
}

/*
schemaTextsFromFeedbackBoard returns the texts from FeedbackBoard on.
*/
func schemaTextsFromFeedbackBoard() map[string]map[string]string {
	return map[string]map[string]string{
		"FeedbackBoard": {
			"post_count": "How many requests the board holds.",
			"slug": "The board's URL name: lowercase letters, digits and hyphens, such as " +
				"feature-requests.",
			"visibility": "Who can open the board: public (everyone), private (signed-in customers " +
				"only) or internal (the team only).",
		},
		"FeedbackComment": {
			"author_name": "The name of who wrote the comment: the teammate, or the customer's name or " +
				"email, or Customer when neither is known.",
			"body":      "The comment's text, 1 to 5000 characters.",
			"from_team": "True when a teammate wrote the comment, false when a customer did.",
			"internal":  "True for an internal note only the team sees, false for a public reply.",
			"parent_id": "Id of the comment this one replies to, starting with fbc_. Null for a " +
				"top-level comment; replies go one level deep.",
		},
		"FeedbackConversation": {
			"company":    "The name of the customer's company. Null when they have none.",
			"contact_id": "Id of the conversation's customer, starting with ctc_. Null when unknown.",
			"conversation_id": "Id of the linked conversation, starting with conv_. Send it wherever a " +
				"conversation is asked for.",
			"conversation_number": "The conversation's human reference number, such as 1042 for #1042.",
			"conversation_status": "Where the conversation stands: open, snoozed or closed.",
			"email":               "The customer's email address. Null when unknown.",
			"linked_at":           "When the conversation was linked to the request.",
			"linked_by_team":      "True when a teammate linked the conversation to the request.",
			"name":                "The customer's name. Null when unknown.",
			"subject":             "The conversation's subject line. Null when it has none.",
		},
		"FeedbackPost": {
			"author": "The name of who the request is from: the customer's name or email, else the " +
				"teammate who posted it, else Customer.",
			"author_contact_id": "Id of the customer the request is from, starting with ctc_. Null " +
				"when a teammate posted it for no particular customer.",
		},
	}
}

/*
schemaTextsFromFeedbackPost returns the texts from FeedbackPost on.
*/
func schemaTextsFromFeedbackPost() map[string]map[string]string {
	return map[string]map[string]string{
		"FeedbackPost": {
			"board_id": "Id of the board the request sits on, starting with fbd_. Null when it is on " +
				"no board.",
			"body_markdown": "The request's description, in Markdown. Empty when only a title was " +
				"given.",
			"changelog_entry_id": "Id of the changelog entry that announces this request as shipped, " +
				"starting with cle_. Null when none is linked.",
			"comment_count": "How many comments the request has.",
			"help_center_id": "Id of the help center whose portal the request is on, starting with " +
				"hc_.",
			"merged_into_id": "Id of the request this one was merged into, starting with fbp_; look " +
				"there instead. Null when it was not merged.",
			"moderation": "Whether the request is shown: approved, pending (waiting for the team's " +
				"approval) or spam.",
			"number": "The request's human reference number, such as 12 for #12. Unique within the " +
				"workspace.",
			"path": "The request's page path under its portal, such as /feedback/12-dark-mode.",
			"slug": "The request's URL name: lowercase letters, digits and hyphens, such as dark-mode.",
			"status": "The key of the workspace's status the request is in. Built-in ones are " +
				"under_review, planned, in_progress, shipped and closed; teams may add their own.",
			"status_changed_at": "When the request's status last changed. Null when it never has.",
			"title":             "The request's title, 1 to 150 characters.",
			"vote_count":        "How many votes the request has.",
		},
		"FeedbackPostsOutputBody": {
			"counts": "How many matching feature requests sit in each status, keyed by status key, " +
				"such as {\"planned\": 4, \"under_review\": 12}.",
			"data": "The feature requests on this page; an empty list when none match.",
			"links": "Optional URLs for this list, such as the next page. Left out when there are " +
				"none.",
		},
		"FeedbackStatus": {
			"category": "The stage of work this status stands for: open, planned, in_progress, done or " +
				"closed.",
			"color": "The status's color as a hex code, such as #3b82f6.",
		},
	}
}

/*
schemaTextsFromFeedbackStatus returns the texts from FeedbackStatus on.
*/
func schemaTextsFromFeedbackStatus() map[string]map[string]string {
	return map[string]map[string]string{
		"FeedbackStatus": {
			"name": "The status name readers see, such as Planned or Under consideration.",
			"roadmap": "True when the public roadmap shows this status as a column. Always false for a " +
				"status in the closed category.",
			"slug": "The status's name in URL form, such as in-progress.",
		},
		"FeedbackVoter": {
			"avatar_url": "Address of the voter's picture, or null when they have none.",
			"by_team": "True when a teammate added this vote on the customer's behalf, false when the " +
				"customer voted themselves.",
			"company": "Name of the voter's company, or null when none is known.",
			"contact_id": "Id of the customer who voted, starting with ctc_. Send it wherever a " +
				"customer is asked for.",
			"email": "The voter's email address, or null when none is known.",
			"name":  "The voter's name, or null when none is known.",
			"priority": "How much the voter wants this request: nice, important or must. A customer's " +
				"own vote is always nice.",
		},
		"GIFSearch": {
			"api_key": "The GIPHY key the browser uses to search GIFs directly. Null when GIF search " +
				"is off.",
			"enabled": "True when teammates may search GIFs: the instance has a GIPHY key and the " +
				"workspace's GIF picker is on.",
			"provider": "Where the GIFs come from, currently always giphy. Null when GIF search is " +
				"off.",
		},
		"Gradient": {
			"angle": "The gradient's direction in degrees, from 0 to 359, such as 135.",
			"from":  "The color the gradient starts with, as a hex code such as #10b981.",
			"to":    "The color the gradient ends with, as a hex code such as #3b82f6.",
		},
		"GroupRole": {
			"custom_role_id": "Id of the custom role the group's members get, starting with rol_. Left " +
				"out when the group maps to a built-in role.",
			"group": "Name of the identity provider group, as the provider sends it at sign-in.",
		},
	}
}

/*
schemaTextsFromGroupRole returns the texts from GroupRole on.
*/
func schemaTextsFromGroupRole() map[string]map[string]string {
	return map[string]map[string]string{
		"GroupRole": {
			"role": "The built-in role the group's members get. Left out when the group maps to a " +
				"custom role instead.",
		},
		"HelpCenter": {
			"ask_ai_enabled": "True when readers of this help center can ask the AI questions.",
			"custom_domain": "The help center's own verified domain, such as help.acme.com. Null until " +
				"one is set up and verified.",
			"indexing_enabled": "True when search engines are allowed to index this help center.",
			"is_default":       "True for the workspace's default help center.",
			"locale":           "The help center's main language as a language code, such as en or fr.",
			"name":             "The help center's name.",
			"status":           "Whether the help center is live: active or disabled.",
			"subdomain": "The part of the address picked for this help center: 3 to 40 lowercase " +
				"letters, digits and dashes, such as acme.",
			"url": "The full public address readers open, on the verified custom domain once there is " +
				"one.",
		},
		"HelpCenterAnalytics": {
			"content_gaps": "Search terms that found nothing, each with how often it was searched and " +
				"the last day; these point at articles worth writing.",
			"daily": "Article views for each day of the period, with the part that came from the " +
				"messenger.",
			"days":           "How many days the report covers.",
			"empty_searches": "How many searches in the period found no article.",
			"helpful":        "How many times readers marked an article helpful in the period.",
			"helpful_percent": "Share of helpful answers out of all helpful and unhelpful answers, a " +
				"whole number from 0 to 100. Null when nobody answered.",
			"searches": "How many searches readers ran in the period.",
			"since":    "The first day the report covers, written YYYY-MM-DD, such as 2026-01-31.",
			"top_articles": "The most read articles in the period, each with its views and helpful and " +
				"unhelpful counts.",
			"unhelpful": "How many times readers marked an article unhelpful in the period.",
			"views":     "Total article views in the period.",
		},
	}
}
