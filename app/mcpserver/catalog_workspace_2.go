/*
|--------------------------------------------------------------------------
| Tool Catalog: Workspace Access And Notifications
|--------------------------------------------------------------------------
|
| The entries for roles, permissions, sign-in rules, the audit log and the
| caller's own notifications. The audit log shows what other people did,
| so reading it needs the person's approval; notifications only ever
| touch the caller's own bell and settings.
|
*/

package mcpserver

import "maps"

/*
workspaceDocs2 returns the entries for roles, permissions, sign-in
rules, the audit log and the caller's notifications.
*/
func workspaceDocs2() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{accessDocs(), auditDocs(), notificationDocs(), notificationSettingDocs()} {
		maps.Copy(out, part)
	}

	return out
}

/*
accessDocs returns the entries for roles, permissions and sign-in rules.
*/
func accessDocs() map[string]Doc {
	return map[string]Doc{
		"roles.index": {
			Title: "List roles",
			Use: "Lists the built-in roles (owner, admin, member), then the workspace's custom roles by name. " +
				"A custom role narrows the admin or member role it is based on.",
			Returns: "the roles with their base role, permissions and, for custom roles, id and member count.",
			Next:    []string{"dimmi_members_update: give a teammate a custom role"},
		},
		"roles.show": {
			Title:   "Get a custom role",
			Use:     "Reads one custom role. Built-in roles have no id; read them with dimmi_roles_list.",
			Returns: "the role with its base role, permissions and member count.",
		},
		"permissions.index": {
			Title: "List permissions",
			Use: "Lists every permission a role is made of, with its name and what it allows, in the reader's " +
				"language, and whether a custom role may hold it.",
			Returns: "the permissions with key, group, label, description, assignable and the base roles it fits.",
		},
		"workspace.access-policy.show": {
			Title: "Get the sign-in rules",
			Use: "Reads how people may sign in to the workspace: allowed sign-in methods and Google domains, " +
				"session limits, IP allowlists for the dashboard and the API, two-factor and single sign-on.",
			Returns: "the rules, the caller's own IP and how many tokens are used from outside the API allowlist.",
		},
	}
}

/*
auditDocs returns the entries for the audit log.
*/
func auditDocs() map[string]Doc {
	return map[string]Doc{
		"audit-logs.index": {
			Title: "List the audit log",
			Use: "Lists who changed what in the workspace: sign-ins, role and member changes, invitations, " +
				"tokens and settings, newest first. It shows other people's activity and addresses, so use it " +
				"only for a question the person asked.",
			Returns: "a page of entries with event, actor, subject, changes and time, and next_cursor for more.",
			Next:    []string{"dimmi_audit_logs_get: read one entry in full"},
			Risk:    new(RiskSensitive),
			Params: map[string]string{
				"filter[event]": "Event names, such as member.role_changed, member.signed_in or invitation.created.",
				"filter[actor]": "One member id (mem_...) who acted.",
				"filter[via]": "One credential that was used: a token (tok_...), " +
					"OAuth client (ocl_...) or grant (ogr_...).",
				"filter[subject]": "One record that was changed: a member, workspace, team, invitation or token, " +
					"by its id.",
				"filter[created_after]":  "An RFC 3339 time; only entries after it.",
				"filter[created_before]": "An RFC 3339 time; only entries before it.",
				"sort":                   "created_at for oldest first; newest first otherwise.",
				"format":                 "Leave out. csv returns every matching entry as a file instead of JSON.",
			},
			Hint: "A filter value that names nothing is refused rather than ignored; check the id's prefix.",
		},
		"audit-logs.show": {
			Title:   "Get an audit log entry",
			Use:     "Reads one entry of the audit log, with the IP address, browser and request it came from.",
			Returns: "the entry with event, actor, subject, changes, ip, user_agent and time.",
			Risk:    new(RiskSensitive),
		},
	}
}

/*
notificationDocs returns the entries for the caller's own notifications
and reading their notification settings.
*/
func notificationDocs() map[string]Doc {
	return map[string]Doc{
		"notifications.index": {
			Title: "List your notifications",
			Use: "Lists your own notifications, the bell in the app: new conversations, assignments, replies, " +
				"mentions and approvals waiting, most recent first. Never anyone else's.",
			Returns: "a page of notifications with unread_count, unseen_count and next_cursor in meta.",
			Next:    []string{"dimmi_notifications_update_many: mark them read"},
			Params: map[string]string{
				"filter[status]":  "unread for unread ones only; all (default) for every one.",
				"filter[trigger]": "One trigger key, such as mentioned or conversation_assigned.",
				"limit":           "How many to return, up to 100. 20 by default.",
			},
		},
		"notifications.update-many": {
			Title: "Mark your notifications",
			Use: "Marks your own notifications read or unread, or the bell seen. With no ids, read marks every " +
				"unread one.",
			Returns: "the bell's unread_count and unseen_count after the change.",
			Params: map[string]string{
				"ids":  "Notification ids to mark, 100 at most. Leave out to mark all unread ones.",
				"read": "true for read, false for unread.",
				"seen": "true to clear the bell's badge without reading anything.",
			},
		},
		"notification-preferences.index": {
			Title: "Get your notification settings",
			Use: "Reads which things you are told about on which channel (in the app, email, push), with the " +
				"defaults and locked cells, and your quiet hours and pause.",
			Returns: "the triggers, channels, cells and settings.",
			Next:    []string{"dimmi_notification_preferences_update: turn a trigger on or off"},
		},
	}
}

/*
notificationSettingDocs returns the entries that change the caller's own
notification settings.
*/
func notificationSettingDocs() map[string]Doc {
	return map[string]Doc{
		"notification-preferences.update": {
			Title: "Change your notification channels",
			Use: "Turns triggers on or off per channel for yourself only. A cell set back to its default " +
				"follows the default again.",
			Returns: "your full notification settings as they now are.",
			Params: map[string]string{
				"preferences": "Up to 50 of {trigger, channel, enabled}. Triggers: new_conversation, " +
					"conversation_assigned, customer_replied, mentioned, ai_handoff_requested, " +
					"ai_action_approval_requested, snooze_expired, team_joined. Channels: in_app, email, push.",
			},
			Hint: "Locked cells cannot be turned off; read them with dimmi_notification_preferences_list.",
		},
		"notification-preferences.destroy": {
			Title:   "Reset your notification channels",
			Use:     "Puts every trigger and channel of your own settings back to the defaults.",
			Returns: "your full notification settings as they now are.",
			Risk:    new(RiskWrite),
		},
		"notification-settings.update": {
			Title: "Change your quiet hours",
			Use: "Changes your own quiet hours, pause, previews and whether you hear about conversations the " +
				"AI agent is answering. Only the fields sent change.",
			Returns: "your settings as they now are.",
			Params: map[string]string{
				"timezone":          "An IANA time zone for quiet hours. Empty follows your account's.",
				"quiet_hours_start": "HH:MM on your clock. A window past midnight belongs to the day it starts.",
				"quiet_hours_end":   "HH:MM on your clock.",
				"quiet_hours_days": "Days as a sum: Monday 1, Tuesday 2, Wednesday 4 and so on " +
					"to Sunday 64; 127 is every day.",
				"quiet_hours_bypass": "Trigger keys that still come through during quiet hours.",
				"paused_until":       "An RFC 3339 time, at most 30 days ahead, to pause everything until.",
				"clear_pause":        "true to end a pause now.",
				"show_previews":      "Whether email and push show the message text.",
				"include_ai_handled": "Whether you are told about conversations the AI agent is answering.",
			},
		},
		"notification-tests.store": {
			Title:   "Send yourself a test notification",
			Use:     "Sends you, and only you, a test notification on one channel to check that it reaches you.",
			Returns: "nothing.",
			Params:  map[string]string{"channel": "in_app, email or push."},
			Hint:    "Email needs a verified address and push a browser with push turned on.",
		},
	}
}
