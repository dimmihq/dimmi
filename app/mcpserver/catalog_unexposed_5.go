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
unexposedDocs29 returns entries from two-factor-resets.update on.
*/
func unexposedDocs29() map[string]Doc {
	return map[string]Doc{
		"two-factor-resets.update": {
			Title: "Finish a two-factor reset",
			Use: "Redeems the link an admin mailed: removes every second factor of the account, ends all " +
				"its sessions and app connections, and lifts its sign-in lock. The teammate calls it from any " +
				"device, signed in or not; the token is the proof, with the password when the account has one.",
			Returns: "the reset, now completed.",
			Params: map[string]string{
				"password": "The account's password. Required when the account has one; leave out when " +
					"it has none.",
				"reset": "The id from the reset link, such as ott_4f9k2. It proves nothing without the " +
					"token.",
				"token": "The secret from the reset link, sent in the body. Required; five wrong " +
					"tokens or passwords burn the link.",
			},
		},
		"user.auth-factors.destroy": {
			Title: "Remove a second factor",
			Use: "Removes one of the signed-in person's own factors. Removing the last one that keeps " +
				"two-factor authentication on turns it off; recovery codes cannot be removed while it is on. " +
				"Only a signed-in person's browser or app can call it, right after confirming their password " +
				"or mailed code.",
			Params: map[string]string{
				"factor": "The factor's id from the list of second factors, such as fac_4f9k2.",
			},
		},
		"user.auth-factors.destroy-many": {
			Title: "Turn off two-factor authentication",
			Use: "Removes every second factor of the signed-in person, which turns two-factor " +
				"authentication off; when it was not on, only unfinished factors are deleted. Only a signed-in " +
				"person's browser or app can call it, right after confirming their password or mailed code.",
		},
	}
}

/*
unexposedDocs30 returns entries from user.auth-factors.index on.
*/
func unexposedDocs30() map[string]Doc {
	return map[string]Doc{
		"user.auth-factors.index": {
			Title: "List my second factors",
			Use: "Lists the signed-in person's own second factors: authenticator apps, passkeys and " +
				"recovery codes, pending or active, and which one is preferred. Only a signed-in person's " +
				"browser or app can call it, never an API token.",
			Returns: "the person's factors with their type, status and last use.",
		},
		"user.auth-factors.store": {
			Title: "Add a second factor",
			Use: "Starts adding a second factor to the signed-in person's account, or makes a fresh set " +
				"of recovery codes. The factor stays pending until the factor update activates it with its " +
				"first code or signed passkey. Only a signed-in person's browser or app can call it, right " +
				"after confirming their password or mailed code.",
			Returns: "the pending factor with its setup (secret and QR code, or passkey options), or the " +
				"new recovery codes.",
			Params: map[string]string{
				"name": "A name to recognize the factor by, up to 60 characters, such as Work phone. " +
					"Leave out for the kind's default name.",
				"type": "The kind of factor to add: totp for an authenticator app, passkey, or " +
					"recovery_code for a new set of recovery codes. Required; it must be a kind the server offers.",
			},
		},
	}
}

/*
unexposedDocs31 returns entries from user.auth-factors.update on.
*/
func unexposedDocs31() map[string]Doc {
	return map[string]Doc{
		"user.auth-factors.update": {
			Title: "Activate or rename a second factor",
			Use: "Activates one of the signed-in person's pending factors with its first code or a signed " +
				"passkey, or renames a factor, or makes it the preferred one. Activating is a step of its own; " +
				"five wrong answers delete the pending factor. Only a signed-in person's browser or app can " +
				"call it, never an API token.",
			Returns: "the factor as it now is, with new recovery codes when it turned two-factor " +
				"authentication on.",
			Params: map[string]string{
				"code": "The first code from the authenticator app, which activates a pending factor, " +
					"such as 123456.",
				"credential": "The JSON object the device signed, which activates a pending passkey, " +
					"sent instead of a code. At most 16 KB.",
				"factor": "The factor's id from the list of second factors, such as fac_4f9k2.",
				"name": "A new name for the factor, up to 60 characters, such as Work phone. It cannot " +
					"be empty.",
				"preferred": "true to make this the factor asked for first at sign-in. Recovery codes " +
					"cannot be preferred.",
			},
		},
		"user.avatar.destroy": {
			Title: "Remove my profile picture",
			Use: "Removes the signed-in person's profile picture; nothing changes when there is none. " +
				"Only a signed-in person's browser or app can call it, never an API token.",
		},
		"user.avatar.update": {
			Title: "Upload my profile picture",
			Use: "Replaces the signed-in person's profile picture with an uploaded image. It needs a file " +
				"sent as a multipart form in the field named avatar, so only that person's browser or app can " +
				"call it.",
			Returns: "the person's account with the new picture.",
		},
	}
}

/*
unexposedDocs32 returns entries from user.confirmation-codes.store on.
*/
func unexposedDocs32() map[string]Doc {
	return map[string]Doc{
		"user.confirmation-codes.store": {
			Title: "Mail me a confirmation code",
			Use: "Mails the signed-in person a six-digit code to confirm who they are, good for ten " +
				"minutes. It is only for an account without a password, at most one a minute and five an hour; " +
				"an account with a password is refused. Only a signed-in person's browser or app can call it, " +
				"never an API token.",
			Returns: "a message saying the code was sent.",
		},
		"user.confirmation.show": {
			Title: "Check my recent confirmation",
			Use: "Tells whether the signed-in person confirmed who they are recently enough for a " +
				"sensitive step, so a screen can ask before the step is refused. Not found when no " +
				"confirmation holds. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "how the person last confirmed and until when it holds.",
		},
		"user.confirmation.store": {
			Title: "Confirm who I am",
			Use: "Proves again that the signed-in person is who they say, before a sensitive step such as " +
				"minting a token or ending other sessions. A person with a password types it; a person without " +
				"one types a code mailed to them. A wrong answer counts toward the sign-in lockout. Only a " +
				"signed-in person's browser or app can call it, never an API token.",
			Returns: "how the person confirmed and until when it holds.",
			Params: map[string]string{
				"code": "The six-digit code mailed to the person, such as 123456. Send it with method " +
					"email_code.",
				"method": "How the person confirms: password for an account that has one, or " +
					"email_code for an account without a password. Required.",
				"password": "The account's password. Send it with method password.",
			},
		},
	}
}

/*
unexposedDocs33 returns entries from user.connected-apps.destroy on.
*/
func unexposedDocs33() map[string]Doc {
	return map[string]Doc{
		"user.connected-apps.destroy": {
			Title: "Disconnect an app",
			Use: "Disconnects one of the signed-in person's connected apps, which ends its access at " +
				"once. Only a signed-in person's browser or app can call it, never an API token.",
			Params: map[string]string{
				"app": "The connected app's id from the list of connected apps, such as ogr_4f9k2.",
			},
		},
		"user.connected-apps.index": {
			Title: "List my connected apps",
			Use: "Lists the apps the signed-in person connected through OAuth, newest first: which app, " +
				"the workspace it acts in, the access it holds and when it last renewed it. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "the person's connected apps.",
		},
		"user.deletion-preview.show": {
			Title: "Preview deleting my account",
			Use: "Shows what deleting the signed-in person's account would do: the workspaces that block " +
				"it because they would be left without an owner, and the workspaces that would be deleted with " +
				"it. Call it before deleting the account. Only a signed-in person's browser or app can call " +
				"it, never an API token.",
			Returns: "the blocking workspaces and the ones that would be deleted.",
		},
	}
}

/*
unexposedDocs34 returns entries from user.destroy on.
*/
func unexposedDocs34() map[string]Doc {
	return map[string]Doc{
		"user.destroy": {
			Title: "Delete my account",
			Use: "Deletes the signed-in person's own account for good. Only that person's browser or app " +
				"can call it, right after confirming their password or mailed code. It is refused while they " +
				"are the only owner of a workspace that has other active members; a workspace they are alone " +
				"in goes to the trash.",
			Params: map[string]string{
				"confirm_email": "The account's own email address, typed to confirm, such as " +
					"ada@example.com. Required; case does not matter.",
			},
		},
		"user.identities.destroy": {
			Title: "Unlink a sign-in account",
			Use: "Removes one linked provider account from the signed-in person. It is refused when it " +
				"would leave them no way to sign in. Only that person's browser or app can call it, right " +
				"after confirming their password or mailed code.",
			Params: map[string]string{
				"identity": "The linked account's id from the list of linked sign-in accounts, such as " +
					"idn_4f9k2.",
			},
		},
		"user.identities.index": {
			Title: "List my linked sign-in accounts",
			Use: "Lists the provider accounts, such as a Google account, linked to the signed-in person " +
				"for signing in. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the person's linked provider accounts.",
		},
	}
}

/*
unexposedDocs35 returns entries from user.memberships.destroy on.
*/
func unexposedDocs35() map[string]Doc {
	return map[string]Doc{
		"user.memberships.destroy": {
			Title: "Leave a workspace",
			Use: "Takes the signed-in person out of one of their workspaces. The last active owner of a " +
				"workspace cannot leave. Only a signed-in person's browser or app can call it, never an API " +
				"token.",
			Params: map[string]string{
				"membership": "The person's own membership id from the list of their workspaces, such " +
					"as mem_4f9k2.",
			},
		},
		"user.memberships.index": {
			Title: "List my workspaces",
			Use: "Lists every workspace the signed-in person is a member of, with their role in each. " +
				"Workspaces in the trash are left out unless filter[trashed] asks for them. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "the person's memberships with each workspace.",
			Params: map[string]string{
				"cursor": "Accepted, but this list has a single page, so there is never a next cursor " +
					"to pass.",
				"filter[trashed]": "with to add workspaces in the trash, or only to list just those. " +
					"Leave out to list live workspaces only.",
				"limit": "Accepted, but this list always returns every membership in one page.",
				"q":     "Accepted but not used; this list cannot be searched.",
			},
		},
	}
}
