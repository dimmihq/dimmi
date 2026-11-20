/*
|--------------------------------------------------------------------------
| Messenger And Instagram Messages
|--------------------------------------------------------------------------
|
| A customer's message becomes an inbound message: its text, its files
| copied from Meta at once because their addresses stop working, and a
| line for what cannot be a file. A shared post, reel or link is a link.
| A reply to one of the account's stories says so, with the story's
| address. A story the customer mentioned the account in is a link
| only: Meta forbids keeping those, and the link stops working when the
| story expires. Something only the app can show, such as a view-once
| picture, is named as that.
|
| A message the account itself sent arrives as an echo. One this install
| sent is already in the conversation and is skipped; one sent from
| another place, such as Meta's own inbox, joins as the team's message.
|
*/

package channels

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ingest hands one message to the inbox, reporting false for one it has
no use for, such as an echo of the install's own reply.
*/
func (s *Social) ingest(ctx context.Context, conn models.ChannelConnection, event social.Event) (bool, error) {
	m := event.Message
	if m.ID == "" || s.own(ctx, conn, m) {
		return false, nil
	}

	body, files := s.content(ctx, m)
	if body == "" && len(files) == 0 {
		return false, nil
	}

	msg := s.inbound(conn, event, m.ID, body)
	msg.Attachments = files

	if !m.Echo {
		profile := s.Profile(ctx, conn, event.Customer)
		msg.Sender.Name = firstOf(profile.Name, profile.Username)
	}
	msg.FromWorkspace = m.Echo

	if m.Echo {
		msg.Echo = &contracts.EchoAuthor{}
	}

	if m.ReplyTo != nil && m.ReplyTo.ID != "" {
		msg.ReplyToChannelMessageID = m.ReplyTo.ID
	}

	return s.hand(ctx, msg)
}

/*
pressed hands a button the customer pressed to the inbox as what they
chose.
*/
func (s *Social) pressed(ctx context.Context, conn models.ChannelConnection, event social.Event) (bool, error) {
	if event.Postback.ID == "" || strings.TrimSpace(event.Postback.Title) == "" {
		return false, nil
	}

	return s.hand(ctx, s.inbound(conn, event, event.Postback.ID, event.Postback.Title))
}

/*
inbound is the event as an inbound message from its customer, which
marks when they last wrote once it is stored.
*/
func (s *Social) inbound(conn models.ChannelConnection, event social.Event, id, body string) contracts.InboundMessage {
	sentAt := event.At
	if sentAt.IsZero() {
		sentAt = s.Clock.Now()
	}

	msg := contracts.InboundMessage{
		ConnectionID: conn.ID, DedupeKey: string(s.Platform) + ":" + id,
		ExternalThreadKey: social.ThreadKey(event.Customer), Body: body, BodyFormat: enums.BodyFormatMarkdown,
		ChannelMessageID: id, SentAt: sentAt,
		Sender: contracts.InboundSender{
			Identities: []contracts.IdentityClaim{
				{Type: s.Platform.Identity(), Value: event.Customer, Verified: true},
			},
			Kind: enums.ContactKindUser,
		},
		Source:   map[string]any{"channel": map[string]any{"type": string(s.Platform)}},
		Metadata: map[string]any{"account_id": event.Account, "standby": event.Standby},
	}

	if event.Message == nil || !event.Message.Echo {
		msg.AfterPersist = s.wrote(conn, id, sentAt)
	}

	return msg
}

/*
wrote records, with the stored message, that the customer wrote on the
connection, which starts over how long the team may answer them.
*/
func (s *Social) wrote(conn models.ChannelConnection, id string, at time.Time) contracts.AfterPersist {
	return func(ctx context.Context, tx *database.Tx, conv models.Conversation, _ models.Message) error {
		_, err := models.New(tx).RecordChannelContactInbound(ctx, models.RecordChannelContactInboundParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ContactID: conv.ContactID, SentAt: at,
			WindowExpiresAt: at.Add(s.Config.HumanWindow), MessageID: &id, Now: s.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("record %s inbound: %w", s.Platform, err)
		}

		return nil
	}
}

/*
hand hands the message to the inbox.
*/
func (s *Social) hand(ctx context.Context, msg contracts.InboundMessage) (bool, error) {
	_, err := s.Receiver.ReceiveInboundMessage(ctx, msg)
	if errors.Is(err, conversations.ErrConnectionGone) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("receive %s message: %w", s.Platform, err)
	}

	return true, nil
}

/*
own reports whether the echo is of a reply this install sent, which the
conversation already holds.
*/
func (s *Social) own(ctx context.Context, conn models.ChannelConnection, m *social.Message) bool {
	if !m.Echo {
		return false
	}

	if app := string(m.App); app != "" && app == SocialFactsOf(conn).AppID {
		return true
	}

	found := false

	_ = s.read(ctx, func(q *models.Queries) error {
		_, err := q.FindChannelMessageRef(ctx, models.FindChannelMessageRefParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalMessageID: m.ID,
		})
		found = err == nil

		return nil
	})

	return found
}

/*
content reads the message's text and files. What cannot be a file is a
line of the text.
*/
func (s *Social) content(ctx context.Context, m *social.Message) (string, []contracts.InboundAttachment) {
	lines := []string{strings.TrimSpace(m.Text)}

	var files []contracts.InboundAttachment

	if m.ReplyTo != nil && m.ReplyTo.Story != nil {
		lines = append([]string{storyLine("Replied to your story", m.ReplyTo.Story.URL)}, lines...)
	}

	for i, a := range m.Attachments {
		switch {
		case a.File():
			file, err := s.file(ctx, m, a, i)
			if err != nil {
				lines = append(lines, "_A "+fileWord(a.Type)+" that could not be copied from "+s.Platform.Name()+"._")

				continue
			}

			files = append(files, file)
		case a.StoryMention():
			lines = append(lines, storyLine("Mentioned you in their story", a.Address()))
		case a.Shared():
			title := firstOf(cleanName(a.Title()), "Shared a "+sharedWord(a.Type))
			lines = append(lines, "["+title+"]("+a.Address()+")")
		default:
			lines = append(lines, "_Sent something only the "+s.Platform.Name()+" app can show._")
		}
	}

	if m.Unsupported {
		lines = append(lines, "_Sent something only the "+s.Platform.Name()+" app can show._")
	}

	return strings.TrimSpace(strings.Join(nonEmpty(lines...), "\n\n")), files
}

/*
storyLine is a line about a story with a link to it, which works until
the story expires.
*/
func storyLine(words, address string) string {
	if address == "" {
		return "_" + words + "._"
	}

	return "_[" + words + "](" + address + ")_"
}

func fileWord(kind string) string {
	switch kind {
	case "image", "sticker":
		return "picture"
	case "file":
		return "file"
	default:
		return kind
	}
}

func sharedWord(kind string) string {
	switch kind {
	case "reel", "ig_reel":
		return "reel"
	case "fallback":
		return "link"
	case "story", "ig_story":
		return "story"
	default:
		return "post"
	}
}

/*
file copies one of the message's files from Meta and names it by its
kind, the message and its place in it.
*/
func (s *Social) file(ctx context.Context, m *social.Message, a social.Attachment, index int) (
	contracts.InboundAttachment, error,
) {
	fetched, err := s.graph("").Fetch(ctx, a.Address(), s.Config.MaxMediaBytes)
	if err != nil {
		return contracts.InboundAttachment{}, err
	}

	if len(fetched.Body) == 0 {
		return contracts.InboundAttachment{}, meta.ErrMediaUnavailable
	}

	contentType := baseType(firstOf(fetched.ContentType, "application/octet-stream"))
	suffix := m.ID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}

	name := fmt.Sprintf("%s-%s-%d%s", fileWord(a.Type), suffix, index+1, commonExtensions()[contentType])
	body := fetched.Body

	return contracts.InboundAttachment{
		Filename: name, ContentType: contentType, SizeBytes: int64(len(body)),
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		},
	}, nil
}
