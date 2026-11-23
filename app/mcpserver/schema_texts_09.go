/*
|--------------------------------------------------------------------------
| Written Schema Texts: HelpCenterDomain To InboundEmail
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts57 returns the texts from HelpCenterDomain on.
*/
func schemaTexts57() map[string]map[string]string {
	return map[string]map[string]string{
		"HelpCenterDomain": {
			"failure_reason": "Why the last DNS check did not pass, in words. Null when no check has " +
				"failed.",
			"help_center_id": "Id of the help center this domain belongs to, starting with hc_. Send " +
				"it wherever a help center is asked for.",
			"hostname":        "The custom domain readers will use, such as help.acme.com.",
			"last_checked_at": "When the DNS records were last checked. Null before the first check.",
			"next_check_at": "When the DNS records will be checked again. Null when no further check " +
				"is planned, as after checks gave up.",
			"records": "The two DNS records the team creates at its DNS provider: one CNAME and one " +
				"TXT, each with its name and value.",
			"status": "Where the domain's DNS check stands: pending, verified or failed. The help " +
				"center is served on it only once verified.",
			"verified_at": "When the domain passed its DNS check. Null until it is verified.",
		},
		"HelpCenterSettings": {
			"appearance": "The color scheme of the help center's pages: auto, light or dark. Left out " +
				"when never set.",
			"changelog": "Whether the help center shows a changelog. Left out when never set, which " +
				"means yes.",
			"changelog_authors": "Whether changelog entries show who wrote them. Left out when never " +
				"set, which means yes.",
			"changelog_email": "What subscribers are mailed for a new changelog entry: full for the " +
				"whole entry or summary for a summary. Left out when never set, which means full.",
			"changelog_reactions": "Whether readers can react to changelog entries. Left out when " +
				"never set, which means yes.",
			"changelog_subscriptions": "Whether readers can subscribe to changelog emails. Left out " +
				"when never set, which means yes.",
			"changelog_title": "The heading of the changelog pages. Left out when never set, and the " +
				"pages then say Changelog.",
			"changelog_undo_minutes": "Minutes to wait after publishing a changelog entry before " +
				"mailing it: 0, 5, 15 or 60. Left out when never set, which means no wait.",
			"contact_url": "The https address the help center's contact link opens. Left out when not " +
				"set.",
		},
	}
}

/*
schemaTexts58 returns the texts from HelpCenterSettings on.
*/
func schemaTexts58() map[string]map[string]string {
	return map[string]map[string]string{
		"HelpCenterSettings": {
			"email_sign_in": "Whether readers may sign in with a code mailed to them. Left out when " +
				"never set, which means yes.",
			"feedback_approval": "True when feature requests posted by customers wait for the team's " +
				"approval before showing. Left out when false.",
			"feedback_board": "True when customers can see and post feature requests on the help " +
				"center. Left out when false.",
			"footer_text":   "The text shown in the help center's footer. Left out when not set.",
			"home_subtitle": "The smaller line under the home page's title. Left out when not set.",
			"home_title":    "The big title on the help center's home page. Left out when not set.",
			"login_url": "The https address in the team's product where readers sign in, so they can " +
				"read articles kept to an audience. Left out when not set.",
			"messenger": "Whether the help center's pages show a messenger. Left out when never set, " +
				"which means yes.",
			"messenger_id": "Id of the messenger the pages show, starting with wgt_. Left out when not " +
				"set, and the workspace's first messenger is used.",
			"primary_color": "The help center's main color as a hex code, such as #10b981. Left out " +
				"when not set.",
		},
		"Hit": {
			"count": "How many matches this redaction rule replaced.",
			"rule":  "The key of the redaction rule that matched.",
		},
		"HostGuide": {
			"deep_link": "Address of the DNS host's records page for this domain, where the records " +
				"are added.",
			"detected": "True when the DNS host was read from the domain's name servers; false when " +
				"the team picked it or it is unknown.",
			"host_form": "How this host wants a record's name typed: relative (without the domain) or " +
				"fqdn (the whole name).",
			"key": "Short key of the DNS host, such as cloudflare, route53 or godaddy; other when the " +
				"host is not known.",
			"name": "The DNS host's name as people know it, such as Cloudflare; Your DNS host when it " +
				"is not known.",
		},
	}
}

/*
schemaTexts59 returns the texts from HostGuide on.
*/
func schemaTexts59() map[string]map[string]string {
	return map[string]map[string]string{
		"HostGuide": {
			"quirks": "Things that trip people up at this DNS host, as sentences in the reader's " +
				"language.",
			"steps": "The steps to add the records at this DNS host, in order, as sentences in the " +
				"reader's language.",
		},
		"IdentityContactPreview": {
			"action": "What a valid token would do: reuse when the customer it names already exists, " +
				"create when a new one would be made.",
			"contact_id": "Id of the existing customer the token names, starting with ctc_. Null when " +
				"the action is create.",
		},
		"IdentityMistake": {
			"code": "Short code of the recognized slip, such as bearer_prefix, exp_milliseconds or " +
				"public_key_as_secret.",
			"hint": "What to change to fix the slip, in words.",
		},
		"IdentitySecret": {
			"algorithm": "The signing algorithm tokens must use with this secret: HS256, RS256, ES256 " +
				"or EdDSA.",
			"audience": "The audience (aud) tokens checked with this secret must carry. Null when none " +
				"is required.",
			"fingerprint": "A short text that tells secrets apart without showing them, such as the " +
				"last 4 characters of a shared secret.",
			"issuer": "The issuer (iss) expected in tokens from the provider's key set. Null unless " +
				"the source is jwks or none is set.",
			"jwks_error": "Why the last fetch of the provider's key set failed. Null when it worked or " +
				"the source is not jwks.",
			"jwks_fetched_at": "When the provider's key set was last fetched. Null when the source is " +
				"not jwks.",
			"jwks_url": "Address of the provider's key set. Null unless the source is jwks.",
			"kid": "The value a token's kid header must carry to pick this secret; equal to the " +
				"secret's id (isk_...). Empty when the source is jwks.",
			"last_used_at": "When a token was last checked with this secret. Null when it was never " +
				"used.",
		},
	}
}

/*
schemaTexts60 returns the texts from IdentitySecret on.
*/
func schemaTexts60() map[string]map[string]string {
	return map[string]map[string]string{
		"IdentitySecret": {
			"name":       "The name the team gave this secret.",
			"retires_at": "When a retiring secret stops working. Null when no retirement is set.",
			"revoked_at": "When the secret was revoked. Null when it was not.",
			"source": "What the secret verifies tokens with: shared_secret, public_key (an uploaded " +
				"key) or jwks (a provider's key set).",
			"status": "Where the secret stands: active, retiring (still works until retires_at) or " +
				"revoked.",
		},
		"IdentitySettings": {
			"client_writable_attributes": "Keys of the customer attributes the browser may set without " +
				"a signed token; empty when none.",
			"mode": "How visitors sign in to the messenger: off, optional (signed tokens are accepted " +
				"but not needed) or required.",
		},
		"IdentityStats": {
			"anonymous": "How many messenger sessions started without a signed-in customer in the " +
				"period.",
			"by_day": "The counts for each day of the period, by outcome.",
			"days":   "How many days the counts cover.",
			"invalid": "How many session starts were refused in the period because their token was not " +
				"valid.",
			"totals": "Counts for the whole period by outcome: verified, anonymous_started, or the " +
				"error code of a refused token.",
			"verified": "How many messenger sessions started with a valid signed token in the period.",
			"verified_share": "The part of session starts that were signed in, from 0 to 1, such as " +
				"0.82. 0 when there were no starts.",
		},
		"IdentityStatsDay": {
			"day": "The day the counts are for, written YYYY-MM-DD in UTC, such as 2026-01-31.",
			"outcomes": "That day's counts by outcome: verified, anonymous_started, or the error code " +
				"of a refused token.",
		},
	}
}

/*
schemaTexts61 returns the texts from IdentityValidation on.
*/
func schemaTexts61() map[string]map[string]string {
	return map[string]map[string]string{
		"IdentityValidation": {
			"claims": "The claims read from the pasted token's payload, as the token says them.",
			"code":   "Error code saying why the token was refused. Null when the token is valid.",
			"header": "The header read from the pasted token, such as its alg and kid.",
			"hint": "What to change so the token passes. Null when the token is valid or no hint " +
				"applies.",
			"message": "Why the token was refused, in words. Null when the token is valid.",
			"valid":   "True when the token passed every check and would sign the customer in.",
			"warnings": "Things about a valid token that work but likely not as meant, each with a " +
				"code and a fix; empty when none.",
		},
		"Import": {
			"columns":       "The file's header row: the column names in order.",
			"created_count": "How many records the import has created so far.",
			"error":         "Why the whole import failed, in words. Null unless its status is failed.",
			"error_report_url": "API path of the report listing every failed row, such as " +
				"/api/v1/imports/imp_.../errors. Null when no row failed.",
			"errors": "The first failed rows, at most 20, each with its line in the file and why it " +
				"failed.",
			"failed_count": "How many rows could not be imported so far.",
			"file_expired_at": "When the uploaded file was deleted, 30 days after the import finished. " +
				"Null while the file is still kept.",
			"file_name":   "Name of the uploaded file, such as customers.csv.",
			"finished_at": "When the import completed or failed. Null until then.",
			"kind":        "What the file holds: contacts (customers) or companies.",
			"mapping": "The chosen mapping: which column fills which field. Empty until the import is " +
				"started.",
			"mode": "What happens to a row matching an existing record: update changes that record, " +
				"skip leaves it alone.",
			"preview": "The first 5 data rows of the file, each a list of cell texts in column order.",
			"processed_rows": "How many rows have been handled so far; compare with row_count for " +
				"progress.",
			"rollback_until": "The time until which the import can be undone, 7 days after it " +
				"finished. Null while it has not finished.",
		},
	}
}

/*
schemaTexts62 returns the texts from Import on.
*/
func schemaTexts62() map[string]map[string]string {
	return map[string]map[string]string{
		"Import": {
			"row_count": "How many data rows the file holds.",
			"skipped_count": "How many rows were left out so far, such as rows matching an existing " +
				"record in skip mode.",
			"started_at": "When the import began running. Null while it is a draft or queued.",
			"status": "Where the import stands: draft (uploaded, not started), queued, running, " +
				"completed or failed.",
			"suggested_mapping": "The mapping guessed from the file's column names, to confirm or " +
				"change before starting.",
			"updated_count": "How many existing records the import has changed so far.",
		},
		"ImportPrediction": {
			"created": "How many records the dry run would create.",
			"errors":  "The rows the dry run found would fail, each with its line in the file and why.",
			"failed":  "How many rows the dry run found would fail.",
			"rows":    "How many rows the dry run read.",
			"skipped": "How many rows the dry run would skip.",
			"updated": "How many existing records the dry run would change.",
		},
		"ImportRollback": {
			"error": "Why undoing the import failed, in words. Null unless the undo's status is " +
				"failed.",
			"finished_at": "When the undo completed or failed. Null until then.",
			"kept": "How many records the undo left as they are, because they changed or were used " +
				"since the import.",
			"kept_records": "The first kept records, at most 100, each with its id, name and why it " +
				"was kept.",
			"processed": "How many records the undo has handled so far; compare with total for " +
				"progress.",
			"removed":      "How many records the undo deleted because the import had created them.",
			"requested_at": "When undoing the import was asked for.",
			"reverted":     "How many records the undo put back to what they were before the import.",
			"status":       "Where undoing the import stands: queued, running, completed or failed.",
			"total":        "How many records the undo has to go through.",
		},
	}
}

/*
schemaTexts63 returns the texts from ImportSummary on.
*/
func schemaTexts63() map[string]map[string]string {
	return map[string]map[string]string{
		"ImportSummary": {
			"created_by_name": "Name of the teammate who started the import. Null when it is not " +
				"known.",
			"created_count": "How many records the import has created so far.",
			"error_report_url": "API path of the report listing every failed row, such as " +
				"/api/v1/imports/imp_.../errors. Null when no row failed.",
			"failed_count": "How many rows could not be imported so far.",
			"file_expired_at": "When the uploaded file was deleted, 30 days after the import finished. " +
				"Null while the file is still kept.",
			"file_name":   "Name of the uploaded file, such as customers.csv.",
			"finished_at": "When the import completed or failed. Null until then.",
			"kind":        "What the file holds: contacts (customers) or companies.",
			"processed_rows": "How many rows have been handled so far; compare with row_count for " +
				"progress.",
			"rollback_until": "The time until which the import can be undone, 7 days after it " +
				"finished. Null while it has not finished.",
			"row_count": "How many data rows the file holds.",
			"skipped_count": "How many rows were left out so far, such as rows matching an existing " +
				"record in skip mode.",
			"status": "Where the import stands: draft (uploaded, not started), queued, running, " +
				"completed or failed.",
			"updated_count": "How many existing records the import has changed so far.",
		},
		"InboundEmail": {
			"auth": "What is known about the sender's authenticity: an object with spf, dkim, dmarc " +
				"and an overall result. Null when not checked yet.",
			"classification": "What kind of mail it is: normal, auto_reply, bounce, complaint, list, " +
				"bulk, noreply, system_notice, loop, spam, forwarded or test. Null until classified.",
			"envelope_to": "The workspace's own addresses the email was delivered to.",
			"error":       "Why processing the email failed, in words. Null when it did not fail.",
			"from":        "The sender's email address from the From line. Null when it could not be read.",
			"ignore_reason": "Why the email did not become a message. Null unless its status is " +
				"ignored.",
			"processed_at": "When handling the email finished. Null while it still waits or is being " +
				"processed.",
		},
	}
}
