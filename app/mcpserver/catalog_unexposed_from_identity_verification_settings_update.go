/*
|--------------------------------------------------------------------------
| Tool Catalog: Operations Only A Person's Browser Calls, Continued
|--------------------------------------------------------------------------
|
| The entries for the API operations that are not tools: signing in,
| the account and its sessions, uploads, single sign-on and the like.
| No agent can call them with a token, but the API document explains
| them the same way it explains the rest.
|
*/

package mcpserver

/*
unexposedDocsFromIdentityVerificationSettingsUpdate returns entries from
identity-verification.settings.update on.
*/
func unexposedDocsFromIdentityVerificationSettingsUpdate() map[string]Doc {
	return map[string]Doc{
		"identity-verification.settings.update": {
			Title: "Change visitor identity settings",
			Use: "Changes how strict the workspace is about who its messenger visitors are, and which " +
				"attributes a signed-in messenger may write itself. Switching to required is refused until " +
				"enough recent visits were signed in, unless forced. Only a signed-in person's browser or app " +
				"can call it, right after confirming their password or mailed code.",
			Returns: "the identity verification settings as they now are.",
			Params: map[string]string{
				"client_writable_attributes": "The attribute keys a signed-in messenger may write " +
					"itself: at most 20 distinct snake_case keys, such as [\"plan\", \"company_size\"]. The list " +
					"replaces the stored one.",
				"force": "true to switch to required even when the counts fall short (100 session " +
					"starts in the last week, 95% signed in). false when left out.",
				"mode": "off to ignore identity tokens, optional (the default) to prove signed-in " +
					"customers and let others chat anonymously, or required so only proven customers get a " +
					"messenger.",
			},
		},
		"imports.store": {
			Title: "Upload an import file",
			Use: "Uploads a CSV of contacts or companies and keeps it as a draft import to map and run " +
				"next. It needs a multipart form with the CSV in the field named file (at most 10 MB and " +
				"50,000 rows) and a field named kind set to contacts or companies, and the contacts:write " +
				"ability.",
			Returns: "the draft import.",
		},
	}
}

/*
unexposedDocsFromIntegrationsInstallationsStore returns entries from
integrations.installations.store on.
*/
func unexposedDocsFromIntegrationsInstallationsStore() map[string]Doc {
	return map[string]Doc{
		"integrations.installations.store": {
			Title: "Start installing an integration",
			Use: "Starts installing a tool's app, such as Slack, for the workspace and answers with the " +
				"provider URL to send the browser to, tied to the browser session that asked. When the " +
				"workspace's installation already holds every scope, the features are added at once and the " +
				"URL leads straight back. Only a signed-in person's browser can call it, with a verified email " +
				"and the right to manage integrations.",
			Returns: "the URL to send the browser to.",
			Params: map[string]string{
				"account": "For a service hosted per customer, such as Zendesk, the account to " +
					"connect, up to 63 characters, such as the subdomain acme.",
				"features": "Up to 20 feature names of the provider that the install is for, each up " +
					"to 40 characters. Leave out or send empty for every feature; an unknown one is refused.",
				"provider": "The provider's key from the list of integration providers, such as slack.",
				"return_to": "The page to come back to afterwards: a path on this dimmi site, up to " +
					"2000 characters, such as /settings/integrations.",
			},
		},
		"integrations.member-grants.destroy": {
			Title: "Disconnect a member's account",
			Use: "Takes back a teammate's own grant on an installation: its token is wiped at once and " +
				"given back to the provider. A teammate may take back their own; whoever manages integrations " +
				"may take back anyone's. Only a signed-in person's browser or app can call it, never an API " +
				"token.",
			Params: map[string]string{
				"grant": "The grant's id, such as itu_4f9k2, shown with the installation as the " +
					"person's own grant.",
				"integration": "The installation's id from the list of integrations, such as " +
					"itg_4f9k2.",
			},
		},
	}
}

/*
unexposedDocsFromIntegrationsMemberGrantsStore returns entries from
integrations.member-grants.store on.
*/
func unexposedDocsFromIntegrationsMemberGrantsStore() map[string]Doc {
	return map[string]Doc{
		"integrations.member-grants.store": {
			Title: "Connect my own account",
			Use: "Starts connecting the signed-in teammate's own account on a working installation, so " +
				"what they send through the tool goes out as them rather than as the app. It answers with the " +
				"provider URL to send their browser to, tied to their browser session. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "the URL to send the browser to.",
			Params: map[string]string{
				"integration": "The installation's id from the list of integrations, such as " +
					"itg_4f9k2. It must be active.",
				"return_to": "The page to come back to afterwards: a path on this dimmi site, up to " +
					"2000 characters, such as /settings/integrations.",
			},
		},
		"invitations.preview.show": {
			Title: "Preview an invitation",
			Use: "Shows whoever holds an invitation link what it invites them to, signed in or not, so a " +
				"page can offer to join or sign up. A link of no invitation is simply not found.",
			Returns: "the invitation as its holder sees it.",
			Params: map[string]string{
				"Dimmi-Invitation-Token": "The secret from the invitation link, sent as a request " +
					"header, never in the address.",
			},
		},
		"knowledge.uploads.store": {
			Title: "Upload knowledge files",
			Use: "Uploads a .zip of Markdown files, or one .md file, of at most 50 MB for a knowledge " +
				"source to read. It needs a multipart form with the file in the field named file, and the " +
				"settings:write ability. Pass the key it returns in the settings of the source you then " +
				"create.",
			Returns: "the upload's key and file name.",
		},
	}
}

/*
unexposedDocsFromMagicLinksStore returns entries from magic-links.store on.
*/
func unexposedDocsFromMagicLinksStore() map[string]Doc {
	return map[string]Doc{
		"magic-links.store": {
			Title: "Mail a sign-in link",
			Use: "Mails a sign-in link and code to an address, which the sign-in call then redeems. It " +
				"always answers the same, so it never tells whether the address has an account; an unknown " +
				"address only gets mail when sign-up is open or an invitation waits for it. Not found when the " +
				"server has mailed links turned off.",
			Returns: "a message saying the link was sent if the address may sign in.",
			Params: map[string]string{
				"captcha_token": "The answer the captcha widget gave the browser, needed only when the " +
					"server's sign-in options say a captcha is on.",
				"email": "The address to mail the link and code to, such as ada@example.com. Required.",
				"name": "The person's name, up to 80 characters, used for the new account when the " +
					"address has none yet.",
			},
		},
		"member-imports.store": {
			Title: "Import members from a CSV",
			Use: "Invites and updates teammates in bulk from an uploaded CSV file of at most 1,000 rows " +
				"with an email column. It needs a multipart form with the file in the field named file (add " +
				"dry_run set to true to only check it) and the permission to import members. A workspace " +
				"imports at most 5 times an hour. Only a signed-in person's browser or app can call it, right " +
				"after confirming their password or mailed code.",
			Returns: "how many members were created, updated, unchanged or failed, with the result of " +
				"each row.",
		},
	}
}

/*
unexposedDocsFromMembersSignInLockDestroy returns entries from
members.sign-in-lock.destroy on.
*/
func unexposedDocsFromMembersSignInLockDestroy() map[string]Doc {
	return map[string]Doc{
		"members.sign-in-lock.destroy": {
			Title: "Lift a member's sign-in lock",
			Use: "Lifts the lock that failed sign-in guesses put on a teammate's account, so they can try " +
				"again. It needs the members:write permission; clearing an account that is not locked changes " +
				"nothing. Only a signed-in person's browser or app can call it, never an API token.",
			Params: map[string]string{
				"member": "The id of the locked teammate, such as mem_4f9k2, from the members list. It " +
					"must be a member of the current workspace.",
			},
		},
		"members.two-factor-resets.store": {
			Title: "Mail a two-factor reset link",
			Use: "Mails a teammate who lost their second factor a link, good for 30 minutes, that turns " +
				"it off. The caller needs the permission to reset two-factor (and to manage owners when the " +
				"teammate is an owner), cannot help themselves, and the teammate must be active with a " +
				"verified email. Only a signed-in person's browser or app can call it, right after confirming " +
				"their password or mailed code.",
			Returns: "the reset with its id, the member, status and when the link runs out.",
			Params: map[string]string{
				"member": "The id of the teammate to help, such as mem_4f9k2, from the members list. " +
					"It must be a member of the current workspace.",
			},
		},
		"notification-focus.store": {
			Title: "Report dashboard focus",
			Use: "The dashboard's heartbeat: says whether the signed-in person's dashboard is visible and " +
				"which conversation it shows, so dimmi knows what they are already looking at when it notifies " +
				"them. Only a signed-in person's browser or app can call it, never an API token.",
			Params: map[string]string{
				"conversation_id": "The id of the conversation the dashboard is showing, such as " +
					"conv_4f9k2. Leave out when none is open; it is ignored when visible is false.",
				"visible": "true while the dashboard is on screen, false when it is hidden or closed.",
			},
		},
	}
}

/*
unexposedDocsFromNotificationUnsubscribesDestroy returns entries from
notification-unsubscribes.destroy on.
*/
func unexposedDocsFromNotificationUnsubscribesDestroy() map[string]Doc {
	return map[string]Doc{
		"notification-unsubscribes.destroy": {
			Title: "Resubscribe to a notification email",
			Use: "Turns the kind of notification email an unsubscribe link names back on, undoing an " +
				"unsubscribe. Nobody signs in; the link's token is the proof.",
			Returns: "the link's email, subscribed again.",
			Params: map[string]string{
				"token": "The signed token from the unsubscribe link in the email. It names only its " +
					"own member and kind of email.",
			},
		},
		"notification-unsubscribes.show": {
			Title: "Read an unsubscribe link",
			Use: "Reads what an unsubscribe link in a teammate notification email names: the workspace, " +
				"the address, the kind of email and whether it is still on. Nobody signs in; the link's token " +
				"is the proof.",
			Returns: "the workspace, email address, kind of email and whether it is subscribed.",
			Params: map[string]string{
				"token": "The signed token from the unsubscribe link in the email. It names only its " +
					"own member and kind of email.",
			},
		},
		"notification-unsubscribes.update": {
			Title: "Unsubscribe from a notification email",
			Use: "Turns off the kind of notification email an unsubscribe link names, for the member the " +
				"link belongs to. Nobody signs in; the link's token is the proof.",
			Returns: "the link's email, now unsubscribed.",
			Params: map[string]string{
				"token": "The signed token from the unsubscribe link in the email. It names only its " +
					"own member and kind of email.",
			},
		},
	}
}

/*
unexposedDocsFromOauthAuthorizationShow returns entries from
oauth.authorization.show on.
*/
func unexposedDocsFromOauthAuthorizationShow() map[string]Doc {
	return map[string]Doc{
		"oauth.authorization.show": {
			Title: "Review an app's access request",
			Use: "Reads an app's request to act for the signed-in person, for the consent screen: which " +
				"app, where it sends the answer, the access it asks for and the workspaces the person may " +
				"connect it to. The request is checked on every call. Only a signed-in person's browser or app " +
				"can call it, never an API token.",
			Returns: "the app's request with the access asked for and the workspaces on offer.",
			Params: map[string]string{
				"query": "The whole query string the app sent to the authorization page, up to 8000 " +
					"characters, such as client_id=abc&redirect_uri=...&response_type=code. Required.",
			},
		},
		"oauth.authorizations.store": {
			Title: "Approve or deny an app",
			Use: "Records the signed-in person's decision on an app's request to act for them. An " +
				"approval names one of their active workspaces and the access to give, never more than the app " +
				"asked for; the person's role still limits what the app can do. Only a signed-in person's " +
				"browser or app can call it, never an API token.",
			Returns: "the address to send the person to: back to the app with a code, or with the news " +
				"that they refused.",
			Params: map[string]string{
				"decision": "approve or deny. Required.",
				"query": "The whole query string the app sent to the authorization page, up to 8000 " +
					"characters, exactly as it was reviewed. Required.",
				"scopes": "The access to give, up to 60 names from those the app asked for, such as " +
					"[\"conversations:read\"]. Leave out to give everything it asked for.",
				"workspace_id": "The id of the workspace the app may act in, such as ws_4f9k2, one the " +
					"person is an active member of. Required to approve.",
			},
		},
	}
}
