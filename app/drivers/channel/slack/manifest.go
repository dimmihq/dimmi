/*
|--------------------------------------------------------------------------
| Slack App Manifest
|--------------------------------------------------------------------------
|
| A self-hosted install connects its own Slack app, created from a
| manifest the product writes, so the app asks for exactly what every
| feature needs from day one. Interactivity, reactions, mentions and
| commands are in it even where little uses them yet: adding a scope
| later forces every such app to be edited and installed again. Over a
| socket the app needs no public URL, so its request URLs are left out.
|
*/

package slack

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
)

/*
manifestNameLimit is the longest name Slack accepts for an app.
*/
const manifestNameLimit = 35

/*
BotScopes are the bot scopes the channel needs.
*/
func BotScopes() []string {
	return []string{
		"channels:history", "groups:history", "im:history", "channels:read", "groups:read", "im:read", "im:write",
		"chat:write", "chat:write.customize", "chat:write.public", "files:read", "files:write", "users:read",
		"users:read.email", "reactions:read", "reactions:write", "team:read", "channels:join", "app_mentions:read",
		"commands", "assistant:write",
	}
}

/*
UserScopes are the scopes a teammate grants to reply as themselves.
*/
func UserScopes() []string {
	return []string{"chat:write", "reactions:write"}
}

/*
BotEvents are the events the channel subscribes to.
*/
func BotEvents() []string {
	return []string{
		"message.channels", "message.groups", "message.im", "app_uninstalled", "tokens_revoked",
		"member_joined_channel", "channel_left", "group_left", "channel_archive", "group_archive",
		"channel_deleted", "group_deleted", "channel_rename", "group_rename", "channel_id_changed",
		"channel_shared", "channel_unshared", "app_mention", "reaction_added", "reaction_removed",
		"channel_created", "assistant_thread_started", "app_home_opened", "agent_session_stopped",
	}
}

/*
ManifestOptions shape a manifest. AppURL is the install's address and
ConnectionID the public id of the connection the app feeds. Socket
leaves the request URLs out. BotScopes and UserScopes default to the
channel's own.
*/
type ManifestOptions struct {
	Name         string
	BotName      string
	AppURL       string
	ConnectionID string
	Socket       bool
	BotScopes    []string
	UserScopes   []string
}

/*
Manifest returns the app manifest as Slack reads it.
*/
func Manifest(opts ManifestOptions) map[string]any {
	name := Truncate(cmp.Or(opts.Name, "dimmi Support"), manifestNameLimit)
	settings := map[string]any{
		"event_subscriptions":    map[string]any{"bot_events": BotEvents()},
		"interactivity":          map[string]any{"is_enabled": true},
		"socket_mode_enabled":    opts.Socket,
		"token_rotation_enabled": false,
		"org_deploy_enabled":     false,
	}

	if !opts.Socket {
		settings["event_subscriptions"] = map[string]any{
			"bot_events": BotEvents(), "request_url": opts.AppURL + "/webhooks/slack/events/" + opts.ConnectionID,
		}
		settings["interactivity"] = map[string]any{
			"is_enabled": true, "request_url": opts.AppURL + "/webhooks/slack/interactions/" + opts.ConnectionID,
		}
	}

	return map[string]any{
		"_metadata":           map[string]any{"major_version": 2, "minor_version": 1},
		"display_information": map[string]any{"name": name, "description": "Customer support by dimmi"},
		"features": map[string]any{
			"app_home": map[string]any{
				"home_tab_enabled": true, "messages_tab_enabled": true, "messages_tab_read_only_enabled": false,
			},
			"bot_user": map[string]any{"display_name": cmp.Or(opts.BotName, name), "always_online": true},
			"shortcuts": []map[string]any{{
				"name": "Track in dimmi", "type": "message", "callback_id": ShortcutCreate,
				"description": "Track this message as a support conversation",
			}},
		},
		"oauth_config": map[string]any{
			"scopes": map[string]any{
				"bot":  orDefault(opts.BotScopes, BotScopes()),
				"user": orDefault(opts.UserScopes, UserScopes()),
			},
			"redirect_urls": []string{opts.AppURL + "/integrations/slack/callback"},
		},
		"settings": settings,
	}
}

/*
CreateAppURL returns the address that opens Slack's app creation with
the manifest filled in.
*/
func CreateAppURL(manifest map[string]any) (string, error) {
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode slack manifest: %w", err)
	}

	return "https://api.slack.com/apps?new_app=1&manifest_json=" + url.QueryEscape(string(encoded)), nil
}

/*
orDefault returns the list, or the fallback when it is empty.
*/
func orDefault(list, fallback []string) []string {
	if len(list) == 0 {
		return fallback
	}

	return list
}
