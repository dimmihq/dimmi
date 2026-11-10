/*
|--------------------------------------------------------------------------
| What The Slack Channel Claims
|--------------------------------------------------------------------------
|
| The channel's own share of Slack's deliveries: messages and their
| edits, the life of channels and of the install, the trigger emoji,
| the answer controls and the shortcut that opens a conversation, and
| the team's own work in Slack. It
| claims them last, after any feature registered for a narrower kind,
| so a feature that takes a direct message from a teammate gets it
| first and the channel only sees what nobody else wanted.
|
*/

package channels

import (
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
)

/*
Claim registers the channel's handlers on the router.
*/
func (s *Slack) Claim(r *Router) {
	s.claimChannelsSeen(r)
	r.OnEvent("message", onConnection(s.onMessage))
	r.OnEvent("member_joined_channel", onConnection(s.onMemberJoined))

	for _, kind := range []string{
		"channel_left", "group_left", "channel_archive", "group_archive", "channel_deleted", "group_deleted",
	} {
		r.OnEvent(kind, onConnection(s.onRoomGone))
	}

	r.OnEvent("channel_rename", onConnection(s.onRenamed))
	r.OnEvent("group_rename", onConnection(s.onRenamed))
	r.OnEvent("channel_id_changed", onConnection(s.onMoved))
	r.OnEvent("channel_shared", onConnection(s.onShared))
	r.OnEvent("channel_unshared", onConnection(s.onShared))
	r.OnEvent("app_uninstalled", s.onRevoked)
	r.OnEvent("tokens_revoked", s.onRevoked)
	r.OnEvent("reaction_added", onConnection(s.onTriggerEmoji))
	r.OnAction(slackdriver.ControlActionPrefix(), pressOnConnection(s.onControl))
	r.OnShortcut(slackdriver.ShortcutCreate, pressOnConnection(s.onCreateShortcut))
	s.claimTeam(r)
}
