/*
|--------------------------------------------------------------------------
| Tool Catalog: Integrations
|--------------------------------------------------------------------------
|
| The entries for the tools a workspace has connected, the Slack apps
| it answers customers through and their channels, and the Slack places
| the team hears about its work in. No token ever comes back.
|
*/

package mcpserver

import "maps"

/*
integrationDocs returns the entries for installations, Slack
connections and notification destinations.
*/
func integrationDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		installationDocs(), slackConnectionDocs(), slackRoomDocs(), destinationDocs(), destinationChangeDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
installationDocs returns the entries for the tools a workspace can
connect and has connected.
*/
func installationDocs() map[string]Doc {
	return map[string]Doc{
		"integration-providers.index": {
			Title: "List integration providers",
			Use: "Lists the tools this install can connect, whether each installs through its shared app, and " +
				"the scopes each feature asks for. Installing one needs the person at a browser.",
			Returns: "every provider with its features and their scopes.",
		},
		"integrations.index": {
			Title:   "List integrations",
			Use:     "Lists the tools the workspace connected and the health of each.",
			Returns: "every installation with its provider, status, scopes, what uses it and the person's own grant.",
			Params:  map[string]string{"filter[provider]": "Only installations of this provider, such as slack."},
		},
		"integrations.show": {
			Title:   "Get an integration",
			Use:     "Reads one connected tool: its health, scopes, the scopes features still miss and what uses it.",
			Returns: "the installation with its features, channel_ids and last error.",
		},
		"integrations.destroy": {
			Title: "Disconnect an integration",
			Use: "Disconnects a tool: its tokens are given back and wiped, every teammate's grant on it is " +
				"revoked, and the channels and destinations using it stop at once. Connecting it again needs " +
				"the person at a browser.",
			Returns: "nothing once it is disconnected.",
		},
		"integrations.slack-channels.index": {
			Title:   "List an integration's Slack channels",
			Use:     "Lists the Slack channels a Slack installation's bot can post in, to pick a destination from.",
			Returns: "the channels with their id and name.",
			Next:    []string{"dimmi_notification_destinations_create: post alerts in one"},
		},
	}
}

/*
slackConnectionDocs returns the entries for connecting a workspace's
own Slack app.
*/
func slackConnectionDocs() map[string]Doc {
	return map[string]Doc{
		"channels.slack.store": {
			Title: "Connect a Slack app",
			Use: "Connects the workspace's own Slack app as a channel customers write in: at once when its " +
				"secrets come along, or waiting for them, with the manifest that creates the app. Secrets are " +
				"checked with Slack. Only use secrets the person gave you for this.",
			Returns: "the connection with how to set its app up; secrets never come back.",
			Next:    []string{"dimmi_channels_credentials_create: add the secrets later"},
			Params: slackAppParams(map[string]string{
				"name":     "A name for the team, up to 80 characters.",
				"inbox_id": "Inbox (inbox_...) its conversations land in. Find it with dimmi_inboxes_list.",
				"inbound_transport": "webhook (Slack calls dimmi) or socket (dimmi keeps a connection open, " +
					"which needs app_token).",
			}),
		},
		"channels.credentials.store": {
			Title:   "Set a Slack app's secrets",
			Use:     "Sets or rotates the secrets of a Slack connection. They are checked with Slack first.",
			Returns: "the connection; secrets never come back.",
			Params:  slackAppParams(map[string]string{}),
		},
	}
}

/*
slackAppParams adds the explanations of a Slack app's pasted secrets to
params. They are listed as pairs so no literal reads as a stored secret.
*/
func slackAppParams(params map[string]string) map[string]string {
	for _, p := range [][2]string{
		{"bot_token", "The app's bot token, starting with xoxb-."},
		{"signing_secret", "The app's signing secret, needed with webhook."},
		{"app_token", "The app-level token, starting with xapp-, needed with socket."},
	} {
		params[p[0]] = p[1]
	}

	return params
}

/*
slackRoomDocs returns the entries for the Slack channels a connection
watches.
*/
func slackRoomDocs() map[string]Doc {
	return map[string]Doc{
		"channels.rooms.index": {
			Title: "List a Slack connection's channels",
			Use: "Lists the Slack channels a connection watches, removed ones included, with who counts as the " +
				"customer in each and when a message becomes a conversation.",
			Returns: "the channels with their mode, trigger, settings and status.",
		},
		"channels.rooms.update": {
			Title: "Change a Slack channel's handling",
			Use: "Changes who counts as the customer in one Slack channel, when a message there becomes a " +
				"conversation, and how the agent answers. Only what is sent changes; a setting sent as null " +
				"goes back to its default.",
			Returns: "the channel as it now is.",
			Params: map[string]string{
				"mode": "shared (the other organization's people are customers), community (everyone but " +
					"teammates), helpdesk (the team's own colleagues) or internal (nobody).",
				"trigger": "When a top-level message becomes a conversation: author_window, every_message, " +
					"emoji, mention, ai, auto or off.",
				"settings": "agent_mode (auto or draft), agent_delay_seconds and auto_reply_after_seconds (0 to " +
					"900), agent_delay_when (always or inside_hours), triage_channel_id, auto_join and " +
					"request_sensitivity (conservative or aggressive).",
			},
		},
	}
}
