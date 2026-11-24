/*
|--------------------------------------------------------------------------
| Written Schema Texts: SnoozeLists To TicketEvent
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromSnoozeLists returns the texts from SnoozeLists on.
*/
func schemaTextsFromSnoozeLists() map[string]map[string]string {
	return map[string]map[string]string{
		"SnoozeLists": {
			"teams": "Each team's own snooze list, keyed by team id such as team_...; empty when no " +
				"team has one.",
			"workspace": "The workspace's own snooze list; null when it uses the built-in presets.",
		},
		"SnoozeOption": {
			"at": "Time of day the snooze ends, as HH:MM such as 09:00, for the kinds day_at and " +
				"weekday_at. Left out otherwise.",
			"days_ahead": "How many days ahead the snooze ends, for the kind day_at. Left out " +
				"otherwise.",
			"key": "Stable name of the choice within its list. Send it to pick this choice.",
			"kind": "How the end is worked out: preset (a built-in preset), in (minutes from now), " +
				"day_at (days ahead at a time) or weekday_at (next such weekday at a time).",
			"label": "Name of the choice as people read it, such as Later today.",
			"minutes": "How many minutes from now the snooze ends, for the kind in. Left out " +
				"otherwise.",
			"preset": "Built-in preset the choice uses, for the kind preset: later_today, tomorrow, " +
				"next_week or one_week. Left out otherwise.",
			"weekday": "Weekday the snooze ends on, 1 for Monday to 7 for Sunday, for the kind " +
				"weekday_at. Left out otherwise.",
		},
		"SocialAccount": {
			"connected": "True when this Page or Instagram account is already connected to the " +
				"workspace.",
			"name":        "The Page's or Instagram account's name.",
			"picture_url": "Address of the account's picture; empty when it has none.",
			"username":    "The account's username on the platform; empty when it has none.",
		},
		"SocialChannel": {
			"account_id":  "Meta's own id of the Facebook Page or Instagram account.",
			"inbox_id":    "Id of the inbox the channel's conversations land in, such as inbox_....",
			"last_error":  "Why the connection last failed; null when it is healthy.",
			"name":        "The Page's or Instagram account's name.",
			"picture_url": "Address of the account's picture; empty when unknown.",
		},
	}
}

/*
schemaTextsFromSocialChannel returns the texts from SocialChannel on.
*/
func schemaTextsFromSocialChannel() map[string]map[string]string {
	return map[string]map[string]string{
		"SocialChannel": {
			"platform": "Which platform the channel is on: facebook or instagram.",
			"status":   "Where the connection stands: pending, active, error or disconnected.",
			"username": "The account's username on the platform; empty when unknown.",
		},
		"SocialConversation": {
			"channel_id": "Id of the Messenger or Instagram connection the conversation runs on, such " +
				"as chn_....",
			"human_until": "Until when a teammate may still reply by hand; null when the customer has " +
				"not written.",
			"last_wrote_at": "When the customer last wrote; null if they have not.",
			"open_until": "Until when anyone, the AI included, may reply; null when the customer has " +
				"not written.",
			"phase": "What Meta still allows: open while anyone may reply, human while only a teammate " +
				"may, closed until the customer writes again.",
			"platform": "Which platform the conversation is on: facebook or instagram.",
		},
		"SocialFinding": {
			"check":  "What was asked of Meta: token, delivery or messages.",
			"detail": "Meta's own words when the check does not hold; empty when it does.",
			"ok":     "True when the check holds.",
		},
		"SocialGrant": {
			"accounts": "The Facebook Pages or Instagram accounts the sign-in at Meta shared, each " +
				"marked if already connected.",
			"grant": "Token for this sign-in at Meta. Send it back with the chosen account to connect " +
				"it.",
		},
		"SocialProfile": {
			"followers": "Instagram only: how many followers the customer has; 0 when unknown or on " +
				"Messenger.",
			"follows":     "Instagram only: true when the customer follows the connected account.",
			"name":        "The customer's name on the platform; empty when unknown.",
			"picture_url": "Address of the customer's profile picture; empty when unknown.",
			"username":    "The customer's username on the platform; empty when unknown.",
		},
	}
}

/*
schemaTextsFromSocialProfile returns the texts from SocialProfile on.
*/
func schemaTextsFromSocialProfile() map[string]map[string]string {
	return map[string]map[string]string{
		"SocialProfile": {
			"verified": "Instagram only: true when the customer's account is verified.",
		},
		"SocialSignupConfig": {
			"app_id":        "Id of the Meta app the sign-in popup opens for.",
			"available":     "True when this server is set up to open Meta's sign-in popup at all.",
			"config_id":     "Id of the platform's login configuration at Meta, passed to the popup.",
			"graph_version": "Version of Meta's Graph API the popup uses, such as v21.0.",
		},
		"StartAnswer": {
			"expires_at": "When this sign-in attempt stops being valid.",
			"interval_seconds": "Device sign-in only: how many seconds to wait between polls of the " +
				"complete step.",
			"method": "Which sign-in was started: device or browser.",
			"redirect_uri": "Browser sign-in only: the address the browser lands on after signing in. " +
				"It does not load; paste that address into the complete step.",
			"state": "Token naming this sign-in attempt. Send it back when completing the sign-in.",
			"url": "Page to open: for a device sign-in where the code is typed, for a browser sign-in " +
				"the authorize page.",
			"user_code": "Device sign-in only: the code to type on the page at url.",
		},
		"Surface": {
			"launcher": "Whether this visitor sees the messenger's launcher: true, false, or a rule " +
				"object the browser still checks on each page.",
			"start": "True when this visitor may start a conversation.",
			"tabs": "Whether this visitor sees each messenger tab, keyed by tab name: true, false or a " +
				"rule object still to check on each page. Left out when no tab has a rule.",
		},
		"Tag": {
			"color": "The tag's color as a hex value, such as #ff5500, kept as the workspace wrote it; " +
				"null when it has none.",
			"icon": "Name of the tag's icon; null when it has none.",
			"name": "The tag's name.",
		},
	}
}

/*
schemaTextsFromTagCounts returns the texts from TagCounts on.
*/
func schemaTextsFromTagCounts() map[string]map[string]string {
	return map[string]map[string]string{
		"TagCounts": {
			"companies":     "How many live companies carry the tag.",
			"contacts":      "How many live contacts carry the tag.",
			"conversations": "How many live conversations carry the tag.",
		},
		"TaggedList": {
			"matched": "How many values of the list named a customer, or how many customers the " +
				"audience matched.",
			"rows": "How many values the uploaded list held. Left out when an audience was tagged " +
				"instead of a list.",
			"tag_id": "Id of the tag that was applied, such as tag_.... Send it wherever a tag is " +
				"asked for.",
			"tag_name": "Name of the tag that was applied.",
			"tagged": "How many customers were newly tagged; those who already had the tag are not " +
				"counted.",
			"unmatched": "How many values of the list named nobody; 0 when an audience was tagged.",
			"unmatched_sample": "Some of the values that named nobody, to help fix the list; empty " +
				"when all matched.",
		},
		"TallyBroadcastRecipientsRow": {
			"bounced":    "How many recipients' messages bounced.",
			"clicked":    "How many recipients clicked a link in the message.",
			"complained": "How many recipients marked the message as spam.",
			"converted":  "How many recipients reached the broadcast's goal.",
			"delivered":  "How many recipients' messages were delivered.",
			"failed":     "How many recipients' messages failed to send.",
			"opened":     "How many recipients opened the message.",
			"queued":     "How many recipients' messages are waiting to be sent.",
			"reacted":    "How many recipients reacted to the message.",
			"reactions":  "How many reactions the message got in all.",
			"recalled":   "How many recipients' messages were recalled.",
			"replied":    "How many recipients replied.",
			"sent":       "How many recipients' messages were sent.",
			"skipped":    "How many recipients were skipped and not sent to.",
			"suppressed": "How many recipients were held back because their address is suppressed.",
		},
	}
}

/*
schemaTextsFromTallyBroadcastRecipientsRow returns the texts from
TallyBroadcastRecipientsRow on.
*/
func schemaTextsFromTallyBroadcastRecipientsRow() map[string]map[string]string {
	return map[string]map[string]string{
		"TallyBroadcastRecipientsRow": {
			"total":        "How many recipients the broadcast has in all.",
			"unsubscribed": "How many recipients unsubscribed.",
		},
		"Team": {
			"member_count": "How many members the team has.",
			"members":      "The team's members. Only present when the request asked to include them.",
			"name":         "The team's name.",
		},
		"TeamConversationCounts": {
			"closed":  "How many of the team's conversations are closed.",
			"open":    "How many of the team's conversations are open.",
			"snoozed": "How many of the team's conversations are snoozed.",
		},
		"TeamMemberResult": {
			"member_id": "The member id exactly as the request sent it.",
			"result":    "What adding this member came to: added, already_member, inactive or not_found.",
		},
		"TeamMemberResultsOutputBody": {
			"data": "One result for each member the request asked to add to the team.",
		},
		"TeamSummary": {
			"name": "The team's name.",
		},
		"Teammate": {
			"avatar_url":     "Address of the teammate's picture; null when they have none.",
			"email":          "The teammate's email address.",
			"email_verified": "True when the teammate has confirmed their email address.",
			"name":           "The teammate's name.",
		},
		"TestConversations": {
			"count": "How many practice conversations were started.",
		},
	}
}

/*
schemaTextsFromTestFinding returns the texts from TestFinding on.
*/
func schemaTextsFromTestFinding() map[string]map[string]string {
	return map[string]map[string]string{
		"TestFinding": {
			"code":     "Stable code naming what the forwarding test learned.",
			"message":  "What the finding means, in plain words.",
			"severity": "How serious the finding is: error, warn or info.",
		},
		"Ticket": {
			"assignee_id": "Id of the teammate who works the ticket, such as mem_...; null when nobody " +
				"is assigned.",
			"close_loop": "True when the ticket is marked to close the loop with its customers; false " +
				"by default.",
			"company_id": "Id of the company the ticket belongs to, such as cmp_...; null for none.",
			"contact_id": "Id of the customer the ticket belongs to, such as ctc_...; null for none.",
			"conversation_id": "Id of the conversation a customer ticket is tied to, where its replies " +
				"live, such as conv_...; null for other kinds.",
			"created_by_type": "Who made the ticket: member, contact, agent, workflow, integration or " +
				"system.",
			"customer_visible": "True when the customer can see the ticket.",
			"deleted_at":       "When the ticket was deleted; null while it is live.",
			"description": "The ticket's description, up to 20000 characters; empty when none was " +
				"written.",
			"external_id": "The caller's own id for the ticket, unique in the workspace, so a retry " +
				"finds the ticket it already made; null when none was sent.",
			"fields": "The ticket's field values, keyed by each field's key; empty when none are set.",
			"group": "Group of the ticket's state: submitted, in_progress, waiting_customer, " +
				"waiting_team or resolved.",
			"inbox_id":             "Id of the inbox the ticket belongs to, such as inbox_....",
			"issues":               "The issues in outside trackers that stand behind the ticket; empty for none.",
			"kind":                 "Kind of ticket: customer (tied to one conversation), back_office or tracker.",
			"linked_conversations": "How many conversations are linked to the ticket.",
			"links": "The ticket's links to other tickets and conversations. Only present when one " +
				"ticket is read on its own.",
		},
	}
}

/*
schemaTextsFromTicket returns the texts from Ticket on.
*/
func schemaTextsFromTicket() map[string]map[string]string {
	return map[string]map[string]string{
		"Ticket": {
			"number":      "The ticket's number within the workspace; the reference is T- followed by it.",
			"priority":    "The ticket's priority: none, low, medium, high or urgent.",
			"reference":   "The ticket's reference as the team reads it, such as T-482.",
			"resolved_at": "When the ticket was resolved; null while it is not.",
			"sla_clocks": "The service level clocks on the ticket. Only present when one ticket is " +
				"read on its own.",
			"sla_policy_id": "Id of the service level policy the ticket follows, such as sla_...; null " +
				"for none.",
			"state_id": "Id of the ticket's state, such as tkts_.... Send it wherever a ticket state " +
				"is asked for.",
			"state_since": "When the ticket entered its current state.",
			"suggestions": "Field values the AI proposed, keyed by field key, waiting until a person " +
				"accepts them; empty for none.",
			"team_id": "Id of the team the ticket is assigned to, such as team_...; null for none.",
			"title":   "The ticket's title, up to 200 characters.",
			"type_id": "Id of the ticket's type, such as tktt_.... Send it wherever a ticket type is " +
				"asked for.",
			"version": "Number that goes up on every change. Send it back when editing so an edit made " +
				"against an older copy is refused.",
		},
		"TicketEvent": {
			"actor_id": "Id of who made the change: a teammate (mem_...) or a customer (ctc_...). Null " +
				"for the system and for other actor types.",
			"actor_type": "Kind of actor that made the change, such as member, contact, agent, " +
				"integration or system.",
			"data": "Details of the change as an object whose keys depend on the kind, such as " +
				"{\"days\": 7} for auto_resolved. Empty object when there is nothing to add.",
			"kind": "What happened, such as created, state, update, assign, type, link, unlink, note, " +
				"close_loop, customer_notified, auto_resolved, deleted or restored.",
			"reference": "The ticket's short reference, such as T-42. Present only in the " +
				"workspace-wide event export; left out in one ticket's history.",
			"ticket_id": "Id of the ticket this entry belongs to, such as tkt_.... Send it wherever a " +
				"ticket is asked for.",
		},
	}
}
