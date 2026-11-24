/*
|--------------------------------------------------------------------------
| Written Schema Texts: WebhookEndpoint To Workflow
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromWebhookEndpoint returns the texts from WebhookEndpoint on.
*/
func schemaTextsFromWebhookEndpoint() map[string]map[string]string {
	return map[string]map[string]string{
		"WebhookEndpoint": {
			"include_notes":   "True when the endpoint also receives events about internal notes.",
			"last_failure_at": "When a delivery to the endpoint last failed. Null when none has.",
			"last_success_at": "When a delivery to the endpoint last succeeded. Null when none has.",
			"metadata": "The caller's own labels kept on the endpoint, as an object of text keys and " +
				"text values. Empty when none are set.",
			"previous_secret_expires_at": "During a secret rotation, when the old signing secret stops " +
				"being used. Null when no rotation is under way.",
			"url": "The web address dimmi sends events to.",
		},
		"WebhookEndpointStats": {
			"attempts": "How many requests were sent to the endpoint over the last day.",
			"p50_ms":   "The median time the endpoint took to answer over the last day, in milliseconds.",
			"p95_ms": "The time within which 95% of the endpoint's answers came over the last day, in " +
				"milliseconds.",
			"succeeded": "How many of the last day's requests the endpoint accepted.",
		},
		"WebhookEventType": {
			"description": "A plain sentence saying when the event is sent, such as A contact was " +
				"added.",
			"group": "The area the event is listed under, such as Conversations, Messages, Contacts or " +
				"Tags.",
			"named_only": "True when the event is only delivered to endpoints that name it exactly; " +
				"wildcard subscriptions do not include it.",
			"notes_opt_in": "True when the event can be about an internal note, in which case only " +
				"endpoints that include notes receive it.",
			"object_type": "The kind of record the event's payload carries, such as conversation, " +
				"message or contact.",
			"type": "The event's dotted name to subscribe an endpoint with, such as " +
				"conversation.created or contact.updated.",
		},
		"WebhookSecret": {
			"endpoint_id": "Id of the webhook endpoint the secret belongs to, such as whk_....",
		},
	}
}

/*
schemaTextsFromWebhookSecret returns the texts from WebhookSecret on.
*/
func schemaTextsFromWebhookSecret() map[string]map[string]string {
	return map[string]map[string]string{
		"WebhookSecret": {
			"previous_expires_at": "When the previous secret stops signing deliveries. Null when no " +
				"rotation is under way.",
			"previous_secret": "The secret used before the last rotation, still valid while the " +
				"rotation overlaps. Null when there is none.",
			"secret": "The endpoint's current signing secret, used to check that a delivery came from " +
				"dimmi. Keep it private.",
		},
		"WhatsAppAccount": {
			"business": "Name of the Meta business that owns the WhatsApp account.",
			"name":     "The WhatsApp account's name as Meta gives it.",
			"numbers": "The phone numbers on the account, each saying whether it is already connected " +
				"here.",
		},
		"WhatsAppBindingBody": {
			"fallback": "Text used in place of the variable for a customer who lacks that detail, up " +
				"to 64 characters. Optional.",
			"key": "The customer detail the template variable is filled in with, as a dotted name such " +
				"as contact.first_name. At most 64 characters.",
		},
		"WhatsAppButton": {
			"phone_number": "The number a phone_number button calls. Left out for other button types.",
			"text":         "The button's label.",
			"type": "The kind of button, lowercase as Meta names it, such as quick_reply, url or " +
				"phone_number.",
			"url": "The address a url button opens. Left out for other button types.",
		},
		"WhatsAppChannel": {
			"coexistence":          "True when the number also stays in use on the WhatsApp Business app.",
			"display_phone_number": "The phone number as customers see it, as Meta formats it.",
			"inbox_id":             "Id of the inbox this number's conversations go to, such as inbox_....",
			"last_error":           "The latest problem with the connection, in words. Null when there is none.",
			"messaging_limit": "The number's messaging limit tier as Meta reports it. Empty when not " +
				"known.",
		},
	}
}

/*
schemaTextsFromWhatsAppChannel returns the texts from WhatsAppChannel on.
*/
func schemaTextsFromWhatsAppChannel() map[string]map[string]string {
	return map[string]map[string]string{
		"WhatsAppChannel": {
			"mode": "How the number is connected: shared (through this dimmi install's own Meta app) " +
				"or own (through the workspace's own Meta app).",
			"name": "The connection's name in dimmi.",
			"name_status": "Meta's review status of the display name, as Meta reports it. Empty when " +
				"not known.",
			"phone_number_id": "Meta's id of the phone number.",
			"quality_rating": "Meta's quality rating of the number, as Meta reports it. Empty when not " +
				"known.",
			"status":        "Where the connection stands: pending, active, error or disconnected.",
			"verified_name": "The business name customers see on WhatsApp, as verified by Meta.",
			"waba_id":       "Meta's id of the WhatsApp Business account the number belongs to.",
		},
		"WhatsAppContact": {
			"email": "The customer's email address, for filling template variables. Empty when " +
				"unknown.",
			"name":  "The customer's name, for filling template variables. Empty when unknown.",
			"phone": "The customer's phone number, for filling template variables. Empty when unknown.",
		},
		"WhatsAppContent": {
			"body":    "The template's main text in Markdown, with its variables in place.",
			"buttons": "The template's buttons. Empty when it has none.",
			"editable": "True when dimmi's editor can hold every part of the template: a text header " +
				"or none, and buttons it can send.",
			"footer": "The template's footer text. Empty when it has none.",
			"header": "The template's text header. Empty when it has none.",
			"samples": "The sample values Meta reviewed the variables with, keyed by part and " +
				"variable, such as {\"body.1\": \"Ada\"}.",
		},
		"WhatsAppConversation": {
			"channel_id": "Id of the WhatsApp connection the conversation is on, such as chn_....",
			"templates": "The connection's message templates, for writing to the customer once free " +
				"text can no longer go out.",
		},
	}
}

/*
schemaTextsFromWhatsAppHookup returns the texts from WhatsAppHookup on.
*/
func schemaTextsFromWhatsAppHookup() map[string]map[string]string {
	return map[string]map[string]string{
		"WhatsAppHookup": {
			"callback_url": "The address to enter in the workspace's own Meta app as its webhook " +
				"callback URL.",
			"verify_token": "The token to enter in the Meta app's webhook setup; Meta echoes it when " +
				"it checks the address.",
		},
		"WhatsAppNumber": {
			"connected":            "True when this number is already connected to this workspace.",
			"display_phone_number": "The phone number as customers see it, as Meta formats it.",
			"quality_rating":       "Meta's quality rating of the number, as Meta reports it.",
			"verified_name":        "The business name customers see on WhatsApp, as verified by Meta.",
		},
		"WhatsAppSignupConfig": {
			"app_id": "Id of the Meta app the signup popup opens with. Empty when signup is not " +
				"available.",
			"available": "True when this server is set up to open Meta's signup popup.",
			"config_id": "Id of the Meta Embedded Signup configuration the popup opens with. Empty " +
				"when signup is not available.",
			"graph_version": "The Meta Graph API version in use, such as v25.0.",
		},
		"WhatsAppTemplate": {
			"bindings": "The customer detail each variable is filled in with, keyed by part and " +
				"variable, such as {\"body.1\": {\"key\": \"contact.first_name\", \"fallback\": \"there\"}}.",
			"category": "Meta's category of the template: marketing, utility or authentication.",
			"language": "The template's language code as Meta names it, such as en_US.",
			"name": "The template's name at Meta. Send it with the language to pick this template for " +
				"a reply.",
			"preview":         "The template's text as the customer reads it, with no variables filled in.",
			"rejected_reason": "Why Meta rejected the template. Null when it was not rejected.",
			"sendable": "True when the inbox can send the template: approved by Meta, with a text " +
				"header or none, and buttons that need no values.",
			"status": "Meta's review status of the template in lowercase, such as approved, pending or " +
				"rejected.",
			"synced_at": "When the template was last copied from Meta.",
		},
	}
}

/*
schemaTextsFromWhatsAppTemplate returns the texts from WhatsAppTemplate on.
*/
func schemaTextsFromWhatsAppTemplate() map[string]map[string]string {
	return map[string]map[string]string{
		"WhatsAppTemplate": {
			"variables": "The values the template needs filled in before it is sent, each with the " +
				"part it is in and its key.",
		},
		"WhatsAppVariable": {
			"key":  "The variable's key within its part, a number or a name, such as 1 or first_name.",
			"part": "The part of the template the variable sits in, such as header or body.",
		},
		"WhatsAppWindow": {
			"expires_at": "Until when free text can go out, a day after the customer's last message. " +
				"Null when the customer never wrote.",
			"open": "True when free text can still be sent to the customer; false when only approved " +
				"templates can go out.",
		},
		"WidgetPreviewConfig": {
			"app_id":         "The messenger's public id that goes in the install snippet, such as wgt_....",
			"config_version": "A number that goes up each time the messenger's settings change.",
			"customer_status": "Whether the messenger shows customers the status of their " +
				"conversations; currently always true.",
			"delay_notice": "Whether the messenger tells customers when a reply is taking long; " +
				"currently always true.",
			"dir": "The reading direction of the chosen language: ltr, or rtl for languages such as " +
				"Arabic and Hebrew.",
			"home_links": "The customer's own links listed on the messenger's home screen, such as " +
				"booking a call.",
			"honeypot_field": "Name of a hidden form field real visitors never fill, used to catch " +
				"bots; currently website.",
			"locale":            "The language the messenger's texts are given in, such as en or fr.",
			"locales":           "Every language the messenger has texts for, as language codes.",
			"open_to":           "The screen the messenger opens on; currently always home.",
			"show_availability": "True when the messenger shows whether the team is available.",
			"show_team_avatars": "True when the messenger shows teammates' pictures.",
		},
	}
}

/*
schemaTextsFromWidgetPreviewConfig returns the texts from
WidgetPreviewConfig on.
*/
func schemaTextsFromWidgetPreviewConfig() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetPreviewConfig": {
			"tabs": "The tabs that are on, in order: home and messages always, then help and changelog " +
				"when turned on and a help center is linked.",
			"texts": "The messenger's words in the chosen language, keyed by text name, with the " +
				"workspace's own wording applied.",
		},
		"WidgetSettings": {
			"allow_anonymous":     "True when visitors who are not identified may use the messenger.",
			"allowed_origins":     "The sites allowed to load the messenger. Empty when no list is set.",
			"attachments_enabled": "True when customers can send files in the messenger.",
			"audiences": "Audience rules for who sees the launcher, who may start a conversation and " +
				"who sees each tab. An empty object means everyone.",
			"changelog_tab_enabled": "True when the messenger's What's new tab is turned on; it shows " +
				"only when a help center is linked.",
			"channel_connection_id": "Id of the channel connection the messenger feeds its inbox " +
				"through, such as chn_....",
			"collect_email": "When the messenger asks a visitor for their email: never, offline or " +
				"always.",
			"config_version": "A number that goes up each time the messenger's settings change.",
			"enabled":        "True when the messenger is turned on.",
			"help_center_id": "Id of the help center linked to the messenger, such as hc_.... Null " +
				"when none is linked.",
			"help_tab_enabled": "True when the messenger's help tab is turned on; it shows only when a " +
				"help center is linked.",
			"home_links": "The customer's own links listed on the messenger's home screen, such as " +
				"booking a call.",
			"intercom_shim": "True when the loader keeps an old Intercom snippet's calls working while " +
				"a customer moves over.",
			"last_boot_at": "When the messenger was last loaded on a site. Null when it never has " +
				"been.",
			"last_boot_origin": "The site the messenger was last loaded on. Null when it never has " +
				"been.",
			"locale":            "The messenger's language setting: a language code such as en, or auto.",
			"max_attachment_mb": "The largest file a customer can send, in megabytes, 1 to 25.",
			"name":              "The messenger's name in dimmi.",
			"native_apps_enabled": "True when requests without an Origin header, which native apps " +
				"send, are accepted despite the list of allowed sites.",
		},
	}
}

/*
schemaTextsFromWidgetSettings returns the texts from WidgetSettings on.
*/
func schemaTextsFromWidgetSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetSettings": {
			"reply_time": "The reply time the messenger promises: auto (follows team availability), " +
				"minutes, hours or day.",
			"require_email_when_offline": "True when a visitor must give an email to write while the " +
				"team is away.",
			"show_availability": "True when the messenger shows whether the team is available.",
			"show_branding": "Whether the messenger shows dimmi's branding: true, false, or null when " +
				"the messenger sets nothing of its own.",
			"show_team_avatars": "True when the messenger shows teammates' pictures.",
			"texts": "The messenger's own wording shared by every language, keyed by text name, with " +
				"defaults filled in.",
			"texts_by_locale": "Wording written for specific languages, keyed by language tag then " +
				"text name, such as {\"fr\": {\"greeting\": \"Bonjour\"}}.",
		},
		"WidgetSettingsLinks": {
			"config":  "Address of the public configuration a browser loads for this messenger.",
			"install": "API address that returns this messenger's install instructions.",
			"self":    "API address of this messenger's settings.",
		},
		"Workflow": {
			"archived": "True when the workflow has been archived (deleted).",
			"description": "A note saying what the workflow is for, up to 500 characters. Null when " +
				"none is set.",
			"draft_hash": "Fingerprint of the working copy; it changes whenever the working copy's " +
				"definition changes.",
			"has_unpublished_changes": "True when the working copy differs from what is live, or " +
				"nothing has been published yet.",
			"kind": "Whether the workflow talks to the customer or works behind the scenes: " +
				"customer_facing or background.",
			"live_version_id": "Id of the published version conversations run on, such as wfv_.... " +
				"Null when nothing is published.",
			"lock_version": "A number that goes up with each edit; it stops two editors from " +
				"overwriting each other.",
			"name":      "The workflow's name, 1 to 80 characters.",
			"paused_at": "When the workflow was paused. Null when it is not paused.",
		},
	}
}
