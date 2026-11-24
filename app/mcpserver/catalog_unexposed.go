/*
|--------------------------------------------------------------------------
| Tool Catalog: Operations Only A Person's Browser Calls
|--------------------------------------------------------------------------
|
| The entries for the API operations that are not tools: signing in,
| the account and its sessions, uploads, single sign-on and the like.
| No agent can call them with a token, but the API document explains
| them the same way it explains the rest.
|
*/

package mcpserver

import "maps"

/*
unexposedDocs returns the entries of the operations that are not tools.
*/
func unexposedDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		unexposedDocsFromAiChatgptCallbackStore(),
		unexposedDocsFromBroadcastPreferencesShow(),
		unexposedDocsFromChannelsFacebookSignupsStore(),
		unexposedDocsFromChannelsWhatsappSignupConfigIndex(),
		unexposedDocsFromContactListsStore(),
		unexposedDocsFromEmailChangesStore(),
		unexposedDocsFromEmailVerificationsUpdate(),
		unexposedDocsFromIdentityVerificationSettingsUpdate(),
		unexposedDocsFromIntegrationsInstallationsStore(),
		unexposedDocsFromIntegrationsMemberGrantsStore(),
		unexposedDocsFromMagicLinksStore(),
		unexposedDocsFromMembersSignInLockDestroy(),
		unexposedDocsFromNotificationUnsubscribesDestroy(),
		unexposedDocsFromOauthAuthorizationShow(),
		unexposedDocsFromOrganizationDomainsChecksStore(),
		unexposedDocsFromOrganizationDomainsStore(),
		unexposedDocsFromPasswordResetsUpdate(),
		unexposedDocsFromPushSubscriptionsIndex(),
		unexposedDocsFromRolesStore(),
		unexposedDocsFromSessionsDestroy(),
		unexposedDocsFromSessionsStore(),
		unexposedDocsFromSsoConnectionsDestroy(),
		unexposedDocsFromSsoConnectionsStore(),
		unexposedDocsFromSsoConnectionsTestsStore(),
		unexposedDocsFromSsoConnectionsUpdate(),
		unexposedDocsFromSsoLookupsStore(),
		unexposedDocsFromTokensIndex(),
		unexposedDocsFromTokensStore(),
		unexposedDocsFromTwoFactorResetsUpdate(),
		unexposedDocsFromUserAuthFactorsIndex(),
		unexposedDocsFromUserAuthFactorsUpdate(),
		unexposedDocsFromUserConfirmationCodesStore(),
		unexposedDocsFromUserConnectedAppsDestroy(),
		unexposedDocsFromUserDestroy(),
		unexposedDocsFromUserMembershipsDestroy(),
		unexposedDocsFromUserMembershipsStore(),
		unexposedDocsFromUserSecurityEventsIndex(),
		unexposedDocsFromUsersStore(),
		unexposedDocsFromWorkspaceAccessPolicyUpdate(),
		unexposedDocsFromWorkspaceDestroy(),
		unexposedDocsFromWorkspacesUpdate(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
unexposedDocsFromAiChatgptCallbackStore returns entries from
ai.chatgpt.callback.store on.
*/
func unexposedDocsFromAiChatgptCallbackStore() map[string]Doc {
	return map[string]Doc{
		"ai.chatgpt.callback.store": {
			Title: "Finish a ChatGPT sign-in",
			Use: "Finishes a ChatGPT sign-in started before. For a device sign-in, call it repeatedly at " +
				"the interval given until it stops answering pending; for a browser sign-in, send the address " +
				"the browser landed on. It needs the ai:keys ability and the same workspace that started the " +
				"attempt.",
			Returns: "pending with when to poll again, or connected with the new connection's id and the " +
				"plan's account id.",
			Params: map[string]string{
				"code": "For a browser sign-in, the full address the browser landed on, or the bare " +
					"code from it, up to 4096 characters. Leave out for a device sign-in.",
				"state": "The state id the start call returned, which names the attempt. Required.",
			},
		},
		"ai.chatgpt.connect.store": {
			Title: "Start a ChatGPT sign-in",
			Use: "Starts connecting a ChatGPT subscription as an AI source: by a device code the person " +
				"types on OpenAI's page, or by a browser sign-in whose final address they paste back. It needs " +
				"the ai:keys ability and a person at a browser to finish the sign-in; the attempt is then " +
				"finished with the ChatGPT callback.",
			Returns: "the page to open, the attempt's state id and expiry, and for a device sign-in the " +
				"code to type and how often to poll.",
			Params: map[string]string{
				"method": "device (the default) for a code typed on OpenAI's page, which needs device " +
					"login turned on in the person's ChatGPT security settings, or browser for a sign-in whose " +
					"final address is pasted back.",
			},
		},
		"auth.config.show": {
			Title: "Read sign-in options",
			Use: "Reads how people sign in on this dimmi server: whether sign-up is open, whether mailed " +
				"links, a captcha and single sign-on are on, the minimum password length, the sign-in " +
				"providers and the second factors offered. Anyone may call it before signing in; it holds no " +
				"secret.",
			Returns: "the server's sign-in options, version and languages.",
		},
	}
}

/*
unexposedDocsFromBroadcastPreferencesShow returns entries from
broadcast-preferences.show on.
*/
func unexposedDocsFromBroadcastPreferencesShow() map[string]Doc {
	return map[string]Doc{
		"broadcast-preferences.show": {
			Title: "Read broadcast preferences",
			Use: "Reads one customer's broadcast email choices from the signed link in a broadcast email: " +
				"the workspace, the address, whether they left every broadcast and each topic they may keep. " +
				"Nobody signs in; the link is the proof.",
			Returns: "the person's choices with each topic and whether they get it.",
			Params: map[string]string{
				"token": "The signed token from the preferences link in a broadcast email. It names " +
					"only its own recipient.",
			},
		},
		"broadcast-preferences.update": {
			Title: "Save broadcast preferences",
			Use: "Saves one customer's broadcast email choices from the signed link in a broadcast email: " +
				"the topics they keep, or leaving every broadcast. Nobody signs in; the link is the proof.",
			Returns: "the person's choices as they now are.",
			Params: map[string]string{
				"token": "The signed token from the preferences link in a broadcast email. It names " +
					"only its own recipient.",
				"topics": "The ids of the topics the person keeps getting, such as [\"bctp_4f9k2\"], " +
					"from the topics the page read. Topics left out are turned off.",
				"unsubscribe_all": "true to leave every broadcast, whatever topics are sent; false to " +
					"keep the topics listed.",
			},
		},
		"channels.facebook.signup-config.index": {
			Title: "Read Messenger signup settings",
			Use: "Reads what Meta's sign-in popup needs to open for connecting a Facebook page, and " +
				"whether this server offers it. It needs the settings:read ability and the permission to " +
				"manage channels.",
			Returns: "whether signup is available, with the Meta app id, config id and Graph version.",
		},
	}
}

/*
unexposedDocsFromChannelsFacebookSignupsStore returns entries from
channels.facebook.signups.store on.
*/
func unexposedDocsFromChannelsFacebookSignupsStore() map[string]Doc {
	return map[string]Doc{
		"channels.facebook.signups.store": {
			Title: "Start a Facebook connection",
			Use: "Trades the code Meta's sign-in popup handed back for the Facebook pages the person " +
				"shared, under a grant to connect one with next. It needs that popup's answer from the " +
				"person's browser, a verified email, the settings:write ability and the permission to manage " +
				"channels.",
			Returns: "a grant and the accounts Meta shared, to pick one from.",
			Params: map[string]string{
				"code": "The code Meta's sign-in popup handed back, up to 2048 characters. Required; " +
					"it is short-lived.",
				"redirect_uri": "The address the popup returned to, up to 2048 characters, which Meta " +
					"checks the code against.",
			},
		},
		"channels.instagram.signup-config.index": {
			Title: "Read Instagram signup settings",
			Use: "Reads what Meta's sign-in popup needs to open for connecting an Instagram account, and " +
				"whether this server offers it. It needs the settings:read ability and the permission to " +
				"manage channels.",
			Returns: "whether signup is available, with the Meta app id, config id and Graph version.",
		},
		"channels.instagram.signups.store": {
			Title: "Start an Instagram connection",
			Use: "Trades the code Meta's sign-in popup handed back for the Instagram accounts the person " +
				"shared, under a grant to connect one with next. It needs that popup's answer from the " +
				"person's browser, a verified email, the settings:write ability and the permission to manage " +
				"channels.",
			Returns: "a grant and the accounts Meta shared, to pick one from.",
			Params: map[string]string{
				"code": "The code Meta's sign-in popup handed back, up to 2048 characters. Required; " +
					"it is short-lived.",
				"redirect_uri": "The address the popup returned to, up to 2048 characters, which Meta " +
					"checks the code against.",
			},
		},
	}
}

/*
unexposedDocsFromChannelsWhatsappSignupConfigIndex returns entries from
channels.whatsapp.signup-config.index on.
*/
func unexposedDocsFromChannelsWhatsappSignupConfigIndex() map[string]Doc {
	return map[string]Doc{
		"channels.whatsapp.signup-config.index": {
			Title: "Read WhatsApp signup settings",
			Use: "Reads what Meta's embedded signup popup needs to open for WhatsApp, and whether this " +
				"server offers it. It needs the settings:read ability and the permission to manage channels.",
			Returns: "whether signup is available, with the Meta settings the popup needs.",
		},
		"channels.whatsapp.signups.store": {
			Title: "Connect WhatsApp from Meta's popup",
			Use: "Connects the WhatsApp number a person just picked in Meta's embedded signup popup: the " +
				"popup's code becomes a token of the customer's business and a number new to the Cloud API is " +
				"registered. It needs that popup's answer from the person's browser, a verified email, the " +
				"settings:write ability and the permission to manage channels.",
			Returns: "the connected WhatsApp channel.",
			Params: map[string]string{
				"business_id": "The id of the Meta business the popup named, when Meta said.",
				"code": "The code Meta's popup handed back, up to 1024 characters. Required; it is " +
					"short-lived and works once.",
				"coexistence": "true when the number also stays on the WhatsApp Business app. false " +
					"when left out.",
				"inbox_id": "The id of the inbox the number's conversations land in, such as " +
					"inbox_4f9k2. Leave out for the default inbox.",
				"phone_number_id": "The id of the phone number picked in the popup, when Meta said, " +
					"such as 106540352242922.",
				"redirect_uri": "The address the popup returned to, up to 512 characters, which Meta " +
					"checks the code against.",
				"waba_id": "The id of the WhatsApp Business account picked in the popup, when Meta " +
					"said, such as 102290129340398.",
			},
		},
	}
}

/*
unexposedDocsFromContactListsStore returns entries from contact-lists.store on.
*/
func unexposedDocsFromContactListsStore() map[string]Doc {
	return map[string]Doc{
		"contact-lists.store": {
			Title: "Tag customers from a CSV",
			Use: "Puts one tag on every existing customer an uploaded CSV names, by email or user id. It " +
				"needs a multipart form with the CSV in the field named file (at most 5 MB and 50,000 rows) " +
				"and the tag's name in the field named tag, and the contacts:write ability. Customers not in " +
				"the workspace are counted, never created.",
			Returns: "the tag with how many rows matched, were tagged and were not found.",
		},
		"conversation-ratings.show": {
			Title: "Read a rating request",
			Use: "Reads the rating request a rating email links to, as the customer sees it, with any " +
				"answer already given. Nobody signs in; the signed link is the proof.",
			Returns: "the rating request with its score, comment and whether it can still be changed.",
			Params: map[string]string{
				"token": "The signed token from the rating link in the email. It names only its own " +
					"rating request.",
			},
		},
		"conversation-ratings.update": {
			Title: "Rate a conversation",
			Use: "Keeps the customer's answer to a rating request: a score, a comment, or both. Nobody " +
				"signs in; the signed link is the proof. A request older than a week can no longer be changed.",
			Returns: "the rating with the answer kept.",
			Params: map[string]string{
				"comment": "What the customer wrote about the conversation, up to 2000 characters.",
				"score":   "The customer's score, a whole number from 1 to 5, such as 4.",
				"token": "The signed token from the rating link in the email. It names only its own " +
					"rating request.",
			},
		},
	}
}

/*
unexposedDocsFromEmailChangesStore returns entries from email-changes.store on.
*/
func unexposedDocsFromEmailChangesStore() map[string]Doc {
	return map[string]Doc{
		"email-changes.store": {
			Title: "Request an email change",
			Use: "Starts moving the signed-in person's account to a new email address by mailing that " +
				"address a confirmation link, good for 24 hours. A new request ends the one before it. Only " +
				"that person's browser or app can call it, right after confirming their password or mailed " +
				"code.",
			Returns: "the pending email change with when its link runs out.",
			Params: map[string]string{
				"email": "The new email address, such as ada@newmail.com. Required; it must differ " +
					"from the current one and belong to no other account.",
			},
		},
		"email-changes.update": {
			Title: "Confirm an email change",
			Use: "Redeems the link mailed to the new address and moves the account to it, from any " +
				"device, signed in or not. An address another account took in the meantime drops the change.",
			Returns: "the confirmed email change.",
			Params: map[string]string{
				"change": "The id from the confirmation link, such as ott_4f9k2. It proves nothing " +
					"without the token.",
				"token": "The secret from the confirmation link, sent in the body. Required; five " +
					"wrong ones burn the link.",
			},
		},
		"email-verifications.store": {
			Title: "Resend the verification email",
			Use: "Mails the signed-in person a new link that proves their email address, at most once a " +
				"minute and six times an hour. An address already verified gets no mail. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "a message saying the link was sent, or that the address is already verified.",
		},
	}
}

/*
unexposedDocsFromEmailVerificationsUpdate returns entries from email-
verifications.update on.
*/
func unexposedDocsFromEmailVerificationsUpdate() map[string]Doc {
	return map[string]Doc{
		"email-verifications.update": {
			Title: "Verify an email address",
			Use: "Redeems a mailed verification link, from any device, signed in or not. A browser that " +
				"is not signed in to the account is asked to choose first, and sends claim true to take the " +
				"account over as the inbox's owner.",
			Returns: "the verified address with its status and the account's id.",
			Params: map[string]string{
				"claim": "true when the person holding the link did not create the account and takes " +
					"it over as they verify. false when left out.",
				"token": "The secret from the verification link, sent in the body. Required; five " +
					"wrong ones burn the link.",
				"verification": "The id from the verification link, such as ott_4f9k2. It proves " +
					"nothing without the token.",
			},
		},
		"help-images.store": {
			Title: "Upload a help article image",
			Use: "Uploads a picture to use in help center articles. It needs a multipart form with a PNG, " +
				"JPEG, WebP or GIF of at most 10 MB in the field named image, and the articles:write ability.",
			Returns: "the stored image.",
		},
	}
}
