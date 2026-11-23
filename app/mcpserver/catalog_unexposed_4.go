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
unexposedDocs22 returns entries from sso-connections.destroy on.
*/
func unexposedDocs22() map[string]Doc {
	return map[string]Doc{
		"sso-connections.destroy": {
			Title: "Delete a single sign-on connection",
			Use: "Deletes a single sign-on connection of the organization; one that is enforced is " +
				"refused until enforcement is turned off. It needs the permission to manage single sign-on, " +
				"which owners hold. Only a signed-in person's browser or app can call it, right after " +
				"confirming their password or mailed code.",
			Params: map[string]string{
				"connection": "The connection's id from the list of connections, such as ssoc_4f9k2.",
			},
		},
		"sso-connections.index": {
			Title: "List single sign-on connections",
			Use: "Lists the single sign-on connections of the organization behind the current workspace. " +
				"It needs the permission to manage single sign-on, which owners hold. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "the organization's connections with their protocol, status and settings.",
		},
		"sso-connections.show": {
			Title: "Read a single sign-on connection",
			Use: "Reads one single sign-on connection of the organization with its settings; the client " +
				"secret is never shown. It needs the permission to manage single sign-on, which owners hold. " +
				"Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the connection with its settings.",
			Params: map[string]string{
				"connection": "The connection's id from the list of connections, such as ssoc_4f9k2.",
			},
		},
	}
}

/*
unexposedDocs23 returns entries from sso-connections.store on.
*/
func unexposedDocs23() map[string]Doc {
	return map[string]Doc{
		"sso-connections.store": {
			Title: "Create a single sign-on connection",
			Use: "Creates a draft SAML or OpenID Connect connection for the organization; it has at most " +
				"ten. OpenID Connect needs an issuer, a client id and a client secret; SAML settings can be " +
				"sent now or later. It still has to be tested and made active. It needs the permission to " +
				"manage single sign-on, which owners hold. Only a signed-in person's browser or app can call " +
				"it, right after confirming their password or mailed code.",
			Returns: "the new connection in draft.",
			Params:  unexposedParams1(),
		},
	}
}

/*
unexposedDocs24 returns entries from sso-connections.tests.store on.
*/
func unexposedDocs24() map[string]Doc {
	return map[string]Doc{
		"sso-connections.tests.store": {
			Title: "Start a test sign-in",
			Use: "Starts a test sign-in through a connection and answers with the identity provider's " +
				"address to open in a new browser tab. The browser gets a cookie with the answer, so the reply " +
				"comes back to this browser only. It needs the permission to manage single sign-on, which " +
				"owners hold. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the URL to open to sign in at the identity provider.",
			Params: map[string]string{
				"connection": "The id of the connection to test, such as ssoc_4f9k2.",
			},
		},
	}
}

/*
unexposedDocs25 returns entries from sso-connections.update on.
*/
func unexposedDocs25() map[string]Doc {
	return map[string]Doc{
		"sso-connections.update": {
			Title: "Update a single sign-on connection",
			Use: "Changes a connection's settings, status, enforcement and how new members join; a field " +
				"left out stays as it is. Changing the protocol settings or the secret sends a connection that " +
				"is not disabled back to draft, unenforced and untested. It needs the permission to manage " +
				"single sign-on, which owners hold. Only a signed-in person's browser or app can call it, " +
				"right after confirming their password or mailed code.",
			Returns: "the connection as it now is.",
			Params:  unexposedParams2(),
		},
	}
}

/*
unexposedDocs26 returns entries from sso-lookups.store on.
*/
func unexposedDocs26() map[string]Doc {
	return map[string]Doc{
		"sso-lookups.store": {
			Title: "Look up an email's single sign-on",
			Use: "Tells the sign-in page whether an email address's company signs in through its identity " +
				"provider, and where to send the person. Anyone may call it before signing in.",
			Returns: "the connection's name and sign-in address, null for none, and whether the company " +
				"requires it.",
			Params: map[string]string{
				"email": "The email address the person is signing in with, such as ada@acme.com. " +
					"Required.",
			},
		},
		"tokens.abilities.index": {
			Title: "List token abilities",
			Use: "Lists every ability group a personal access token can be given, each marked allowed " +
				"when the caller's role holds a permission of it. Call it before creating a token. Only a " +
				"signed-in person's browser or app can call it, never an API token.",
			Returns: "the ability groups, each marked allowed or not.",
		},
		"tokens.destroy": {
			Title: "Revoke an access token",
			Use: "Revokes a personal access token the caller may manage, so it stops working at once; the " +
				"token's owner sees it in their security log. Only a signed-in person's browser or app can " +
				"call it, never an API token.",
			Params: map[string]string{
				"token": "The token's id from the token list, such as tok_4f9k2. This is not the " +
					"secret.",
			},
		},
	}
}

/*
unexposedDocs27 returns entries from tokens.index on.
*/
func unexposedDocs27() map[string]Doc {
	return map[string]Doc{
		"tokens.index": {
			Title: "List access tokens",
			Use: "Lists personal access tokens in the current workspace, newest first: the caller's own, " +
				"or with filter[member] another member's or everyone's, which needs the permission to manage " +
				"any token. Secrets are never shown. Only a signed-in person's browser or app can call it, " +
				"never an API token.",
			Returns: "a page of tokens with their names, abilities and expiry.",
			Params: map[string]string{
				"cursor": "Leave out for the first page. To read the next page, pass the next_cursor " +
					"the previous page returned.",
				"filter[member]": "A member id, such as mem_4f9k2, to list that member's tokens, or " +
					"all for everyone's. Leave out for the caller's own.",
				"limit": "How many items to return in one page, from 1 to 100; 25 when left out.",
				"q":     "Accepted but not used; this list cannot be searched.",
			},
		},
		"tokens.show": {
			Title: "Read one access token",
			Use: "Reads one personal access token the caller may manage: their own, or anyone's with the " +
				"permission to manage any token. The secret is never shown again. Only a signed-in person's " +
				"browser or app can call it, never an API token.",
			Returns: "the token with its name, abilities and expiry.",
			Params: map[string]string{
				"token": "The token's id from the token list, such as tok_4f9k2. This is not the " +
					"secret.",
			},
		},
	}
}

/*
unexposedDocs28 returns entries from tokens.store on.
*/
func unexposedDocs28() map[string]Doc {
	return map[string]Doc{
		"tokens.store": {
			Title: "Create an access token",
			Use: "Mints a personal access token for the signed-in person in the current workspace and " +
				"shows its secret this one time. Only that person's browser or app can call it, with a " +
				"verified email and right after confirming their password or mailed code. A token can never do " +
				"more than the person's role allows.",
			Returns: "the new token with its secret, shown only once.",
			Params: map[string]string{
				"abilities": "What the token may do: 1 to 40 ability groups from the token abilities " +
					"list, such as [\"conversations:read\", \"contacts:write\"], or [\"*\"] for every group the " +
					"person's role allows.",
				"expires_at": "When the token stops working, as an ISO 8601 time in the future and at " +
					"most five years away, such as 2027-01-01T00:00:00Z. Leave out for a token that does not " +
					"expire.",
				"name": "A name to recognize the token by, up to 80 characters, such as Zapier sync. " +
					"Required.",
			},
		},
		"tokens.update": {
			Title: "Rename an access token",
			Use: "Changes the name of a personal access token the caller may manage; nothing else about a " +
				"token can change. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the token as it now is.",
			Params: map[string]string{
				"name": "The token's new name, up to 80 characters, such as Zapier sync. Required.",
				"token": "The token's id from the token list, such as tok_4f9k2. This is not the " +
					"secret.",
			},
		},
	}
}
