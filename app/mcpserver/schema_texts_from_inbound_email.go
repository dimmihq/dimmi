/*
|--------------------------------------------------------------------------
| Written Schema Texts: InboundEmail To KnowledgeDocument
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromInboundEmail returns the texts from InboundEmail on.
*/
func schemaTextsFromInboundEmail() map[string]map[string]string {
	return map[string]map[string]string{
		"InboundEmail": {
			"received_at": "When the email arrived.",
			"size_bytes":  "Size of the raw email in bytes.",
			"status": "Where handling the email stands: received, processing, processed, ignored, " +
				"failed or quarantined.",
			"subject": "The email's subject line. Null when it has none.",
		},
		"Inbox": {
			"default": "True for the inbox where conversations land when nothing routes them " +
				"elsewhere.",
			"default_team_id": "Id of the team new conversations in this inbox go to, starting with " +
				"team_. Null for no team.",
			"name": "The inbox's name.",
			"schedule_id": "Id of the schedule holding this inbox's own office hours, starting with " +
				"sch_. Null when it follows the workspace's default.",
			"ticket_type_id": "Id of the ticket type new conversations in this inbox become, starting " +
				"with tktt_. Null for none.",
		},
		"InboxAccess": {
			"member_ids": "Ids of the teammates who may open the inbox, each starting with mem_. Used " +
				"when mode is restricted.",
			"mode": "Who may open the inbox: everyone, or restricted to the teams and teammates " +
				"listed.",
			"team_ids": "Ids of the teams who may open the inbox, each starting with team_. Used when " +
				"mode is restricted.",
		},
		"InboxAssignment": {
			"capacity_by_channel": "The most open conversations a member holds per channel, keyed by " +
				"widget, email or api, each 1 to 500, such as {\"email\": 20}.",
			"default_capacity": "The most open conversations a member without their own limit holds at " +
				"once. Null for no limit.",
			"include_away": "True when members who are away still get conversations assigned.",
			"mode":         "How new conversations get an assignee: manual, round_robin or balanced.",
			"overflow_team_id": "Id of the team tried when the first pool has nobody free, starting " +
				"with team_. Null for none.",
		},
	}
}

/*
schemaTextsFromInboxAssignment returns the texts from InboxAssignment on.
*/
func schemaTextsFromInboxAssignment() map[string]map[string]string {
	return map[string]map[string]string{
		"InboxAssignment": {
			"queue_order": "The order waiting conversations are taken in: oldest_waiting or " +
				"priority_then_oldest.",
		},
		"InboxCount": {
			"open": "How many open conversations this inbox, team or member holds.",
		},
		"InboxPreferences": {
			"pinned_attributes": "The customer attributes this teammate pinned in their inbox; empty " +
				"when none.",
			"sidebar": "This teammate's own arrangement of the inbox sidebar, a free-form object the " +
				"app saves; {} before the first save.",
			"views": "This teammate's choices per view, keyed by view key (mine, team_..., cvw_...): " +
				"its sort, status shown, list or table layout and columns.",
		},
		"InboxRef": {
			"name": "Name of the inbox this email address feeds.",
		},
		"InboxSettings": {
			"assign_on_reply": "True when a conversation nobody holds goes to whoever replies to it.",
			"color":           "The color that marks the inbox in lists. Empty when none is set.",
			"reopen_window_days": "How many days after closing a customer's reply reopens the " +
				"conversation, 0 to 365; 0 means never.",
		},
		"Instance": {
			"api_version": "The version of the API this server answers with.",
			"audit_retention_days": "How many days audit log entries are kept; 0 means for ever. Do " +
				"not look further back than this.",
			"edition": "How this server is run: self_host for an install run by its operator, or " +
				"cloud.",
			"mail_configured": "True when the server can send email, such as invitations.",
			"region": "The region this server runs in; self for a self-hosted install with no region " +
				"set.",
			"server_time": "The server's current time.",
			"version":     "The version of dimmi this server runs.",
		},
	}
}

/*
schemaTextsFromIntegration returns the texts from Integration on.
*/
func schemaTextsFromIntegration() map[string]map[string]string {
	return map[string]map[string]string{
		"Integration": {
			"channel_ids": "Ids of the channel connections that run on this installation, each " +
				"starting with chn_; empty when none.",
			"disconnected_at": "When the installation was disconnected. Null while it is connected.",
			"external_account_id": "The id of the connected account at the provider, such as a Jira " +
				"site's id. Null when unknown.",
			"external_app_id": "The id of the provider app that was installed. Null when unknown.",
			"features": "The dimmi features that use this installation, each ready or blocked by " +
				"scopes still missing.",
			"installed_at": "When the installation was connected.",
			"installed_by": "Id of the teammate who connected it, starting with mem_. Null when " +
				"unknown or they are gone.",
			"last_checked_at": "When the connection's health was last checked. Null when never " +
				"checked.",
			"last_error": "What went wrong with the connection last, in words. Null when there is no " +
				"error.",
			"last_error_code": "Short code of the connection's last error. Null when there is no " +
				"error.",
			"member_grant_count": "How many teammates have connected their own account on this " +
				"installation.",
			"metadata": "Provider-specific facts kept about the installation; its keys differ per " +
				"provider and it is {} when empty.",
			"name": "The installation's name, usually the connected account's name at the provider.",
			"provider": "The tool connected, such as linear, jira, github, notion, intercom or " +
				"zendesk.",
			"scopes": "The permission scopes the provider granted to this installation.",
			"source": "How it was connected: oauth through the provider's approval page, or manual " +
				"with pasted tokens or a key.",
			"status": "The connection's health: active, error (see last_error) or disconnected.",
		},
		"IntegrationAuthorization": {
			"authorize_url": "The provider's address to open in the browser so the person can approve " +
				"the connection.",
		},
		"IntegrationFeature": {
			"bot_scopes": "The scopes this feature needs on the installation itself; empty when none.",
			"name":       "The feature's name.",
		},
	}
}

/*
schemaTextsFromIntegrationFeature returns the texts from IntegrationFeature on.
*/
func schemaTextsFromIntegrationFeature() map[string]map[string]string {
	return map[string]map[string]string{
		"IntegrationFeature": {
			"user_scopes": "The scopes this feature needs from each teammate's own account; empty when " +
				"none.",
		},
		"IntegrationFeatureState": {
			"missing_scopes": "The scopes this feature still needs; empty when it is ready. Reconnect " +
				"the provider to grant them.",
			"name": "The feature's name.",
			"status": "Whether the feature can work on this installation: ready or " +
				"blocked_missing_scope.",
		},
		"IntegrationMemberGrant": {
			"external_user_id": "The teammate's user id at the provider.",
			"member_id":        "Id of the teammate who connected their own account, starting with mem_.",
			"revoked_at":       "When the grant was revoked. Null while it stands.",
			"scopes":           "The scopes the teammate granted from their own account.",
			"status":           "Where the grant stands: active, revoked or error.",
		},
		"IntegrationProvider": {
			"connect": "How the provider connects: oauth through an approval page, or api_key for a " +
				"key the workspace pastes.",
			"features": "The dimmi features that use this provider, each with the scopes it asks for.",
			"installable": "True when the provider can be installed from here; false when only a " +
				"custom app's pasted tokens connect it.",
			"label": "The provider's name as people know it, such as Linear or GitHub.",
			"name": "The provider's key, such as linear, jira or github. Send it wherever a provider " +
				"is asked for.",
		},
		"Invitation": {
			"accept_url": "The link the invited person opens to join, to pass on by hand. Set only in " +
				"the answer that issued or resent the invitation; null otherwise.",
			"email":        "The email address invited.",
			"expires_at":   "When the invitation stops working.",
			"last_sent_at": "When the invitation email last went out.",
		},
	}
}

/*
schemaTextsFromInvitation returns the texts from Invitation on.
*/
func schemaTextsFromInvitation() map[string]map[string]string {
	return map[string]map[string]string{
		"Invitation": {
			"role":       "The role the person gets on joining: admin or member.",
			"send_count": "How many times the invitation has been sent.",
			"status":     "Where the invitation stands: pending, accepted, revoked or expired.",
			"team_ids": "Ids of the teams the person joins on accepting, each starting with team_; " +
				"empty when none.",
		},
		"InvitationDelivery": {
			"sent_at": "When this sending of the invitation went out.",
		},
		"InvitationError": {
			"code":    "Short error code saying why this address or row was refused.",
			"message": "Why this address or row was refused, in words.",
		},
		"InvitationPreview": {
			"email_masked": "The invited address with most of its name hidden, such as g***@acme.com.",
			"expires_at":   "When the invitation stops working.",
			"has_account": "True when an account already uses the invited address, so the person signs " +
				"in instead of signing up.",
			"role":   "The role the person gets on joining: admin or member.",
			"status": "Where the invitation stands: pending, accepted, revoked or expired.",
		},
		"InvitationPreviewInviter": {
			"name": "Name of the person who sent the invitation.",
		},
		"InvitationPreviewSpace": {
			"avatar_url": "Address of the workspace's picture, or null when there is none.",
			"name":       "Name of the workspace the invitation is for.",
		},
		"InvitationResult": {
			"email": "The email address as it was given in the batch.",
			"result": "What became of this address: created, resent, already_member, reactivation or " +
				"invalid.",
		},
	}
}

/*
schemaTextsFromInvitationUser returns the texts from InvitationUser on.
*/
func schemaTextsFromInvitationUser() map[string]map[string]string {
	return map[string]map[string]string{
		"InvitationUser": {
			"name": "Name of the teammate who sent the invitation.",
		},
		"Issue": {
			"code": "Short code of the problem found in the workflow definition, such as http_raw_url.",
			"field": "The field of the step the problem is in. Left out when it is not about one " +
				"field.",
			"message": "What is wrong, in words.",
			"path_id": "Id of the path inside the workflow the problem is on. Left out when it is not " +
				"about one path.",
			"step_id": "Id of the step inside the workflow the problem is on. Left out when it is not " +
				"about one step.",
		},
		"IssueFound": {
			"external_id": "The issue's or incident's id in the connected tool. Send it to link the " +
				"issue to a ticket.",
			"key":      "The short reference people use for it in the tool, such as ENG-123.",
			"severity": "Name of the severity the tool gives it. Empty when the tool has none.",
			"status":   "Name of its current status in the tool, such as In Progress.",
			"status_group": "The stage that status stands for: backlog, unstarted, started, done or " +
				"canceled.",
			"team_name": "Name of the team, project, repository or service it belongs to in the tool.",
			"title":     "The issue's or incident's title.",
			"url":       "Address of the issue or incident in the tool.",
		},
		"IssueOption": {
			"detail": "An extra line that tells this option apart. Empty when there is none.",
			"group": "For a workflow status, the stage it stands for: backlog, unstarted, started, " +
				"done or canceled. Empty for other options.",
			"name": "The option's name as the tool shows it.",
			"rank": "For a severity, how bad it is; higher is worse. 0 for other options.",
		},
	}
}

/*
schemaTextsFromIssueSettings returns the texts from IssueSettings on.
*/
func schemaTextsFromIssueSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"IssueSettings": {
			"deploy_environment": "The deployment environment that counts as shipped, such as " +
				"production. Empty means any environment of the repository.",
			"deploy_repository": "The repository whose successful deployments prove a fix shipped. " +
				"Empty when not set.",
			"from_email": "The email address dimmi acts as when it calls the tool, used by PagerDuty. " +
				"Empty when not set.",
			"has_signing_secret": "True when the secret that signs the tool's deliveries has been " +
				"saved.",
			"impact_field_id": "Id of the tool's field where dimmi writes how many customers an " +
				"incident reaches. Empty when not set.",
			"installation_id": "Id of the installation these settings belong to, starting with itg_.",
			"needs_signing_secret": "True when this tool needs a signing secret pasted before its " +
				"deliveries can be trusted.",
			"project_id": "The tool's id of the project new issues go to by default. Empty when not " +
				"set.",
			"provider": "The tool these settings are for, such as linear, jira or github.",
			"service_id": "The tool's id of the service new incidents go to by default. Empty when not " +
				"set.",
			"severity_id": "The tool's id of the severity new incidents get by default. Empty when not " +
				"set.",
			"states": "Which dimmi ticket state each of the tool's statuses maps to: keys are the " +
				"tool's status ids, values are state ids starting with tkts_.",
			"team_id":     "The tool's id of the team new issues go to by default. Empty when not set.",
			"webhook_url": "The address to give the tool so it delivers its updates to dimmi.",
		},
		"KeptRecord": {
			"name": "Name of the record the undo left as it is.",
			"reason": "Why the record was kept, such as Changed since the import. or Has had a " +
				"conversation since the import.",
		},
		"KnowledgeDocument": {
			"chunk_count": "How many searchable pieces the document's text was cut into.",
			"error": "Why the document could not be fetched or indexed, in words. Null when there is " +
				"no error.",
			"error_code": "Short code of the document's error. Null when there is no error.",
		},
	}
}
