/*
|--------------------------------------------------------------------------
| Tool Catalog: Settings, Continued
|--------------------------------------------------------------------------
|
| The entries for identity verification and its secrets, redaction and
| retention, the messenger's settings and office hours.
|
*/

package mcpserver

import "maps"

/*
settingsDocs2 returns the entries for identity verification, redaction
and retention, the messenger's settings and office hours.
*/
func settingsDocs2() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		identityDocs(), identitySecretChangeDocs(), identityVerificationDocs(), redactionDocs(),
		redactionChangeDocs(), widgetDocs(), widgetChangeDocs(), scheduleDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
identityDocs returns the entries for listing, reading and creating
the secrets that sign visitors' identity tokens.
*/
func identityDocs() map[string]Doc {
	return map[string]Doc{
		"identity-secrets.index": {
			Title: "List identity secrets",
			Use: "Lists the keys the customer's own server signs messenger identity tokens with, revoked ones " +
				"included. Shows names, algorithms, fingerprints and status, never the secret values.",
			Returns: "every identity secret with its algorithm, key id, fingerprint, status and when it was last used.",
		},
		"identity-secrets.show": {
			Title:   "Get an identity secret",
			Use:     "Reads one identity secret's details and status. The secret value itself is never shown again.",
			Returns: "the secret's name, algorithm, key id, fingerprint, status and retirement or revocation dates.",
		},
		"identity-secrets.store": {
			Title: "Create an identity secret",
			Use: "Makes a new key for signing visitors' identity tokens. For HS256 dimmi generates the shared secret " +
				"and returns it once, in this answer only: hand it to the person to store, never repeat it elsewhere.",
			Returns: "the secret with, for HS256, its value and the environment variable names to keep it in.",
			Params: map[string]string{
				"algorithm":  "HS256 (a shared secret dimmi makes), or RS256, ES256 or EdDSA with a public key.",
				"public_key": "The public key as PEM text, for RS256, ES256 or EdDSA without jwks_url.",
				"jwks_url":   "An https address of an auth provider's key set, instead of a public key.",
				"issuer":     "The iss claim tokens must carry, at most 200 characters.",
				"audience":   "The aud claim tokens must carry, at most 200 characters.",
				"name":       "A label, at most 60 characters.",
			},
			Hint: "A workspace keeps a limited number of active secrets; retire or revoke an old one first.",
		},
	}
}

/*
identitySecretChangeDocs returns the entries for retiring,
revoking and rotating identity secrets.
*/
func identitySecretChangeDocs() map[string]Doc {
	return map[string]Doc{
		"identity-secrets.update": {
			Title: "Rename, retire or revoke an identity secret",
			Use: "Renames a secret, retires it so it keeps working until a date, or revokes it at once. Revoking is " +
				"final and, unless revoke_sessions is false, signs out every visitor it verified.",
			Returns: "the secret as it now is.",
			Params: map[string]string{
				"status": "retiring keeps it working until retires_at; revoked stops it now. A revoked secret " +
					"cannot change again.",
				"retires_at":      "RFC 3339 time in the next 30 days, with status retiring; defaults to a week out.",
				"reason":          "With status revoked: manual (default) or compromised.",
				"revoke_sessions": "With status revoked: false keeps the visitors it signed in; defaults to true.",
			},
		},
		"identity-secrets.rotations.store": {
			Title: "Rotate an identity secret",
			Use: "Makes a new secret with the same name and algorithm and retires the old one after a grace period, " +
				"so the customer's servers can switch without failed sign-ins. A new HS256 value is returned once.",
			Returns: "the new secret with, for HS256, its value and the environment variable names to keep it in.",
			Params: map[string]string{
				"public_key": "The new public key as PEM text, required for RS256, ES256 and EdDSA secrets.",
			},
			Hint: "Only an active secret rotates, and one using an auth provider's key set rotates at the provider.",
		},
	}
}

/*
identityVerificationDocs returns the entries for identity
verification settings, stats and token checks.
*/
func identityVerificationDocs() map[string]Doc {
	return map[string]Doc{
		"identity-verification.settings.show": {
			Title: "Get identity verification settings",
			Use: "Reads how strict the messenger is about who visitors are: off ignores tokens, optional (default) " +
				"proves signed-in customers and lets others chat anonymously, required admits only proven customers.",
			Returns: "the mode and the attributes the messenger may set from the page.",
		},
		"identity-verification.stats.show": {
			Title: "Get identity verification stats",
			Use: "Counts messenger sessions over recent days by outcome: verified, invalid token or anonymous. " +
				"Use it to see whether sign-in works before relying on it.",
			Returns: "the totals, the counts per day and the share of sessions that were verified.",
			Params: map[string]string{
				"days": "How many days back, up to 90; defaults to 7.",
			},
		},
		"identity-verification.validations.store": {
			Title: "Check an identity token",
			Use: "Explains what the messenger would make of a pasted identity token: valid or not, why, the likely " +
				"mistake and the contact it would sign in. A dry run: the token is never stored, logged or used.",
			Returns: "whether the token is valid, the reason and hint, its header and claims, and a contact preview.",
			Risk:    new(RiskRead),
			Params: map[string]string{
				"token":  "The identity token (a JWT) exactly as the customer's server made it.",
				"app_id": "The messenger's app id it is meant for, when the workspace has several.",
			},
		},
	}
}

/*
redactionDocs returns the entries for reading, switching and adding
redaction rules.
*/
func redactionDocs() map[string]Doc {
	return map[string]Doc{
		"redaction-rules.index": {
			Title: "List redaction rules",
			Use: "Reads what is masked out of messages before they are stored: the built-in rules for card numbers " +
				"and US social security numbers, then the workspace's own patterns in the order they run.",
			Returns: "the built-in switches and the workspace's rules in order.",
		},
		"redaction-rules.settings": {
			Title: "Update redaction settings",
			Use: "Turns the built-in redaction rules on or off and puts the workspace's own rules in a new order. " +
				"Applies to messages stored from now on; earlier messages are never rewritten.",
			Returns: "the built-in switches and the rules in their new order.",
			Params: map[string]string{
				"card_numbers": "true masks payment card numbers.",
				"us_ssn":       "true masks US social security numbers.",
				"order":        "Every rule id (rxr_...) exactly once, in the order they should run.",
			},
			Hint: "An order must name every rule; read them again with dimmi_redaction_rules_list.",
		},
		"redaction-rules.store": {
			Title: "Create a redaction rule",
			Use: "Adds a pattern whose matches are replaced in every message stored from now on, with no copy of " +
				"the original kept. It runs after the others. A workspace keeps at most 10 rules.",
			Returns: "the rule as saved.",
			Next:    []string{"dimmi_redaction_tests_create: try the rules on a sample text first"},
			Params: map[string]string{
				"pattern":     "An RE2 regular expression, at most 256 characters.",
				"replacement": "Text written in place of a match, at most 80 characters; defaults to [redacted].",
				"name":        "A label, 1 to 80 characters.",
				"enabled":     "Whether the rule runs; defaults to true.",
			},
			Hint: "A pattern must be a valid RE2 expression: no lookarounds or backreferences.",
		},
	}
}

/*
redactionChangeDocs returns the entries for changing and
trying redaction rules, and the retention preview.
*/
func redactionChangeDocs() map[string]Doc {
	return map[string]Doc{
		"redaction-rules.update": {
			Title: "Update a redaction rule",
			Use: "Changes a rule's name, pattern or replacement, or switches it off. Messages it already changed " +
				"stay as they were stored.",
			Returns: "the rule as it now is.",
			Params: map[string]string{
				"pattern":     "An RE2 regular expression, at most 256 characters.",
				"replacement": "Text written in place of a match, at most 80 characters.",
			},
		},
		"redaction-rules.destroy": {
			Title:   "Delete a redaction rule",
			Use:     "Removes a rule from the next message on. What it already replaced stays replaced.",
			Returns: "nothing once it is removed.",
		},
		"redaction-tests.store": {
			Title: "Try redaction on a text",
			Use: "Shows what the workspace's enabled redaction rules would make of a text, and which rules matched. " +
				"Nothing is stored and the text is never logged.",
			Returns: "the text as it would be stored and the rules that changed it.",
			Risk:    new(RiskRead),
			Params: map[string]string{
				"text": "The sample text, at most 65536 characters.",
			},
		},
		"retention-previews.store": {
			Title: "Preview conversation retention",
			Use: "Counts how many closed conversations a retention period would move to the trash today, before " +
				"the person sets it. Nothing changes.",
			Returns: "the count of conversations the period would move.",
			Risk:    new(RiskRead),
			Next:    []string{"dimmi_workspace_update: set the period under retention once the person agrees"},
			Params: map[string]string{
				"closed_conversation_days": "The period in days, from 30 to 3650.",
			},
		},
	}
}

/*
widgetDocs returns the entries for listing, reading, creating and
changing messengers.
*/
func widgetDocs() map[string]Doc {
	return map[string]Doc{
		"widget-settings.index": {
			Title:   "List messengers",
			Use:     "Lists the workspace's messengers, the chat widgets customers see on its sites, oldest first.",
			Returns: "every messenger with its settings, inbox, audiences and when it last loaded on a site.",
		},
		"widget-settings.show": {
			Title:   "Get a messenger",
			Use:     "Reads one messenger's settings: look, texts, allowed sites, email collection, tabs, audiences.",
			Returns: "the messenger with all its settings and its config version.",
		},
		"widget-settings.store": {
			Title: "Create a messenger",
			Use: "Adds a messenger with its own inbox connection and any settings sent. It shows on a site only once " +
				"its snippet is installed there.",
			Returns: "the new messenger with its settings.",
			Params:  widgetParams(),
		},
		"widget-settings.update": {
			Title: "Update a messenger",
			Use: "Changes what customers see and can do in a messenger on the sites it is installed on; live pages " +
				"pick it up within a minute. Only the fields sent change.",
			Returns: "the messenger as it now is.",
			Risk:    new(RiskReach),
			Next:    []string{"dimmi_widget_settings_preview: check the result before saving"},
			Params:  widgetParams(),
			Hint:    "A help tab needs help_center_id set, and only a paying cloud workspace may hide the badge.",
		},
	}
}

/*
widgetChangeDocs returns the entries for removing,
previewing and targeting a messenger.
*/
func widgetChangeDocs() map[string]Doc {
	return map[string]Doc{
		"widget-settings.destroy": {
			Title:   "Delete a messenger",
			Use:     "Removes a messenger. Sites that embed it stop showing it.",
			Returns: "nothing once it is removed.",
		},
		"widget-settings.preview": {
			Title: "Preview messenger changes",
			Use: "Builds the config a messenger would serve if the change sent were saved, checked like a save. " +
				"Nothing is written.",
			Returns: "the messenger's config as a browser would receive it, with the change applied.",
			Risk:    new(RiskRead),
			Params: map[string]string{
				"locale": "The language to show the texts in, such as en or fr.",
			},
		},
		"widget-settings.audiences.update": {
			Title: "Set who sees a messenger",
			Use: "Sets audience rules for who sees the launcher, who may start a conversation and who sees the " +
				"help and changelog tabs. A rule sent replaces that one, null removes it, one left out stays.",
			Returns: "the messenger's rules as stored.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"launcher": "An audience rule for who sees the launcher; null means everyone.",
				"start":    "An audience rule for who may start a conversation; null means everyone.",
				"tabs":     "A rule per tab, keyed help or changelog.",
			},
			Hint: "Rules use the audience rule language; dimmi_audience_fields_list lists the fields they can test.",
		},
	}
}

/*
widgetParams explains the settings a messenger is created and updated
with.
*/
func widgetParams() map[string]string {
	return map[string]string{
		"inbox_id": "Id of the inbox (inbox_...) its conversations land in.",
		"enabled":  "false hides the messenger on every site.",
		"appearance": "Keys merge one by one and null resets a key; for example theme light, dark or auto, " +
			"position right or left.",
		"reply_time": "The reply time customers are told: auto, minutes, hours or day.",
		"allowed_origins": "Sites that may load it, such as https://app.company.com; *.company.com covers one " +
			"label. An empty list allows any site.",
		"collect_email":     "When to ask for an email: never, offline or always.",
		"max_attachment_mb": "Largest file customers may attach, 1 to 25.",
		"help_center_id": "Id of the help center (hc_...) the help tab shows. Find it with " +
			"dimmi_help_centers_list.",
		"home_links": "Replaces the home screen links: each a title, an https or mailto url and an icon " +
			"(calendar, book or link).",
	}
}

/*
scheduleDocs returns the entries for office hours schedules.
*/
func scheduleDocs() map[string]Doc {
	return map[string]Doc{
		"schedules.index": {
			Title: "List office hours",
			Use: "Lists the workspace's office hours schedules, the default first. Workflows use them to check " +
				"whether the team is around and to wait until it is.",
			Returns: "every schedule with its timezone, weekly hours, holidays and whether it is open right now.",
		},
		"schedules.show": {
			Title:   "Get office hours",
			Use:     "Reads one schedule with its weekly hours and holidays.",
			Returns: "the schedule, whether it is open now and when it next opens or closes.",
		},
		"schedules.store": {
			Title:   "Create office hours",
			Use:     "Adds a named schedule of open hours per weekday in one timezone, with holidays closed all day.",
			Returns: "the schedule with whether it is open now.",
			Params:  scheduleParams(),
		},
		"schedules.update": {
			Title: "Update office hours",
			Use: "Changes a schedule's name, timezone, hours or holidays, or makes it the default. Waits already " +
				"counting on it keep their wake time; new ones use the new hours.",
			Returns: "the schedule as it now is.",
			Params:  scheduleParams(),
		},
		"schedules.destroy": {
			Title:   "Delete office hours",
			Use:     "Removes a schedule. The default schedule and one a workflow still names cannot be removed.",
			Returns: "nothing once it is removed.",
			Hint:    "Make another schedule the default, or change the workflows that name it, first.",
		},
	}
}

/*
scheduleParams explains the hours a schedule is saved with.
*/
func scheduleParams() map[string]string {
	return map[string]string{
		"timezone": "An IANA timezone, such as Europe/Paris.",
		"weekly": "Maps mon to sun to up to four [start, end] ranges as \"HH:MM\", such as " +
			"{\"mon\": [[\"09:00\", \"17:00\"]]}; 24:00 ends at midnight and a day left out is closed.",
		"holidays":   "Days closed all day, each a date as YYYY-MM-DD and an optional name; at most 100.",
		"is_default": "true makes it the schedule used wherever none is named.",
	}
}
