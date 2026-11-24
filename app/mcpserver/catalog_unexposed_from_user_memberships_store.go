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

import "maps"

/*
unexposedDocsFromUserMembershipsStore returns entries from
user.memberships.store on.
*/
func unexposedDocsFromUserMembershipsStore() map[string]Doc {
	return map[string]Doc{
		"user.memberships.store": {
			Title: "Accept an invitation",
			Use: "Joins a workspace as the signed-in person by accepting an invitation link. The person's " +
				"email must be the invited one, and the invitation must still be pending. Joining any other " +
				"way is refused. Only a signed-in person's browser or app can call it, never an API token.",
			Returns: "the new membership with its workspace.",
			Params: map[string]string{
				"invitation_token": "The secret from the invitation link, up to 200 characters. " +
					"Required.",
				"workspace_id": "Never send this; a request that names a workspace is refused, since " +
					"joining takes an invitation.",
			},
		},
		"user.password.update": {
			Title: "Change my password",
			Use: "Sets a new password for the signed-in person and ends their other sessions and app " +
				"connections; the current session stays. A person with no password yet leaves current_password " +
				"out and must have confirmed with a mailed code a moment ago. Only a signed-in person's " +
				"browser or app can call it, never an API token.",
			Returns: "the outcome, with a count of the person's access tokens that remain.",
			Params: map[string]string{
				"current_password": "The password the account has now. Required when it has one; leave " +
					"out when the account has no password yet.",
				"password": "The new password, at least the server's minimum length and at most 256 " +
					"characters. A very common one, or the account's email or name, is refused.",
			},
		},
	}
}

/*
unexposedDocsFromUserSecurityEventsIndex returns entries from
user.security-events.index on.
*/
func unexposedDocsFromUserSecurityEventsIndex() map[string]Doc {
	return map[string]Doc{
		"user.security-events.index": {
			Title: "List my security log",
			Use: "Lists what happened to the signed-in person's own account, newest first: sign-ins, " +
				"failed tries, and changes to passwords, factors, tokens and sessions. Only a signed-in " +
				"person's browser or app can call it, never an API token.",
			Returns: "a page of the person's security events.",
			Params: map[string]string{
				"cursor": "Leave out for the first page. To read the next page, pass the next_cursor " +
					"the previous page returned.",
				"limit": "How many items to return in one page, from 1 to 100; 25 when left out.",
				"q":     "Accepted but not used; this list cannot be searched.",
			},
		},
		"user.update": {
			Title: "Update my profile",
			Use: "Changes the signed-in person's own name, language, time zone or personal preferences; a " +
				"field left out stays as it is. Only a signed-in person's browser or app can call it, never an " +
				"API token.",
			Returns: "the person's account as it now is.",
			Params: map[string]string{
				"locale": "The language the person reads dimmi in, one the server offers, such as en. " +
					"Send null to follow the workspace's language again.",
				"name": "The person's display name, up to 120 characters, such as Ada Lovelace. It " +
					"cannot be empty.",
				"preferences": "An object of personal preferences to change, one key at a time; null " +
					"returns a key to its default. The only key is send_with_enter (true or false), such as " +
					"{\"send_with_enter\": true}.",
				"timezone": "The person's time zone as an IANA name, such as Europe/Paris. Send null " +
					"to follow the workspace's time zone again.",
			},
		},
	}
}

/*
unexposedDocsFromUsersStore returns entries from users.store on.
*/
func unexposedDocsFromUsersStore() map[string]Doc {
	return map[string]Doc{
		"users.store": {
			Title: "Sign up for an account",
			Use: "Creates a new account and signs the person in, setting the session cookie in their " +
				"browser. It makes a first workspace when workspace_name is sent, or joins the inviting " +
				"workspace when invitation_token is sent. Only a browser or app can call it; a server that " +
				"closed sign-up refuses anyone without a pending invitation.",
			Returns: "the signed-in person with their workspaces.",
			Params: map[string]string{
				"captcha_token": "The answer the captcha widget gave the browser, needed only when the " +
					"server's sign-in options say a captcha is on.",
				"email": "The email address the account signs in with, such as ada@example.com. " +
					"Required, and it must not already have an account.",
				"invitation_token": "The secret from an invitation link, up to 200 characters. The " +
					"account then joins that workspace, and the email must be the invited one.",
				"name": "The person's full name, up to 80 characters, such as Ada Lovelace. Required.",
				"password": "The new password, at least the server's minimum length (see the sign-in " +
					"options) and at most 256 characters. A very common password, or the person's own email or " +
					"name, is refused.",
				"workspace_name": "Name of a first workspace to create and own, up to 80 characters, " +
					"such as Acme Support. Leave out to create none; it is not used when an invitation is " +
					"accepted.",
			},
		},
	}
}

/*
unexposedDocsFromWorkspaceAccessPolicyUpdate returns entries from
workspace.access-policy.update on.
*/
func unexposedDocsFromWorkspaceAccessPolicyUpdate() map[string]Doc {
	return map[string]Doc{
		"workspace.access-policy.update": {
			Title: "Change sign-in and access rules",
			Use: "Changes the workspace's access rules: allowed ways of signing in, Google domains, " +
				"session limits and IP allowlists. A key left out stays as it is and a list replaces the whole " +
				"stored list. It needs the permission to manage the security policy, and a change that would " +
				"lock the caller out is refused. Only a signed-in person's browser or app can call it, right " +
				"after confirming their password or mailed code.",
			Returns: "the access rules as they now are.",
			Params: map[string]string{
				"allowed_sign_in_methods": "The ways members may sign in, up to 20, from those the " +
					"server offers: password, magic_link, a provider such as google, or sso. It must keep the " +
					"caller's own way and one an active owner can use.",
				"api_ip_allowlist": "Up to 100 address ranges API tokens may be used from; the list " +
					"replaces the stored one. An empty list binds no token.",
				"api_ip_allowlist.cidr": "An IP address or range in CIDR form, up to 64 characters, " +
					"such as 198.51.100.7/32. Required in each entry.",
				"api_ip_allowlist.label": "A name for the range, up to 60 characters, such as Billing " +
					"server. Required in each entry.",
				"dashboard_ip_allowlist": "Up to 100 address ranges the dashboard may be used from; " +
					"the list replaces the stored one and must keep the caller's own address.",
				"dashboard_ip_allowlist.cidr": "An IP address or range in CIDR form, up to 64 " +
					"characters, such as 203.0.113.0/24. Required in each entry.",
				"dashboard_ip_allowlist.label": "A name for the range, up to 60 characters, such as " +
					"Paris office. Required in each entry.",
				"google_allowed_domains": "Up to 20 email domains whose Google accounts may sign in, " +
					"such as [\"acme.com\"]. It must keep the caller's own domain when they signed in through " +
					"Google.",
				"session_idle_timeout_minutes": "How long a session may sit unused before signing in " +
					"again, from 15 to 20160 minutes, such as 60. Send null to clear the limit.",
				"session_max_age_minutes": "How long a session lasts before signing in again, from 60 " +
					"to 43200 minutes, such as 1440. Send null to clear the limit.",
			},
		},
	}
}

/*
unexposedDocsFromWorkspaceDestroy returns entries from workspace.destroy on.
*/
func unexposedDocsFromWorkspaceDestroy() map[string]Doc {
	return map[string]Doc{
		"workspace.destroy": {
			Title: "Delete the workspace",
			Use: "Moves the current workspace to the trash, revokes its access tokens and withdraws its " +
				"pending invitations; it is purged for good when the grace period ends unless an owner " +
				"restores it. Only an owner's own browser or app can call it, after confirming who they are " +
				"recently.",
			Returns: "the workspace as it sits in the trash.",
			Params: map[string]string{
				"confirm_slug": "The workspace's own slug, typed to confirm, such as acme-support. " +
					"Required; case does not matter.",
			},
		},
		"workspace.logo.update": {
			Title: "Upload the workspace logo",
			Use: "Replaces the workspace's logo with an uploaded image. It needs a file sent as a " +
				"multipart form in the field named logo, and the settings:write permission.",
			Returns: "the workspace with its new logo.",
		},
		"workspaces.store": {
			Title: "Create a workspace",
			Use: "Creates a new workspace, with its own organization, owned by the signed-in person. Only " +
				"a signed-in person's browser or app can call it, never an API token.",
			Returns: "the new workspace.",
			Params: map[string]string{
				"name": "The workspace's name, up to 80 characters, such as Acme Support. Required.",
			},
		},
	}
}

/*
unexposedDocsFromWorkspacesUpdate returns entries from workspaces.update on.
*/
func unexposedDocsFromWorkspacesUpdate() map[string]Doc {
	return map[string]Doc{
		"workspaces.update": {
			Title: "Restore a workspace from trash",
			Use: "Brings a deleted workspace back from the trash; the only change it takes is trashed " +
				"false. Only an active owner's own browser or app can call it, after confirming who they are " +
				"recently, and only until the grace period ends. Every other setting changes through the " +
				"workspace update instead.",
			Returns: "the workspace as it now is.",
			Params: map[string]string{
				"gif_picker": "Never send this here; it is refused. Change it through the workspace " +
					"update.",
				"link_safety": "Never send this here; it is refused. Change link checks through the " +
					"workspace update.",
				"locale": "Never send this here; it is refused. Change the language through the " +
					"workspace update.",
				"name": "Never send this here; it is refused. Change the name through the workspace " +
					"update.",
				"require_two_factor": "Never send this here; it is refused. Change it through the " +
					"workspace update.",
				"show_viewers_on_list": "Never send this here; it is refused. Change it through the " +
					"workspace update.",
				"slug": "Never send this here; it is refused. Change the slug through the workspace " +
					"update.",
				"team_mentions_enabled": "Never send this here; it is refused. Change it through the " +
					"workspace update.",
				"test_mode": "Never send this here; it is refused. Change test mode through the " +
					"workspace update.",
				"timezone": "Never send this here; it is refused. Change the time zone through the " +
					"workspace update.",
				"trashed": "Must be false, which restores the workspace. Required; true is refused.",
				"undo_send_seconds": "Never send this here; it is refused. Change it through the " +
					"workspace update.",
				"workspace": "The id of the workspace to restore, such as ws_4f9k2.",
			},
		},
	}
}

/*
unexposedParamsFromClientSecret explains the parameters and fields of
sso-connections.store.
*/
func unexposedParamsFromClientSecret() map[string]string {
	out := map[string]string{
		"client_secret": "The OpenID Connect client secret, up to 2000 characters. Required " +
			"for oidc; it is written only and never read back.",
		"config": "The connection's settings: the idp_ fields and name_id_format for SAML; " +
			"issuer, client_id and scopes for OpenID Connect; attribute_map for both.",
		"config.attribute_map": "The names of the identity provider's attributes that carry " +
			"each part of a person.",
		"config.attribute_map.display_name": "The attribute that carries the person's full " +
			"name, up to 200 characters, such as name.",
		"config.attribute_map.email": "The attribute that carries the person's email address, " +
			"up to 200 characters, such as email.",
		"config.attribute_map.first_name": "The attribute that carries the person's first " +
			"name, up to 200 characters, such as given_name.",
		"config.attribute_map.groups": "The attribute that carries the person's groups, up to " +
			"200 characters, such as groups. Groups are what group_role_map reads.",
		"config.attribute_map.last_name": "The attribute that carries the person's last name, " +
			"up to 200 characters, such as family_name.",
		"config.client_id": "OpenID Connect: the client id the identity provider gave, up to " +
			"500 characters. Required for oidc.",
	}
	maps.Copy(out, unexposedParams1More())

	return out
}

/*
unexposedParams1More explains the rest of them.
*/
func unexposedParams1More() map[string]string {
	return map[string]string{
		"config.idp_certificates": "SAML: up to 5 of the identity provider's signing " +
			"certificates, each as text of at most 10000 characters.",
		"config.idp_entity_id": "SAML: the identity provider's entity id, up to 500 " +
			"characters, such as http://www.okta.com/exk123.",
		"config.idp_metadata_url": "SAML: the URL of the identity provider's metadata, such as " +
			"https://acme.okta.com/app/abc/sso/saml/metadata. The entity id, sign-in URL and certificates " +
			"are then kept up to date from it.",
		"config.idp_sso_url": "SAML: the identity provider's sign-in URL, such as " +
			"https://acme.okta.com/app/abc/sso/saml.",
		"config.issuer": "OpenID Connect: the issuer URL, such as https://accounts.acme.com. " +
			"Required for oidc.",
		"config.name_id_format": "SAML: the name id format to ask for, up to 200 characters, " +
			"such as urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress.",
		"config.scopes": "OpenID Connect: up to 20 scopes to ask for, such as [\"openid\", " +
			"\"email\", \"profile\"].",
		"name":     "A name for the connection, up to 80 characters, such as Okta. Required.",
		"protocol": "saml or oidc (OpenID Connect). Required.",
	}
}

/*
unexposedParamsFromClientSecretAndConfig explains the parameters and fields of
sso-connections.update.
*/
func unexposedParamsFromClientSecretAndConfig() map[string]string {
	out := map[string]string{
		"client_secret": "A new OpenID Connect client secret, up to 2000 characters. It is " +
			"written only and never read back.",
		"config": "The connection's settings: the idp_ fields and name_id_format for SAML; " +
			"issuer, client_id and scopes for OpenID Connect; attribute_map for both.",
		"config.attribute_map": "The names of the identity provider's attributes that carry " +
			"each part of a person.",
		"config.attribute_map.display_name": "The attribute that carries the person's full " +
			"name, up to 200 characters, such as name.",
		"config.attribute_map.email": "The attribute that carries the person's email address, " +
			"up to 200 characters, such as email.",
		"config.attribute_map.first_name": "The attribute that carries the person's first " +
			"name, up to 200 characters, such as given_name.",
		"config.attribute_map.groups": "The attribute that carries the person's groups, up to " +
			"200 characters, such as groups. Groups are what group_role_map reads.",
		"config.attribute_map.last_name": "The attribute that carries the person's last name, " +
			"up to 200 characters, such as family_name.",
		"config.client_id": "OpenID Connect: the client id the identity provider gave, up to " +
			"500 characters.",
		"config.idp_certificates": "SAML: up to 5 of the identity provider's signing " +
			"certificates, each as text of at most 10000 characters.",
		"config.idp_entity_id": "SAML: the identity provider's entity id, up to 500 " +
			"characters, such as http://www.okta.com/exk123.",
		"config.idp_metadata_url": "SAML: the URL of the identity provider's metadata, such as " +
			"https://acme.okta.com/app/abc/sso/saml/metadata. The entity id, sign-in URL and certificates " +
			"are then kept up to date from it.",
		"config.idp_sso_url": "SAML: the identity provider's sign-in URL, such as " +
			"https://acme.okta.com/app/abc/sso/saml.",
		"config.issuer": "OpenID Connect: the issuer URL, such as https://accounts.acme.com.",
		"config.name_id_format": "SAML: the name id format to ask for, up to 200 characters, " +
			"such as urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress.",
	}
	maps.Copy(out, unexposedParams2More())

	return out
}

/*
unexposedParams2More explains the rest of them.
*/
func unexposedParams2More() map[string]string {
	return map[string]string{
		"config.scopes": "OpenID Connect: up to 20 scopes to ask for, such as [\"openid\", " +
			"\"email\", \"profile\"].",
		"confirm_owners_can_sign_in": "true as the caller's word that owners can sign in " +
			"through a connection tested within the last day; needed to enforce when the caller did not " +
			"sign in through it.",
		"connection": "The connection's id from the list of connections, such as ssoc_4f9k2.",
		"default_custom_role_id": "The id of a custom role of the workspace that new members " +
			"start with, such as rol_4f9k2. Send null to clear it.",
		"default_role": "The built-in role new members start with: admin or member.",
		"default_team_ids": "The ids of the teams new members join, such as [\"team_4f9k2\"]. " +
			"The list replaces the stored one.",
		"enforced": "true to make members of the routed domains sign in through this " +
			"connection. It needs an active connection, and a caller who signed in through it or confirms " +
			"a test from the last day.",
		"group_role_map": "Up to 50 rules that give the members of an identity provider group " +
			"a role. The list replaces the stored one.",
		"group_role_map.custom_role_id": "The id of a custom role the group's members get, " +
			"such as rol_4f9k2.",
		"group_role_map.group": "The identity provider group's name, up to 200 characters, " +
			"such as support-admins. Required in each rule.",
		"group_role_map.role": "The built-in role the group's members get: admin or member.",
		"jit_enabled": "true to let a person who signs in through the connection for the first " +
			"time become a member on the spot; false to stop that.",
		"name": "A new name, 1 to 80 characters, such as Okta.",
		"status": "active to turn the connection on (it needs a passed test and a verified " +
			"domain routed to it) or disabled to turn an active one off, which also stops enforcing. " +
			"Nothing else can be set by hand.",
	}
}
