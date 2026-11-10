/*
|--------------------------------------------------------------------------
| The Team's Slack Threads
|--------------------------------------------------------------------------
|
| A triage card and a discussion each open a thread that belongs to the
| team. What a linked teammate writes there becomes a note on the
| conversation, never seen by the customer; a message starting with >>
| is the one way out, posted to the customer's thread as that teammate.
| Someone who writes a note under a triage card is reminded of >> once a
| week. Notes written in dimmi come back into every such thread, and a
| note that came from Slack never goes back to it.
|
| In a channel only the team reads, pasting a dimmi conversation link
| offers to start a discussion of it right there.
|
| Nothing a customer room's rules say applies inside these threads, and
| nothing else in an internal room is ever stored.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The prefix that sends a thread message to the customer, and how often a
teammate is reminded of it.
*/
const (
	customerPrefix = ">>"
	tipEvery       = 7 * 24 * time.Hour
)

/*
conversationLinkPattern finds a dimmi conversation's public id in a
pasted link.
*/
var conversationLinkPattern = regexp.MustCompile(`/d/inbox/[a-z0-9_-]+/(conv_[A-Za-z0-9]+)`)

/*
teamMessage reads a message in the team's own places: a thread under a
triage card or a discussion, wherever it is, and anything in an
active internal room. Done reports that the message belonged there,
handled or not, so no customer rule reads it.
*/
func (s *Slack) teamMessage(ctx context.Context, ev SlackEvent, room models.ChannelRoom) (
	handled, done bool, err error,
) {
	if ev.Event.InThread() {
		post, found, err := s.postAt(ctx, ev.Connection, ev.Event.ChannelID(), ev.Event.ThreadTS)
		if err != nil || found {
			handled, err = s.teamThreadReply(ctx, ev, post)

			return handled, true, err
		}
	}

	if room.Status != string(enums.RoomStatusActive) || room.Mode != string(enums.RoomModeInternal) {
		return false, false, nil
	}

	return s.offerDiscussion(ctx, ev), true, nil
}

/*
postAt returns the team's post at the message, reporting false when it
is no post of the team.
*/
func (s *Slack) postAt(ctx context.Context, conn models.ChannelConnection, channel, ts string) (
	models.ChannelConversationPost, bool, error,
) {
	var post models.ChannelConversationPost

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		post, err = q.FindChannelConversationPost(ctx, models.FindChannelConversationPostParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalChannel: channel, ExternalTs: ts,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return post, false, nil
	}

	if err != nil {
		return post, false, fmt.Errorf("find team post: %w", err)
	}

	return post, post.Kind != "draft", nil
}

/*
teamThreadReply records a teammate's message in the team's thread: a
note, or with >> a reply to the customer.
*/
func (s *Slack) teamThreadReply(ctx context.Context, ev SlackEvent, post models.ChannelConversationPost) (
	bool, error,
) {
	a, err := s.classify(ctx, ev.Connection, nil, ev.Event.User)
	if err != nil || a.kind == authorIgnored {
		return false, err
	}

	if a.memberID == nil {
		s.promptLink(ctx, ev.Connection, ev.Event, a)

		return false, nil
	}

	conv, err := s.findConversation(ctx, ev.Connection.WorkspaceID, post.ConversationID)
	if err != nil {
		return false, ignoreMissing(err)
	}

	who := teammate{memberID: *a.memberID, name: a.memberName, userID: ev.Event.User}
	text := slackdriver.ToMarkdown(ev.Event.Text, ev.Event.Blocks, s.names(ctx, ev.Connection))

	if rest, ok := strings.CutPrefix(strings.TrimSpace(text), customerPrefix); ok {
		reply := teamReply{conv: conv, body: strings.TrimSpace(rest), meta: fromSlack(ev.Event)}

		return true, s.replyAs(ctx, who, reply)
	}

	if post.Kind == "triage" {
		s.tip(ctx, ev, a)
	}

	return true, s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		_, _, err := w.Append(ctx, conv, conversations.NewMessage{
			Type: string(enums.MessageTypeNote), Body: text, BodyFormat: new(string(enums.BodyFormatMarkdown)),
			Metadata: fromSlack(ev.Event),
		})

		return err
	})
}

/*
fromSlack marks what came from a Slack message, so it never echoes
back into Slack.
*/
func fromSlack(ev slackdriver.Event) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"slack": map[string]string{"channel": ev.ChannelID(), "ts": ev.TS}})

	return raw
}

/*
tip reminds the teammate, once a week, how to reach the customer.
*/
func (s *Slack) tip(ctx context.Context, ev SlackEvent, a author) {
	first, err := s.Cache.Add(ctx, "slack:triage-tip:"+ev.Connection.ID.String()+":"+a.identity, []byte{1}, tipEvery)
	if err != nil || !first {
		return
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return
	}

	_ = api.Ephemeral(ctx, slackdriver.Notice{
		Channel: ev.Event.ChannelID(), User: ev.Event.User, ThreadTS: ev.Event.ThreadTS,
		Text: "Saved as a note only the team sees. Start with >> to reply to the customer.",
	})
}

/*
offerDiscussion offers a teammate who pasted a dimmi conversation link
to discuss it in the thread.
*/
func (s *Slack) offerDiscussion(ctx context.Context, ev SlackEvent) bool {
	match := conversationLinkPattern.FindStringSubmatch(ev.Event.Text)
	if match == nil || !strings.Contains(ev.Event.Text, s.AppURL) {
		return false
	}

	if _, ok, err := s.conversationByPublicID(ctx, ev.Connection, match[1]); err != nil || !ok {
		return false
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return false
	}

	offer := slackdriver.DiscussionOffer(ev.Event.ChannelID(), ev.Event.RootTS(), match[1])

	return api.EphemeralTeamCard(ctx, ev.Event.User, offer) == nil
}

/*
onDiscussionStart starts the discussion a teammate accepted.
*/
func (s *Slack) onDiscussionStart(ctx context.Context, in SlackInteraction) (bool, error) {
	publicID, _, ok := slackdriver.ParseTeamActionID(slackdriver.DiscussionPrefix(), in.Action.ActionID)
	if !ok {
		return false, nil
	}

	conv, ok, err := s.conversationByPublicID(ctx, in.Connection, publicID)
	if err != nil || !ok {
		return ok, err
	}

	if _, allowed, err := s.teammateFor(ctx, in, conv); err != nil || !allowed {
		return true, err
	}

	return true, s.openDiscussion(ctx, in.Connection, conv, in.Interaction.ChannelID())
}

/*
StartDiscussion opens a discussion of the conversation in a Slack
channel of its connection: a summary posted there, whose thread's
messages become notes.

Errors: [exceptions.ErrForbidden] without conversations:write, and
[exceptions.ErrNotFound] for a conversation that is not on Slack.
*/
func (s *Slack) StartDiscussion(ctx context.Context, m authz.Membership, conversationID ids.UUID,
	channel string,
) error {
	if !m.Can("conversations:write") {
		return exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conv, err := s.findConversation(ctx, m.Workspace.ID, conversationID)
	if err != nil {
		return notFound(err)
	}

	conn, err := s.connection(ctx, m.Workspace.ID, conv.ChannelConnectionID)
	if err != nil || conn.Type != slackdriver.Type {
		return notFound(errors.Join(err, exceptions.ErrNotFound))
	}

	return s.openDiscussion(ctx, conn, conv, channel)
}

/*
notFound turns a missing row into the not found error.
*/
func notFound(err error) error {
	if errors.Is(err, database.ErrNotFound) || errors.Is(err, exceptions.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return err
}

/*
openDiscussion posts the conversation's summary in the channel and
remembers the thread.
*/
func (s *Slack) openDiscussion(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	channel string,
) error {
	card, err := s.triageCard(ctx, conv, models.ChannelRoom{})
	if err != nil {
		return err
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	ts, err := api.PostTeamCard(ctx, slackdriver.DiscussionCard(channel, card.Customer+": "+card.Excerpt,
		s.conversationLink(conv.ID)))
	if err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.CreateChannelConversationPost(ctx, models.CreateChannelConversationPostParams{
			WorkspaceID: conv.WorkspaceID, ChannelConnectionID: conn.ID, ConversationID: conv.ID,
			Kind: "discussion", ExternalChannel: channel, ExternalTs: ts, Now: s.Clock.Now(),
		})

		return err
	})
}

/*
echoNote posts a note written in dimmi into the team's threads of its
conversation, under its writer's name.
*/
func (s *Slack) echoNote(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	messageID *ids.UUID,
) error {
	if messageID == nil {
		return nil
	}

	echo, err := s.noteEcho(ctx, conv, *messageID)
	if err != nil || len(echo.posts) == 0 {
		return err
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	for _, post := range echo.posts {
		_, err := api.PostText(ctx, slackdriver.TeamText{
			Channel: post.ExternalChannel, ThreadTS: post.ExternalTs, Markdown: body(echo.note),
			Name: echo.author + " (note)",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
echoed is a note on its way into Slack: the note, its writer's name and
the threads it goes to.
*/
type echoed struct {
	note   models.Message
	author string
	posts  []models.ChannelConversationPost
}

/*
noteEcho returns the note and the threads it goes to, none for a note
that came from Slack.
*/
func (s *Slack) noteEcho(ctx context.Context, conv models.Conversation, messageID ids.UUID) (echoed, error) {
	var out echoed

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		out.note, err = q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
			WorkspaceID: conv.WorkspaceID, ID: messageID,
		})
		if err != nil || out.note.Type != string(enums.MessageTypeNote) || cameFromSlack(out.note) {
			return err
		}

		out.author = "A teammate"
		if out.note.AuthorID != nil {
			if member, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
				WorkspaceID: conv.WorkspaceID, ID: *out.note.AuthorID,
			}); err == nil {
				out.author = member.User.Name
			}
		}

		out.posts, err = q.ListConversationThreadPosts(ctx, models.ListConversationThreadPostsParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		})

		return err
	})

	return out, ignoreMissing(err)
}

/*
cameFromSlack reports whether the message was written in Slack.
*/
func cameFromSlack(m models.Message) bool {
	var meta struct {
		Slack json.RawMessage `json:"slack"`
	}

	return json.Unmarshal(m.Metadata, &meta) == nil && len(meta.Slack) > 0
}

/*
body returns a message's text.
*/
func body(m models.Message) string {
	if m.Body == nil {
		return ""
	}

	return *m.Body
}
