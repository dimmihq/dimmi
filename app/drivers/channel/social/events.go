/*
|--------------------------------------------------------------------------
| Events
|--------------------------------------------------------------------------
|
| What an event holds. A message is text, files, or something only the
| app can show, such as a view-once picture; it may answer an earlier
| message or an Instagram story, and may be a notice that an earlier one
| was unsent. Files arrive as addresses on Meta's servers that stop
| working, so the inbox copies them at once, except a story someone
| mentioned the account in: Meta forbids keeping those, so only the
| address is kept.
|
*/

package social

/*
Message is a message in a thread. Echo marks one the account itself
sent, from this app, Meta's own inbox or another app, named by App.
Deleted marks a notice that the message with the ID was unsent.
*/
type Message struct {
	ID          string       `json:"mid"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
	ReplyTo     *ReplyTo     `json:"reply_to"`
	QuickReply  *struct {
		Payload string `json:"payload"`
	} `json:"quick_reply"`
	Echo        bool     `json:"is_echo"`
	App         flexible `json:"app_id"`
	Deleted     bool     `json:"is_deleted"`
	Unsupported bool     `json:"is_unsupported"`
	Notice      string   `json:"admin_text"`
}

/*
Attachment is one thing a message carries beside its text: a file, a
sticker, a shared post or reel, a link, or a story mention.
*/
type Attachment struct {
	Type    string `json:"type"`
	Payload *struct {
		URL     string   `json:"url"`
		Title   string   `json:"title"`
		Sticker flexible `json:"sticker_id"`
	} `json:"payload"`
}

/*
Address is where Meta serves the attachment, empty when it has none.
*/
func (a Attachment) Address() string {
	if a.Payload == nil {
		return ""
	}

	return a.Payload.URL
}

/*
Title is what the shared post or link is called, empty when unnamed.
*/
func (a Attachment) Title() string {
	if a.Payload == nil {
		return ""
	}

	return a.Payload.Title
}

/*
File reports whether the attachment is a file the inbox copies and
shows: a picture, a video, audio, a document or a sticker.
*/
func (a Attachment) File() bool {
	switch a.Type {
	case "image", "video", "audio", "file", "sticker":
		return a.Address() != ""
	default:
		return false
	}
}

/*
Shared reports whether the attachment is a post, reel or link someone
shared, shown as a link.
*/
func (a Attachment) Shared() bool {
	switch a.Type {
	case "share", "post", "ig_post", "reel", "ig_reel", "fallback", "story", "ig_story", "media":
		return a.Address() != ""
	default:
		return false
	}
}

/*
StoryMention reports whether the attachment is a story the customer
mentioned the account in.
*/
func (a Attachment) StoryMention() bool {
	return a.Type == "story_mention"
}

/*
ReplyTo is what a message answers: an earlier message, or a story of
the account's.
*/
type ReplyTo struct {
	ID    string `json:"mid"`
	Story *struct {
		ID  flexible `json:"id"`
		URL string   `json:"url"`
	} `json:"story"`
}
