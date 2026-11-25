/*
|--------------------------------------------------------------------------
| Written Schema Texts: CatalogEntry To ConnectedApp
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromCatalogEntry returns the texts from CatalogEntry on.
*/
func schemaTextsFromCatalogEntry() map[string]map[string]string {
	return map[string]map[string]string{
		"CatalogEntry": {
			"suggested": "The jobs this model is suggested for: any of answer, classifier and " +
				"fallback.",
		},
		"ChangelogAnalytics": {
			"daily": "Views of the changelog's updates for each day of the period, as a date and a " +
				"count.",
			"days":                     "How many days the report covers, counting today; between 1 and 90.",
			"reactions":                "How many reactions readers left on updates over the period.",
			"since":                    "The first day the report covers, as a date such as 2026-11-01.",
			"subscribers":              "How many people subscribe to the changelog now.",
			"subscribers_joined":       "How many people subscribed during the period.",
			"subscribers_unsubscribed": "How many people unsubscribed during the period.",
			"top_entries": "The most read updates of the period, each with its id, title, slug, " +
				"publish time, views and reactions.",
			"views": "Total views of the changelog's updates over the period.",
		},
		"ChangelogDailyViews": {
			"date":  "The day counted, as a date such as 2026-11-01.",
			"views": "How many times updates were viewed that day.",
		},
		"ChangelogEntriesOutputBody": {
			"counts": "How many entries are in each status, keyed by status (draft, scheduled, " +
				"published), such as {\"draft\": 3, \"published\": 12}.",
			"data": "The entries on this page, without their bodies; read one entry to get " +
				"body_markdown.",
			"links": "Optional URLs for moving through the list, such as the next page. Left out when " +
				"there are none.",
		},
	}
}

/*
schemaTextsFromChangelogEntry returns the texts from ChangelogEntry on.
*/
func schemaTextsFromChangelogEntry() map[string]map[string]string {
	return map[string]map[string]string{
		"ChangelogEntry": {
			"authors": "The teammates shown as the entry's authors, each with member id, name and " +
				"avatar. While nobody has chosen authors, these are the editors.",
			"authors_chosen": "True when someone picked the entry's authors by hand; false while the " +
				"people who edited it stand as its authors.",
			"body_markdown": "The working copy of the entry's body, in Markdown. Present when one " +
				"entry is read; left out of lists.",
			"cover_url": "URL of the entry's cover image. Null when it has none.",
			"editors":   "The teammates who edited the entry, each with member id, name and avatar.",
			"has_unpublished_changes": "True when a published entry's working copy differs from what " +
				"readers see, so it needs publishing again. Always false for drafts and scheduled entries.",
			"help_center_id": "Id of the help center whose changelog holds this entry, starting with " +
				"hc_. Send it wherever a help center is asked for.",
			"labels": "The labels the entry is filed under, each with id, name, slug and color.",
			"lock_version": "A number that goes up with every save, starting at 1. Send the value you " +
				"read back when updating so a newer edit is not overwritten.",
			"notified_at": "When subscribers were mailed about this entry; an entry mails only once. " +
				"Null when no mailing has gone out.",
			"notify_subscribers": "True when subscribers are to be mailed when the entry is published.",
			"published_at":       "When the entry was published. Null while it has never been published.",
			"scheduled_for": "When a scheduled entry will publish itself. Null when no publish time is " +
				"set.",
			"slug":    "The entry's URL name within the changelog, such as dark-mode-is-here.",
			"status":  "Where the entry stands: draft, scheduled or published.",
			"summary": "A short summary of the entry. Null when none was written.",
			"title":   "The entry's title.",
		},
		"ChangelogLabel": {
			"color":    "The label's color as a six-digit hex code, such as #3b82f6.",
			"name":     "The label's name, 1 to 40 characters, such as Improvement.",
			"position": "The label's place in the changelog's label order; lower comes first.",
			"slug": "The label's URL name: lowercase letters, digits and hyphens, such as bug-fixes. " +
				"Unique within the help center.",
		},
	}
}

/*
schemaTextsFromChangelogPerson returns the texts from ChangelogPerson on.
*/
func schemaTextsFromChangelogPerson() map[string]map[string]string {
	return map[string]map[string]string{
		"ChangelogPerson": {
			"avatar_url": "URL of the teammate's avatar image. Null when they have none.",
			"member_id": "Id of the teammate, starting with mem_. Send it wherever a member is asked " +
				"for.",
			"name": "The teammate's display name.",
		},
		"ChangelogSubscriber": {
			"confirmed_at": "When the person confirmed their subscription. Null while unconfirmed.",
			"contact_id": "Id of the contact this subscriber is, starting with ctc_. Null when the " +
				"subscriber is not tied to a contact.",
			"email": "The email address updates are sent to.",
			"status": "Where the subscription stands: pending (awaiting email confirmation), active or " +
				"unsubscribed.",
			"unsubscribed_at": "When the person unsubscribed. Null while they have not.",
		},
		"ChangelogTag": {
			"color": "The label's color as a six-digit hex code, such as #3b82f6.",
			"name":  "The label's name, such as Improvement.",
			"slug":  "The label's URL name, such as bug-fixes.",
		},
		"Channel": {
			"name": "The name the team gave this channel connection.",
			"type": "The kind of channel: widget, email, api, slack, whatsapp, facebook or instagram.",
		},
		"Collection": {
			"audience_id": "Id of the audience allowed to see this collection, starting with audn_. " +
				"Null when it is not limited to an audience.",
			"description": "Text describing what the collection holds. Null when none was written.",
			"help_center_id": "Id of the help center the collection belongs to, starting with hc_. " +
				"Send it wherever a help center is asked for.",
			"icon": "The icon shown beside the collection. Null when none is set.",
			"name": "The collection's name as readers see it.",
			"parent_id": "Id of the collection this one sits inside, starting with col_. Null for a " +
				"top-level collection.",
		},
	}
}

/*
schemaTextsFromCollection returns the texts from Collection on.
*/
func schemaTextsFromCollection() map[string]map[string]string {
	return map[string]map[string]string{
		"Collection": {
			"position": "The collection's place among its siblings; lower comes first.",
			"slug":     "The collection's URL name within the help center, such as getting-started.",
		},
		"Company": {
			"custom_attributes": "The company's custom attributes as an object of names and values, " +
				"such as {\"plan\": \"pro\"}. An empty object when it has none.",
			"domain": "The company's web domain, such as acme.com, used to match it when it has no " +
				"external id. Null when unknown.",
			"external_id": "The company's id in your own system, unique within the workspace. Null " +
				"when none was given.",
			"name":  "The company's name.",
			"notes": "Free-text notes the team keeps about the company. Null when there are none.",
			"people_count": "How many contacts belong to the company. Given only in lists; absent when " +
				"one company is read.",
			"tags": "The tags on the company, each with its id and name.",
		},
		"CompanySummary": {
			"domain": "The company's web domain, such as acme.com. Null when unknown.",
			"name":   "The company's name.",
		},
		"Config": {
			"client_id": "OpenID Connect only: the client id dimmi signs in with at the identity " +
				"provider. Absent on SAML connections.",
			"idp_certificates": "SAML only: the identity provider's signing certificates, PEM encoded. " +
				"Absent on OpenID Connect connections.",
			"idp_entity_id": "SAML only: the identity provider's entity id. Absent on OpenID Connect " +
				"connections.",
			"idp_metadata_url": "SAML only: the URL of the identity provider's metadata, which its " +
				"settings are refreshed from. Absent when none was given.",
			"idp_sso_url": "SAML only: the identity provider's sign-in URL people are sent to. Absent " +
				"on OpenID Connect connections.",
			"issuer": "OpenID Connect only: the identity provider's issuer URL. Absent on SAML " +
				"connections.",
		},
	}
}

/*
schemaTextsFromConfig returns the texts from Config on.
*/
func schemaTextsFromConfig() map[string]map[string]string {
	return map[string]map[string]string{
		"Config": {
			"name_id_format": "SAML only: the NameID format asked of the identity provider. Absent " +
				"when none is set.",
			"scopes": "OpenID Connect only: the scopes asked for at sign-in. Absent when none are set.",
			"sp_certificate": "SAML only: dimmi's own service provider certificate, made with the " +
				"connection, to give to the identity provider.",
		},
		"ConfigAppearance": {
			"button_color": "The color of buttons as #RRGGBB, such as #1f2328. Falls back to " +
				"primary_color when no button color is set.",
			"button_ink": "The text color that reads on button_color: dark on a light color, light on " +
				"a dark one.",
			"font": "The messenger's font: system for the device's font or inherit to take the host " +
				"page's.",
			"header_color": "The color of the header as #RRGGBB. Falls back to primary_color when no " +
				"header color is set.",
			"header_ink": "The text color that reads on header_color: dark on a light color, light on " +
				"a dark one.",
			"padding_x": "How far the launcher sits from the side edge of the page, in pixels, 0 to " +
				"200. Defaults to 20.",
			"padding_y": "How far the launcher sits from the bottom edge of the page, in pixels, 0 to " +
				"200. Defaults to 20.",
			"position":      "The side of the page the launcher sits on: right or left.",
			"primary_color": "The messenger's brand color as #RRGGBB, such as #1f2328.",
			"reply_sound": "The sound to play when a reply arrives: none, pop, chime, bubble, swoosh or " +
				"tick.",
			"sent_sound": "The sound to play when the visitor sends a message: none, pop, chime, bubble, " +
				"swoosh or tick.",
			"sound": "True when the messenger plays a sound for new messages.",
			"theme": "The messenger's color theme: light, dark or auto to follow the visitor's device.",
		},
		"ConfigBackground": {
			"fade":      "True when the background softens into the page below it.",
			"image_url": "URL of the background image. Absent when there is none.",
			"type":      "What fills the home hero and header: solid, gradient or image.",
		},
	}
}

/*
schemaTextsFromConfigCompany returns the texts from ConfigCompany on.
*/
func schemaTextsFromConfigCompany() map[string]map[string]string {
	return map[string]map[string]string{
		"ConfigCompany": {
			"logo_url": "URL of the workspace's logo. Null when it has none.",
			"name":     "The workspace's name, which the messenger speaks for.",
		},
		"ConfigDevContext": {
			"console": "True when the messenger captures the browser's console output for the team.",
			"network": "True when the messenger captures the browser's network activity for the team.",
		},
		"ConfigFeatures": {
			"ai_agent": "True when the AI agent is available in the messenger.",
			"allow_anonymous": "True when visitors who are not signed in may use the messenger. Always " +
				"false when identity_mode is required.",
			"attachments": "True when visitors may attach files to messages.",
			"collect_email": "When the messenger asks a visitor for their email: never, offline (only " +
				"when the team is away) or always.",
			"help_tab":      "True when the help tab is on and the messenger has a help center to show.",
			"identity_mode": "Whether the host app must sign visitors in: off, optional or required.",
			"intercom_shim": "True when the messenger also answers Intercom's JavaScript calls, for " +
				"sites moving from Intercom.",
			"max_attachment_mb": "The largest file a visitor may attach, in megabytes.",
			"opener": "True when a workflow greets visitors as they open a chat, instead of the " +
				"conversation starting from their first message.",
			"require_email_when_offline": "True when a visitor must give an email to write while the " +
				"team is away.",
		},
		"ConfigGradient": {
			"angle": "The gradient's direction, in degrees.",
			"from":  "The gradient's starting color as #RRGGBB.",
			"to":    "The gradient's ending color as #RRGGBB.",
		},
		"ConfigHomeLink": {
			"icon": "The icon shown beside the link: link, calendar, book, mail, phone, chat, video or " +
				"help.",
			"title": "The link's text on the messenger's home screen.",
		},
	}
}

/*
schemaTextsFromConfigHomeLink returns the texts from ConfigHomeLink on.
*/
func schemaTextsFromConfigHomeLink() map[string]map[string]string {
	return map[string]map[string]string{
		"ConfigHomeLink": {
			"url": "The address the link opens.",
		},
		"ConfigLauncher": {
			"icon":      "The icon on the launcher: chat, help or wave.",
			"image_url": "URL of a custom launcher image. Null when the launcher uses its icon.",
			"label":     "The text on the launcher, up to 24 characters. Empty when it shows none.",
			"shape":     "The launcher's shape: circle or squircle.",
			"style":     "How the launcher shows: bubble, tab or hidden.",
		},
		"ConfigPrivacy": {
			"enabled": "True when a privacy notice shows above the composer.",
			"url":     "The address of the privacy policy the notice links to. Null when none is set.",
		},
		"ConfigRealtime": {
			"polling_interval_ms": "How often, in milliseconds, a browser should poll for updates when " +
				"it cannot hold a live stream.",
			"token_url": "The address the browser asks for a token to open the live stream.",
			"url":       "The address of the live update stream.",
		},
		"ConfigSurface": {
			"dynamic": "True when the messenger's rules depend on page conditions the browser must " +
				"check.",
			"version": "The messenger's config version these rules were read at; it goes up when the " +
				"messenger's settings change.",
		},
		"Confirmation": {
			"confirmed_until": "Until when sensitive steps need no new proof of who the caller is, as " +
				"a UTC time. After it, confirm again.",
			"method": "How the caller last proved who they are: password, or email_code for an account " +
				"without a password.",
		},
		"ConnectedApp": {
			"client_id": "The OAuth client id of the app the person connected.",
		},
	}
}
