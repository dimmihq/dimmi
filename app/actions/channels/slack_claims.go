/*
|--------------------------------------------------------------------------
| What The Slack Channel Claims
|--------------------------------------------------------------------------
|
| The channel's own share of Slack's deliveries: messages and their
| edits, the life of channels and of the install, the trigger emoji,
| the answer controls and the shortcut that opens a conversation. It
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
	r.OnEvent("message", s.onMessage)
	r.OnEvent("member_joined_channel", s.onMemberJoined)

	for _, kind := range []string{
		"channel_left", "group_left", "channel_archive", "group_archive", "channel_deleted", "group_deleted",
	} {
		r.OnEvent(kind, s.onRoomGone)
	}

	r.OnEvent("channel_rename", s.onRenamed)
	r.OnEvent("group_rename", s.onRenamed)
	r.OnEvent("channel_id_changed", s.onMoved)
	r.OnEvent("channel_shared", s.onShared)
	r.OnEvent("channel_unshared", s.onShared)
	r.OnEvent("app_uninstalled", s.onRevoked)
	r.OnEvent("tokens_revoked", s.onRevoked)
	r.OnEvent("reaction_added", s.onTriggerEmoji)
	r.OnAction(slackdriver.ControlActionPrefix(), s.onControl)
	r.OnShortcut(slackdriver.ShortcutCreate, s.onCreateShortcut)
}
