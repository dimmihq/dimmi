/*
|--------------------------------------------------------------------------
| Slack Events
|--------------------------------------------------------------------------
|
| One event inside an Events API delivery: a message, an edit or a
| deletion of one, a reaction, a change to a channel, an uninstall. A
| channel arrives as an id in most events and as an object in a few, so
| both are read into one id.
|
*/

package slack

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
)

/*
Event is one Slack event. Message is the new version of an edited
message and Previous the old one; DeletedTS names a deleted message.
Item is what a reaction was added to and ItemUser who wrote it, Tokens
the tokens a revocation names, and Tab the App Home tab a user opened.
*/
type Event struct {
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype"`
	User          string          `json:"user"`
	BotID         string          `json:"bot_id"`
	Text          string          `json:"text"`
	TS            string          `json:"ts"`
	ThreadTS      string          `json:"thread_ts"`
	EventTS       string          `json:"event_ts"`
	ChannelRaw    json.RawMessage `json:"channel"`
	ChannelType   string          `json:"channel_type"`
	Team          string          `json:"team"`
	UserTeam      string          `json:"user_team"`
	SourceTeam    string          `json:"source_team"`
	Blocks        json.RawMessage `json:"blocks"`
	Files         []File          `json:"files"`
	Message       *Event          `json:"message"`
	Previous      *Event          `json:"previous_message"`
	DeletedTS     string          `json:"deleted_ts"`
	Reaction      string          `json:"reaction"`
	Item          Item            `json:"item"`
	OldChannelID  string          `json:"old_channel_id"`
	NewChannelID  string          `json:"new_channel_id"`
	IsExtShared   bool            `json:"is_ext_shared"`
	Tokens        RevokedTokens   `json:"tokens"`
	ConnectedTeam string          `json:"connected_team_id"`
	ItemUser      string          `json:"item_user"`
	Tab           string          `json:"tab"`
}

/*
File is a file shared with a message.
*/
type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Mimetype    string `json:"mimetype"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"url_private_download"`
	PrivateURL  string `json:"url_private"`
}

/*
Item is the message a reaction is on.
*/
type Item struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

/*
RevokedTokens lists the users and bots whose tokens were revoked.
*/
type RevokedTokens struct {
	OAuth []string `json:"oauth"`
	Bot   []string `json:"bot"`
}

/*
ParseEvent reads the event of an envelope.
*/
func ParseEvent(raw []byte) (Event, error) {
	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return ev, fmt.Errorf("read slack event: %w", err)
	}

	return ev, nil
}

/*
ChannelID returns the channel the event happened in, whether Slack sent
it as an id or as an object.
*/
func (e Event) ChannelID() string {
	var id string
	if json.Unmarshal(e.ChannelRaw, &id) == nil {
		return id
	}

	var obj struct {
		ID string `json:"id"`
	}

	if json.Unmarshal(e.ChannelRaw, &obj) == nil {
		return obj.ID
	}

	return ""
}

/*
ChannelName returns the channel's new name in a rename event.
*/
func (e Event) ChannelName() string {
	var obj struct {
		Name string `json:"name"`
	}

	if json.Unmarshal(e.ChannelRaw, &obj) == nil {
		return obj.Name
	}

	return ""
}

/*
InThread reports whether the message is a reply inside a thread rather
than a top-level message.
*/
func (e Event) InThread() bool {
	return e.ThreadTS != "" && e.ThreadTS != e.TS
}

/*
RootTS returns the timestamp of the thread the message belongs to: its
own when it is top level.
*/
func (e Event) RootTS() string {
	if e.ThreadTS != "" {
		return e.ThreadTS
	}

	return e.TS
}

/*
mentionPattern matches a mention of a user in Slack's markup.
*/
var mentionPattern = regexp.MustCompile(`<@([UW][A-Z0-9]+)(?:\|[^>]*)?>`)

/*
Mentions returns the users the message mentions, in order, once each.
*/
func (e Event) Mentions() []string {
	var out []string

	for _, match := range mentionPattern.FindAllStringSubmatch(e.Text, -1) {
		if !slices.Contains(out, match[1]) {
			out = append(out, match[1])
		}
	}

	return out
}
