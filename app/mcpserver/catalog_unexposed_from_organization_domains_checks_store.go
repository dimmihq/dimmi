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
unexposedDocsFromOrganizationDomainsChecksStore returns entries from
organization-domains.checks.store on.
*/
func unexposedDocsFromOrganizationDomainsChecksStore() map[string]Doc {
	return map[string]Doc{
		"organization-domains.checks.store": {
			Title: "Check a domain's proof",
			Use: "Looks up the proof record of a domain now. A record found makes the domain verified, " +
				"unless another organization already holds it; none found leaves it pending. It needs the " +
				"permission to manage single sign-on, which owners hold. Only a signed-in person's browser or " +
				"app can call it, never an API token.",
			Returns: "the check's status, the records found and the domain.",
			Params: map[string]string{
				"domain": "The domain's id from the list of organization domains, such as odom_4f9k2.",
			},
		},
		"organization-domains.destroy": {
			Title: "Remove an organization domain",
			Use: "Removes a domain from the organization; one routed to an enforced connection is " +
				"refused. It needs the permission to manage single sign-on, which owners hold. Only a " +
				"signed-in person's browser or app can call it, right after confirming their password or " +
				"mailed code.",
			Params: map[string]string{
				"domain": "The domain's id from the list of organization domains, such as odom_4f9k2.",
			},
		},
		"organization-domains.index": {
			Title: "List organization domains",
			Use: "Lists the email domains the organization behind the current workspace added, each with " +
				"its status, the record that proves it and the connection its sign-ins go to. It needs the " +
				"permission to manage single sign-on, which owners hold. Only a signed-in person's browser or " +
				"app can call it, never an API token.",
			Returns: "every domain of the organization.",
		},
	}
}

/*
unexposedDocsFromOrganizationDomainsStore returns entries from
organization-domains.store on.
*/
func unexposedDocsFromOrganizationDomainsStore() map[string]Doc {
	return map[string]Doc{
		"organization-domains.store": {
			Title: "Add an organization domain",
			Use: "Adds an email domain the company wants to prove it owns, and answers with the record to " +
				"publish as proof; an organization has at most twenty. Public suffixes and free mail domains " +
				"are refused, and a domain already added is returned as it is. It needs the permission to " +
				"manage single sign-on, which owners hold. Only a signed-in person's browser or app can call " +
				"it, right after confirming their password or mailed code.",
			Returns: "the domain, pending, with the name and value of the record that proves it.",
			Params: map[string]string{
				"domain": "The domain as typed, up to 253 characters, such as acme.com. Required; it " +
					"is put into its standard form.",
			},
		},
		"organization-domains.update": {
			Title: "Route a domain to a connection",
			Use: "Sends the sign-ins of a verified domain to a single sign-on connection, or to none. A " +
				"domain routed to an enforced connection cannot be moved away. It needs the permission to " +
				"manage single sign-on, which owners hold. Only a signed-in person's browser or app can call " +
				"it, right after confirming their password or mailed code.",
			Returns: "the domain as it now is.",
			Params: map[string]string{
				"domain": "The domain's id from the list of organization domains, such as odom_4f9k2.",
				"sso_connection_id": "The id of the connection the domain's sign-ins go to, such as " +
					"ssoc_4f9k2, or null for none. Required.",
			},
		},
		"password-resets.store": {
			Title: "Mail a password reset link",
			Use: "Mails a link that sets a new password; it works for 60 minutes and replaces any earlier " +
				"one. It answers the same for every address, so it never tells whether an account exists. " +
				"Anyone may call it before signing in.",
			Returns: "a message saying the link was sent if the address has an account.",
			Params: map[string]string{
				"email": "The account's email address, such as ada@example.com. Required.",
			},
		},
	}
}

/*
unexposedDocsFromPasswordResetsUpdate returns entries from password-
resets.update on.
*/
func unexposedDocsFromPasswordResetsUpdate() map[string]Doc {
	return map[string]Doc{
		"password-resets.update": {
			Title: "Set a new password",
			Use: "Redeems a mailed reset link: sets the new password, ends every session, app connection " +
				"and access token of the account, and signs the person in on this browser. Only a browser or " +
				"app holding the link can call it.",
			Returns: "the signed-in person, or the second-factor challenge to answer.",
			Params: map[string]string{
				"password": "The new password, at least the server's minimum length and at most 256 " +
					"characters. A very common one, or the account's email or name, is refused.",
				"reset": "The id from the reset link, such as prs_4f9k2. It proves nothing without the " +
					"token.",
				"token": "The secret from the reset link, sent in the body. Required; five wrong ones " +
					"delete the reset.",
			},
		},
		"push-config.show": {
			Title: "Read the push key",
			Use: "Reads the public key a browser needs to subscribe to push notifications from this dimmi " +
				"server. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the server's VAPID public key and its key id.",
		},
		"push-subscriptions.destroy": {
			Title: "Remove a push device",
			Use: "Removes one of the signed-in person's push devices, so it stops getting push " +
				"notifications. Only a signed-in person's browser or app can call it, never an API token.",
			Params: map[string]string{
				"subscription": "The device's id from the list of push devices, such as psub_4f9k2.",
			},
		},
	}
}

/*
unexposedDocsFromPushSubscriptionsIndex returns entries from push-
subscriptions.index on.
*/
func unexposedDocsFromPushSubscriptionsIndex() map[string]Doc {
	return map[string]Doc{
		"push-subscriptions.index": {
			Title: "List my push devices",
			Use: "Lists the browsers the signed-in person turned push notifications on in. They belong to " +
				"the person across workspaces, and their keys are never shown. Only a signed-in person's " +
				"browser or app can call it, never an API token.",
			Returns: "the person's push devices.",
		},
		"push-subscriptions.store": {
			Title: "Register this browser for push",
			Use: "Registers the browser's push subscription so the signed-in person gets push " +
				"notifications on it. The values come from the browser's own push subscription, so only that " +
				"browser can call it.",
			Returns: "the registered device.",
			Params: map[string]string{
				"device_label": "A name to recognize the device by, up to 80 characters, such as " +
					"Chrome on MacBook.",
				"endpoint": "The push service URL from the browser's subscription, up to 2048 " +
					"characters, such as https://fcm.googleapis.com/fcm/send/abc.",
				"keys":        "The subscription's two keys, as the browser gave them.",
				"keys.auth":   "The subscription's auth secret in base64url, up to 100 characters.",
				"keys.p256dh": "The subscription's public key in base64url, up to 200 characters.",
			},
		},
		"roles.destroy": {
			Title: "Delete a custom role",
			Use: "Deletes a custom role nobody holds; a role any member still has is refused, with how " +
				"many hold it. It needs the permission to manage roles. Only a signed-in person's browser or " +
				"app can call it, right after confirming their password or mailed code.",
			Params: map[string]string{
				"role": "The custom role's id from the roles list, such as rol_4f9k2.",
			},
		},
	}
}

/*
unexposedDocsFromRolesStore returns entries from roles.store on.
*/
func unexposedDocsFromRolesStore() map[string]Doc {
	return map[string]Doc{
		"roles.store": {
			Title: "Create a custom role",
			Use: "Creates a custom role that narrows the admin or member role to a chosen set of " +
				"permissions; a workspace has at most 50. It needs the permission to manage roles, and the " +
				"caller can only grant permissions they hold themselves. Only a signed-in person's browser or " +
				"app can call it, right after confirming their password or mailed code.",
			Returns: "the new role with its permissions.",
			Params: map[string]string{
				"base_role": "The built-in role this one narrows: admin or member. Required, and it " +
					"can never change afterwards.",
				"description": "A short note on what the role is for, up to 200 characters.",
				"name": "The role's name, up to 50 characters, such as Billing support. Required and " +
					"unique among the workspace's roles, ignoring case.",
				"permissions": "The permissions the role holds, up to 200 names from the permissions " +
					"list, such as [\"members:read\"]. Each must be one the base role and the caller hold. " +
					"Required.",
			},
		},
		"roles.update": {
			Title: "Update a custom role",
			Use: "Changes a custom role's name, description or permissions; a field left out stays as it " +
				"is. It needs the permission to manage roles, and the caller must hold every permission the " +
				"role holds. Only a signed-in person's browser or app can call it, right after confirming " +
				"their password or mailed code.",
			Returns: "the role as it now is.",
			Params: map[string]string{
				"base_role": "Never send this; the base role is chosen when the role is made and a " +
					"change is refused.",
				"description": "A new note on what the role is for, up to 200 characters.",
				"name": "A new name, up to 50 characters, unique among the workspace's roles, ignoring " +
					"case.",
				"permissions": "The full new list of permissions, replacing the old one, up to 200 " +
					"names from the permissions list. Each must be one the base role and the caller hold.",
				"role": "The custom role's id from the roles list, such as rol_4f9k2.",
			},
		},
	}
}

/*
unexposedDocsFromSessionsDestroy returns entries from sessions.destroy on.
*/
func unexposedDocsFromSessionsDestroy() map[string]Doc {
	return map[string]Doc{
		"sessions.destroy": {
			Title: "End one session",
			Use: "Signs out one of the person's own sessions: the one making the request, or another from " +
				"their list, such as a lost laptop's. Ending the current one clears the browser's cookie. Only " +
				"a signed-in person's browser or app can call it, never an API token.",
			Params: map[string]string{
				"session": "The session to end: its id from the session list, such as ses_4f9k2, or " +
					"the word current for the session making the request.",
			},
		},
		"sessions.destroy-many": {
			Title: "Sign out everywhere else",
			Use: "Ends every session of the signed-in person except the one making the request. Only that " +
				"person's browser or app can call it, and they must have confirmed their password or mailed " +
				"code a moment ago.",
		},
		"sessions.index": {
			Title: "List my browser sessions",
			Use: "Lists the browser sessions the signed-in person has, so they can spot one they do not " +
				"recognize. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the person's sessions.",
		},
	}
}

/*
unexposedDocsFromSessionsStore returns entries from sessions.store on.
*/
func unexposedDocsFromSessionsStore() map[string]Doc {
	return map[string]Doc{
		"sessions.store": {
			Title: "Sign in",
			Use: "Signs a person in and sets the session cookie in their browser, by password, a mailed " +
				"link's token, a mailed code, or the second factor that finishes a sign-in. Only a browser or " +
				"app can call it. An account with a second factor gets a challenge first, which a second call " +
				"with grant two_factor answers from the same browser.",
			Returns: "the signed-in person with their workspaces, or the second-factor challenge to " +
				"answer.",
			Params: map[string]string{
				"code": "The code the person typed: the mailed sign-in code for email_code, or the " +
					"authenticator or recovery code for two_factor, such as 123456.",
				"credential": "For two_factor with a passkey, the JSON object the device signed, sent " +
					"instead of a code. At most 16 KB.",
				"email": "The account's email address, such as ada@example.com. Required for the " +
					"password and email_code grants.",
				"grant": "How to sign in: password (email and password), magic_link (token), " +
					"email_code (email and code) or two_factor (method with code or credential). Required.",
				"method": "For two_factor, the kind of factor answering, one the challenge offered: " +
					"totp, passkey or recovery_code.",
				"password": "The account's password. Required for the password grant.",
				"reauth": "true to sign in again as the person the session already belongs to; another " +
					"account is refused. false when left out.",
				"sso_bypass": "true to let an owner's password sign-in go around their company's " +
					"required single sign-on; used with the password grant. false when left out.",
				"token": "The secret from a mailed sign-in link. Required for the magic_link grant.",
			},
		},
	}
}
