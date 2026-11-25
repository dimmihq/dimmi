/*
|--------------------------------------------------------------------------
| Written Schema Texts: CountViewsMentionsStruct To EmailDomain
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromCountViewsMentionsStruct returns the texts from
CountViewsMentionsStruct on.
*/
func schemaTextsFromCountViewsMentionsStruct() map[string]map[string]string {
	return map[string]map[string]string{
		"CountViewsMentionsStruct": {
			"unread": "How many conversations mention the caller and are still unread, up to 999.",
		},
		"CountViewsMineStruct": {
			"open":    "How many open conversations are assigned to the caller, up to 999.",
			"snoozed": "How many snoozed conversations are assigned to the caller, up to 999.",
			"unread":  "How many conversations assigned to the caller are unread, up to 999.",
		},
		"CountViewsSpamStruct": {
			"total": "How many conversations are marked as spam, up to 999.",
		},
		"CountViewsUnassignedStruct": {
			"open":   "How many open conversations have no assignee, up to 999.",
			"unread": "How many unassigned conversations are unread, up to 999.",
		},
		"CreatedIdentitySecret": {
			"algorithm": "The signing algorithm identity tokens are checked with: HS256 (shared " +
				"secret), RS256, ES256 or EdDSA (public key or key set).",
			"audience": "An extra aud value accepted on tokens checked with this secret. Null when " +
				"none is set.",
			"fingerprint": "A short mark to tell secrets apart: the last 4 characters of a shared " +
				"secret, or 16 hex characters for a public key or key set.",
			"issuer": "For a key set only: the issuer tokens must carry. Null otherwise or when it is " +
				"read off the key set's address.",
			"jwks_error": "For a key set only: why the last fetch of the provider's keys failed. Null " +
				"when it worked or the secret is not a key set.",
			"jwks_fetched_at": "For a key set only: when the provider's keys were last fetched. Null " +
				"when the secret is not a key set.",
			"jwks_url": "For a key set only: the address the provider's public keys are fetched from. " +
				"Null for a shared secret or a single public key.",
			"kid": "The value a token's kid header names this secret by; it equals the secret's id " +
				"(starting with isk_). Empty for a key set, whose tokens carry the provider's key ids.",
			"last_used_at": "When a token was last checked with this secret. Null when it has never " +
				"been used.",
		},
	}
}

/*
schemaTextsFromCreatedIdentitySecret returns the texts from
CreatedIdentitySecret on.
*/
func schemaTextsFromCreatedIdentitySecret() map[string]map[string]string {
	return map[string]map[string]string{
		"CreatedIdentitySecret": {
			"name":       "The name the team gave the secret.",
			"retires_at": "When a retiring secret stops working. Null unless status is retiring.",
			"revoked_at": "When the secret was revoked. Null unless status is revoked.",
			"secret": "The shared secret itself, shown only in this one answer; store it now. Absent " +
				"for a public key or key set.",
			"source": "What the secret verifies tokens with: shared_secret, public_key or jwks (a " +
				"provider's key set).",
			"status": "Where the secret stands: active, retiring (still works until retires_at) or " +
				"revoked.",
		},
		"CreatedIdentitySecretMeta": {
			"env_var": "The environment variable your server should keep the secret in: " +
				"DIMMI_IDENTITY_SECRET.",
			"kid_env_var": "The environment variable your server should keep the secret's kid in: " +
				"DIMMI_IDENTITY_KID.",
			"secret_shown_once": "True when this answer carries a shared secret that will never be " +
				"shown again; false when there was none to show.",
		},
		"CustomerRating": {
			"comment": "What the customer wrote with their rating. Null when they wrote nothing.",
			"editable": "True while the customer may still set or change the rating: for 7 days after " +
				"they were asked.",
			"faces": "The five faces the customer picks from, each with its score from 1 to 5, its " +
				"emoji and its word, worst first.",
			"score": "The score the customer picked, from 1 (worst) to 5 (best). Null while they have " +
				"not rated.",
			"workspace": "The name of the workspace asking for the rating, shown on the page an email " +
				"links to. Absent when not needed.",
		},
		"DNSRecord": {
			"name":  "The host name to create the record at, at the team's DNS provider.",
			"type":  "The DNS record type to create, such as CNAME or TXT.",
			"value": "The value the record must hold.",
		},
	}
}

/*
schemaTextsFromDNSSetupOffer returns the texts from DNSSetupOffer on.
*/
func schemaTextsFromDNSSetupOffer() map[string]map[string]string {
	return map[string]map[string]string{
		"DNSSetupOffer": {
			"apply_url": "The link to open at the DNS host to approve adding the records. Present only " +
				"once someone asked for the setup.",
			"host_name": "The name of the DNS host that can add the records itself.",
		},
		"DailyViews": {
			"date":      "The day counted, as a date such as 2026-11-01.",
			"messenger": "How many of that day's article views happened inside the messenger.",
			"views":     "How many times articles were viewed that day.",
		},
		"DataExport": {
			"error": "A sentence saying the export could not be built. Null unless status is failed.",
			"expires_at": "When the export file is removed and can no longer be downloaded. Null until " +
				"the file is built.",
			"file_name":     "The name of the export file, such as export.csv.",
			"format":        "The file format: txt, pdf, csv or json.",
			"include_notes": "True when the team's internal notes are part of the export.",
			"kind": "What is exported: conversations, contacts or conversation_facts (each " +
				"conversation's report figures as a spreadsheet).",
			"row_count":  "How many rows the file holds. Null until it is built.",
			"size_bytes": "Size of the export file in bytes. Null until it is built.",
			"status":     "Where the export stands: pending, running, ready, failed or expired.",
			"url": "The API path the ready file downloads from, such as " +
				"/api/v1/data-exports/dex_.../download. Null unless status is ready.",
			"warnings": "A list naming what the file could not hold as asked. An empty list when there " +
				"is nothing to flag.",
		},
		"DataExportLink": {
			"expires_at": "When the signed download link stops working, in UTC.",
			"url":        "The signed link the ready export file downloads from without credentials.",
		},
		"DataStruct": {
			"url": "The address the uploaded help center image is served from, for use in an article's " +
				"body.",
		},
		"DataStruct2": {
			"trackers": "The tracker tickets this conversation has been linked to. Empty when it " +
				"joined none.",
		},
	}
}

/*
schemaTextsFromDataStruct3On returns the texts from DataStruct3 on.
*/
func schemaTextsFromDataStruct3On() map[string]map[string]string {
	return map[string]map[string]string{
		"DataStruct3": {
			"body": "The text of an undone reply, so it can be put back in the composer. Empty when " +
				"the message was deleted rather than undone.",
			"undone": "True when the message was a reply still waiting to leave and was undone; false " +
				"when a message was deleted.",
		},
		"DeletionPreview": {
			"blocking": "Workspaces that stop the account from being deleted: the caller is the only " +
				"owner and other members remain.",
			"will_be_deleted": "Workspaces that would go to the trash with the account: the caller is " +
				"the only owner and the only member.",
		},
		"DeletionWorkspace": {
			"name": "The workspace's name.",
			"slug": "The workspace's URL name.",
		},
		"DomainCheck": {
			"found_records": "The TXT values found at the domain's verification record when it was " +
				"checked. Empty when none were published.",
			"status": "The domain's state after this check: pending, verified or failed.",
		},
		"DomainRecord": {
			"host": "The record's name as it should be typed at the domain's DNS host, which may be " +
				"relative to the domain, such as dimmi._domainkey.",
			"name": "The record's fully qualified name, such as dimmi._domainkey.acme.com.",
			"observed": "The values the last check actually found at this record. Empty when nothing " +
				"was found or no check has run.",
			"priority": "The priority to set on an MX record. Absent on other record types.",
			"purpose": "What the record is for: ownership, dkim, return_path, return_path_spf, spf or " +
				"dmarc.",
			"required": "True when the domain cannot be verified until this record is in place; false " +
				"for guidance.",
			"selector": "On a guidance record, the mail provider's own hint it refers to. Absent " +
				"otherwise.",
		},
	}
}

/*
schemaTextsFromDomainRecord returns the texts from DomainRecord on.
*/
func schemaTextsFromDomainRecord() map[string]map[string]string {
	return map[string]map[string]string{
		"DomainRecord": {
			"status": "What the last check found: verified, mismatch, missing, propagating, error, or " +
				"info for guidance a check never blocks on.",
			"type":  "The DNS record type, such as TXT, CNAME or MX.",
			"value": "The value the record must hold.",
		},
		"DomainWarning": {
			"code": "The kind of mistake found, such as duplicate_spf, dmarc_missing, " +
				"cloudflare_proxied, doubled_domain, propagating or domain_claimed.",
			"host": "The DNS name of the record the mistake is at. Absent when it does not point at " +
				"one record.",
			"message": "A sentence explaining the mistake and what to do about it.",
			"value":   "The value that fixes the mistake, where one helps. Absent otherwise.",
		},
		"Draft": {
			"attachment_ids": "Ids of the files staged in the draft, each starting with att_, at most " +
				"10. Empty when there are none.",
			"body": "The text written so far. Empty when nothing has been typed.",
			"client_updated_at": "When the draft was last changed on the writer's device. The newest " +
				"one wins when two devices save.",
			"conversation_id": "Id of the conversation the draft answers, starting with conv_. Null " +
				"when mode is new, for a conversation not yet started.",
			"mode": "What is being written: reply (to the customer), note (internal) or new (the first " +
				"message of a new conversation).",
			"recipients": "Who the draft will go to, as an object that may hold to, cc and bcc address " +
				"lists, a subject and a channel_connection_id. An empty object when nothing is staged.",
		},
		"EmailAddress": {
			"address": "The public address customers write to, such as support@acme.com.",
			"display_name": "The name shown as the sender of replies from this address, 1 to 80 " +
				"characters.",
			"forwarding_address": "The address on dimmi's inbound domain that mail really arrives at. " +
				"Forward the public address to it.",
		},
	}
}

/*
schemaTextsFromEmailAddress returns the texts from EmailAddress on.
*/
func schemaTextsFromEmailAddress() map[string]map[string]string {
	return map[string]map[string]string{
		"EmailAddress": {
			"forwarding_confirmation": "The last forwarding confirmation mail the mail provider sent: " +
				"its provider, code, confirm_url, sender and received_at. Null when none has arrived.",
			"from_name_style": "How the sender of a reply is named: member_and_name (the teammate and " +
				"the team) or name_only (just the display name).",
			"inbound_verified_at": "When mail was first proven to arrive at this address. Null while " +
				"forwarding is not yet verified.",
			"is_default": "True for the workspace's default address, which sends when no other is " +
				"chosen. At most one per workspace.",
			"last_inbound_at":  "When mail last arrived at this address. Null when none has.",
			"last_outbound_at": "When mail was last sent from this address. Null when none has.",
			"reply_to_mode": "Whether replies carry a Reply-To address with the thread's token so " +
				"answers find their conversation: token or none.",
			"reply_to_preview": "A sample of what the Reply-To address looks like, such as " +
				"support+k7f3x9q2mz8b4c1d@in.example.com. Null when reply_to_mode is none.",
			"sending_blocked_reason": "Why the address is held back: domain_degraded (its domain " +
				"stopped passing checks) or sending_paused (the workspace's sending is paused). Null when " +
				"nothing holds it.",
			"sending_state": "How replies leave: own_domain (from the address itself, on a verified " +
				"domain) or default_sender (from the forwarding address).",
			"sends_as": "The From line replies carry right now, such as \"Acme Support\" " +
				"<support@acme.com>.",
			"setup_guides": "How to forward mail to this address in each mail service: steps, common " +
				"problems and a settings link, recommended first. Empty when not loaded.",
		},
		"EmailAddressSettings": {
			"away_reply": "true when a new email that arrives outside office hours gets one automatic " +
				"reply saying when the team is back. Defaults to false.",
			"away_reply_text": "What the away reply says. {{reply_time}} becomes when the team is back.",
			"brand_color":     "The color a branded reply uses. Empty when none is set.",
			"bulk_mail_handling": "What becomes of newsletters and notifications that start no thread: " +
				"close (a closed conversation), open (an open one) or drop. Defaults to close.",
			"footer_markdown": "The Markdown text a branded reply closes with. Empty when none is set.",
			"quote_previous": "True when each reply puts the message it answers below it. Defaults to " +
				"true.",
			"subject_tag": "True when reply subjects carry the conversation number.",
			"template":    "The look of replies: plain or branded. Defaults to plain.",
		},
	}
}

/*
schemaTextsFromEmailAddressTest returns the texts from EmailAddressTest on.
*/
func schemaTextsFromEmailAddressTest() map[string]map[string]string {
	return map[string]map[string]string{
		"EmailAddressTest": {
			"completed_at": "When the test finished, by arriving, failing or timing out. Null while it " +
				"is still running.",
			"email_address": "Id of the email address under test, starting with ema_.",
			"expires_at":    "When the test stops waiting for its mail and times out.",
			"findings": "What the test learned, each with a code such as slow_forward or test_bounced, " +
				"a severity (error, warn or info) and a message. Empty when nothing was found.",
			"latency_ms": "How long the test mail took to come back, in milliseconds. Null until it " +
				"arrives.",
			"message_id": "The Message-ID of the test mail. Empty before it has left.",
			"sent_to":    "The public address the test mail was sent to.",
			"status": "Where the test stands: sending, waiting (for the mail to come back), received, " +
				"failed or timed_out.",
		},
		"EmailChange": {
			"confirmed_at": "When the link was opened and the new address took effect, as a UTC time. " +
				"Absent while pending.",
			"email": "The new email address the account is moving to.",
			"expires_at": "When the confirmation link stops working, as a UTC time. Absent when not " +
				"known.",
			"status":  "Where the move stands: pending until the mailed link is opened, then confirmed.",
			"user_id": "Id of the user account changing its address, starting with usr_.",
		},
		"EmailDomain": {
			"ascii_domain": "The domain as DNS stores it, in ASCII, such as xn--mnchen-3ya.de. Same as " +
				"domain for plain names.",
			"domain": "The domain as people write it, such as acme.com.",
			"driver": "The mail service the domain is set up to send through: ses, smtp, postmark, " +
				"resend, mailgun or sendgrid.",
			"failure_message": "A sentence explaining failure_reason in words. Null when there is no " +
				"failure.",
			"failure_reason": "Why the domain failed: not_verified, records_removed, " +
				"ownership_transferred or provider_rejected_sender. Null when it has not failed.",
		},
	}
}
