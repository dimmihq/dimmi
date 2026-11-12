/*
|--------------------------------------------------------------------------
| Tool Catalog: Workspace
|--------------------------------------------------------------------------
|
| The entries for the workspace itself, its setup, its members, teams and
| invitations. Changing who is in the workspace or what they may see is
| treated with care, and anything that mails a person outside the team
| needs the person's approval.
|
*/

package mcpserver

import "maps"

/*
workspaceDocs returns the entries for the workspace, members, teams,
roles, invitations, the audit log and the caller's notifications.
*/
func workspaceDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		workspaceSetupDocs(), onboardingDocs(), memberDocs(), memberChangeDocs(), invitationDocs(), teamDocs(),
		teamChangeDocs(), workspaceAccessDocs(), callerDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
workspaceSetupDocs returns the entries for the workspace's own settings,
logo and setup steps.
*/
func workspaceSetupDocs() map[string]Doc {
	return map[string]Doc{
		"workspace.update": {
			Title: "Change workspace settings",
			Use: "Changes the workspace's name, address (slug), language, time zone, the name customers see, " +
				"composer options, link checks and how long data is kept. Only the fields sent change. A new slug " +
				"keeps the old address pointing here for thirty days.",
			Returns: "the workspace as it now is.",
			Params: map[string]string{
				"slug":                 "The workspace's address: 3 to 40 lowercase letters, digits and dashes.",
				"timezone":             "An IANA time zone, such as Europe/Berlin.",
				"locale":               "The fallback language for members and customers, such as en or de.",
				"customer_facing_name": "The name customers see instead of the workspace's name. null removes it.",
				"test_mode":            "Fixed when the workspace is created; sending a different value is refused.",
				"require_two_factor": "Cannot be changed through a token: the person must switch it in the app, " +
					"after confirming their password.",
				"undo_send_seconds": "How long a reply can be taken back after sending: 0, 5, 10, 20 or 30.",
				"link_safety": "enabled, allow_domains, block_domains (hostnames, 500 at most each, a list " +
					"replaces the list) and strip_inbound_email_links.",
				"retention": "Days to keep data: closed_conversation_days 30 to 3650 (null keeps them for ever), " +
					"spam_days and trash_days 7 to 365, raw_email_days 0 to 365, export_days 1 to 30.",
			},
			Hint: "Send only the fields to change; a period outside its range is refused with the field named.",
		},
		"workspace.logo.destroy": {
			Title: "Remove the workspace logo",
			Use: "Takes the workspace logo down, so customers see the workspace without one. " +
				"Removing it twice is fine.",
			Returns: "nothing.",
			Risk:    new(RiskReach),
		},
	}
}

/*
onboardingDocs returns the entries for the setup steps of a new workspace.
*/
func onboardingDocs() map[string]Doc {
	return map[string]Doc{
		"workspace.onboarding.update": {
			Title: "Record a setup step",
			Use: "Records how far a new workspace's setup has come, so it resumes on any device. Completing it " +
				"opens the app for everyone and is final: the workspace never goes back to setup.",
			Returns: "the workspace with its onboarding state.",
			Params: map[string]string{
				"step": "The step reached: workspace, logo, channels, tags, team, knowledge, test_conversations " +
					"or done.",
				"completed": "true to finish setup for good.",
			},
		},
		"workspace.onboarding.test-conversations.store": {
			Title: "Start practice conversations",
			Use: "Starts five made-up conversations from pretend customers, so the team can practice replying, " +
				"tagging and closing. They are marked as tests: never billed and never sent anywhere.",
			Returns: "how many practice conversations started.",
			Next:    []string{"dimmi_conversations_list: open the practice conversations"},
			Hint:    "The workspace needs an inbox or messenger to receive them; create one first.",
		},
	}
}

/*
memberDocs returns the entries that read teammates and set your own
availability.
*/
func memberDocs() map[string]Doc {
	return map[string]Doc{
		"members.index": {
			Title: "List teammates",
			Use: "Lists the people in the workspace, by name by default, with role, status, availability, " +
				"conversation access and custom role. Use it to find a teammate's id (mem_...) before assigning " +
				"or mentioning them.",
			Returns: "a page of members, and next_cursor for more.",
			Next:    []string{"dimmi_members_get: read one teammate", "dimmi_teams_list: see the teams they form"},
			Params: map[string]string{
				"filter[status]":     "active or deactivated.",
				"filter[role]":       "owner, admin or member.",
				"filter[team]":       "Team ids (team_...): members of any of them.",
				"filter[two_factor]": "true or false: whether they turned on a second factor.",
				"sort":               "name (default), -created_at for newest first, -last_active_at for most recent.",
				"include":            "teams adds the teams each member is in.",
				"format":             "Leave out. csv returns every matching member as a file instead of JSON.",
			},
		},
		"members.show": {
			Title:   "Get a teammate",
			Use:     "Reads one member of the workspace. Pass me for your own membership.",
			Returns: "the member with role, status, availability, conversation access and custom role.",
			Params:  map[string]string{"include": "teams adds the teams they are in."},
		},
		"members.me.update": {
			Title: "Set your own availability",
			Use: "Changes your own membership, mostly whether you are online or away and how new conversations " +
				"reach you. Going away can hand customers' answers back to the team; coming online ends that.",
			Returns: "your membership as it now is.",
			Params: map[string]string{
				"availability":          "online or away.",
				"away_reassign":         "true to hand conversations back to the team when customers reply while away.",
				"assign_on_reply":       "inherit, always or never: whether replying assigns the conversation to you.",
				"conversation_capacity": "Most open conversations you hold at once, 1 to 500. null for no limit.",
				"capacity_by_channel":   "A capacity per channel type: widget, email or api, each 1 to 500.",
			},
		},
	}
}

/*
memberChangeDocs returns the entries that change teammates.
*/
func memberChangeDocs() map[string]Doc {
	return map[string]Doc{
		"members.update": {
			Title: "Change a teammate",
			Use: "Changes one member's role (admin or member), deactivates or reactivates them, gives or takes " +
				"away a custom role, or sets what conversations they see and their availability. A custom role " +
				"is a change of its own: send it without role or status.",
			Returns: "the member as they now are.",
			Params: map[string]string{
				"role":   "owner, admin or member. Changes touching owners need an owner in the app.",
				"status": "active or deactivated. Nobody deactivates themselves.",
				"reassign_to_member_id": "With status deactivated: the teammate (mem_...) " +
					"who takes their conversations.",
				"keep_owner":     "With role owner: true keeps you an owner too when handing ownership over.",
				"custom_role_id": "A custom role (rol_...) to give, or null to take it away.",
				"conversation_access": "all, assigned (only theirs), teams (their access teams) or " +
					"all_except_teams. Owners always see everything.",
				"conversation_access_unassigned": "Whether they also see conversations nobody holds.",
				"access_team_ids":                "Team ids (team_...) the access setting refers to.",
				"availability":                   "online or away.",
				"conversation_capacity": "Most open conversations they hold at once, 1 to 500. " +
					"null for no limit.",
			},
			Hint: "Owner changes and the caller's own access cannot be made through a token; ask the person to " +
				"do it in the app.",
		},
		"members.update-many": {
			Title: "Change several teammates",
			Use: "Gives up to 100 members the same role (admin or member) or custom role, such as moving everyone " +
				"off a custom role before it is deleted. Each member is changed on its own by the same rules as " +
				"dimmi_members_update.",
			Returns: "one result per member: updated, skipped or failed with the reason, and the counts.",
			Params: map[string]string{
				"ids":     "Member ids (mem_...), 1 to 100.",
				"changes": "role (admin or member) or custom_role_id (rol_..., or null to take it away).",
			},
		},
	}
}

/*
invitationDocs returns the entries for invitations into the workspace.
*/
func invitationDocs() map[string]Doc {
	return map[string]Doc{
		"invitations.index": {
			Title:   "List invitations",
			Use:     "Lists the workspace's invitations, newest first. Only pending ones unless filter[status] asks.",
			Returns: "a page of invitations with email, role, status, expiry and send count, and next_cursor.",
			Params:  map[string]string{"filter[status]": "pending (default), accepted, revoked or expired."},
		},
		"invitations.store": {
			Title: "Invite people",
			Use: "Emails up to 50 people an invitation to join the workspace as admin or member. Each address " +
				"gets its own answer; inviting someone already invited sends them a fresh link instead. Use only " +
				"when the person asked, with the addresses they gave.",
			Returns: "one result per address: created, resent, reactivation, already_member or invalid.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"invitations": "A list of {email, role, custom_role_id, team_ids}. role is admin or member; owners " +
					"are never invited. team_ids (team_..., 50 at most) are joined on accepting.",
			},
			Hint: "Inviting needs a verified email on the inviting account.",
		},
		"invitations.deliveries.store": {
			Title: "Resend an invitation",
			Use: "Emails a pending invitation again with a fresh link, which stops the old one working and " +
				"restarts its expiry. Allowed a few times a day per invitation.",
			Returns: "when it was sent, and the invitation.",
			Risk:    new(RiskReach),
			Hint:    "Accepted or withdrawn invitations cannot be sent again; invite the address anew instead.",
		},
		"invitations.destroy": {
			Title:   "Withdraw an invitation",
			Use:     "Withdraws a pending invitation, so its link stops working. The person can be invited again.",
			Returns: "nothing.",
		},
	}
}

/*
teamDocs returns the entries for teams and who is in them.
*/
func teamDocs() map[string]Doc {
	return map[string]Doc{
		"teams.index": {
			Title:   "List teams",
			Use:     "Lists the workspace's teams by name. Teams group teammates and can hold conversations.",
			Returns: "a page of teams with their member counts, and next_cursor for more.",
			Params:  map[string]string{"include": "members adds who is in each team."},
		},
		"teams.show": {
			Title:   "Get a team",
			Use:     "Reads one team.",
			Returns: "the team with its member count.",
			Params: map[string]string{
				"include": "members adds who is in it; conversation_counts adds its open, snoozed and closed counts.",
			},
		},
		"teams.store": {
			Title:   "Create a team",
			Use:     "Creates a team. Two teams cannot share a name, whatever the letter case.",
			Returns: "the new team.",
			Next:    []string{"dimmi_teams_members_create: put teammates in it"},
			Risk:    new(RiskWrite),
			Params:  map[string]string{"name": "The team's name, 80 characters at most."},
		},
		"teams.update": {
			Title:   "Rename a team",
			Use:     "Renames a team. Two teams cannot share a name, whatever the letter case.",
			Returns: "the team as it now is.",
			Risk:    new(RiskWrite),
			Params:  map[string]string{"name": "The new name, 80 characters at most."},
		},
	}
}

/*
teamChangeDocs returns the entries for deleting teams and changing who
is in them.
*/
func teamChangeDocs() map[string]Doc {
	return map[string]Doc{
		"teams.destroy": {
			Title: "Delete a team",
			Use: "Deletes a team. Its conversations, open, snoozed and closed, move to the team or teammate " +
				"named, or are left without a team when neither is. Ask the person where they should go first.",
			Returns: "nothing.",
			Params: map[string]string{
				"reassign_to_team_id":   "Another team (team_...) that takes the conversations.",
				"reassign_to_member_id": "A teammate (mem_...) who takes the conversations.",
			},
		},
		"teams.members.store": {
			Title: "Add teammates to a team",
			Use: "Puts up to 100 members in a team. They are told they joined, and teammates whose access " +
				"follows their teams start seeing its conversations. Deactivated members are skipped.",
			Returns: "one result per id, in order: added, already_member, inactive or not_found.",
			Params:  map[string]string{"member_ids": "Member ids (mem_...), 1 to 100."},
		},
		"teams.members.destroy": {
			Title:   "Remove a teammate from a team",
			Use:     "Takes one member out of a team. Their access to the team's conversations may end with it.",
			Returns: "nothing.",
		},
	}
}
