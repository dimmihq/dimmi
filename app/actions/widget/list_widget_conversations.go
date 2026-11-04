/*
|--------------------------------------------------------------------------
| List Widget Conversations
|--------------------------------------------------------------------------
|
| A visitor sees their own conversations on the widget they are on: those
| started there, those of another channel that live in the widget's inbox,
| and for a signed-in customer the history imported from elsewhere. Notes,
| events and anything about the team's workings stay out; each answer is
| built from what a visitor may see and nothing else.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Page sizes of a visitor's lists.
*/
const (
	DefaultConversationPage = 20
	MaxConversationPage     = 50
	DefaultMessagePage      = 30
	MaxMessagePage          = 100
	attachmentLinkTTL       = 30 * time.Minute
)

/*
ListWidgetConversations reads a visitor's conversations and messages. Disk
signs the links of their files.
*/
type ListWidgetConversations struct {
	DB   *database.DB
	Disk filesystem.Filesystem
}

/*
List returns a page of the visitor's conversations, latest activity first.
*/
func (a *ListWidgetConversations) List(ctx context.Context, in ConversationQuery) (ConversationPage, error) {
	var out ConversationPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		params := listParams(in)

		rows, err := q.ListWidgetConversations(ctx, params)
		if err != nil {
			return fmt.Errorf("list widget conversations: %w", err)
		}

		if len(rows) > in.Limit {
			last := rows[in.Limit-1].Conversation
			rows, out.Next = rows[:in.Limit], &Cursor{At: activity(last), ID: last.ID}
		}

		items := make([]VisibleConversation, 0, len(rows))
		for _, row := range rows {
			items = append(items, VisibleConversation{
				Conversation: row.Conversation, ChannelType: row.ChannelType, UnreadCount: row.UnreadCount,
			})
		}

		out.Items, err = a.withLatest(ctx, q, in.Auth.Session.WorkspaceID, items)

		return err
	})

	return out, err
}

/*
Find returns one of the visitor's conversations, and
[exceptions.ErrWidgetConversationNotFound] for any other.
*/
func (a *ListWidgetConversations) Find(
	ctx context.Context, auth Authenticated, id ids.UUID,
) (VisibleConversation, error) {
	var out VisibleConversation

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.FindWidgetConversation(ctx, scopeOf(auth, id))
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrWidgetConversationNotFound
		}

		if err != nil {
			return fmt.Errorf("find widget conversation: %w", err)
		}

		items, err := a.withLatest(ctx, q, auth.Session.WorkspaceID, []VisibleConversation{{
			Conversation: row.Conversation, ChannelType: row.ChannelType,
			UnreadCount: row.UnreadCount,
		}})
		out = items[0]

		return err
	})

	return out, err
}

/*
withLatest adds each conversation's latest visible message.
*/
func (a *ListWidgetConversations) withLatest(
	ctx context.Context, q *models.Queries, ws ids.UUID, items []VisibleConversation,
) ([]VisibleConversation, error) {
	convIDs := make([]ids.UUID, 0, len(items))
	for _, item := range items {
		convIDs = append(convIDs, item.Conversation.ID)
	}

	latest, err := q.LatestWidgetMessages(ctx, models.LatestWidgetMessagesParams{
		WorkspaceID: ws, ConversationIds: convIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list latest widget messages: %w", err)
	}

	visible, err := a.visible(ctx, q, ws, latest)
	if err != nil {
		return nil, err
	}

	byConversation := map[ids.UUID]*VisibleMessage{}
	for i := range visible {
		byConversation[visible[i].Message.ConversationID] = &visible[i]
	}

	for i := range items {
		items[i].Last = byConversation[items[i].Conversation.ID]
	}

	return items, nil
}

/*
Messages returns a page of the conversation's messages, oldest first.
*/
func (a *ListWidgetConversations) Messages(ctx context.Context, in MessageQuery) (MessagePage, error) {
	var out MessagePage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation)); err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return exceptions.ErrWidgetConversationNotFound
			}

			return fmt.Errorf("find widget conversation: %w", err)
		}

		rows, err := q.ListWidgetMessages(ctx, messageParams(in))
		if err != nil {
			return fmt.Errorf("list widget messages: %w", err)
		}

		out.HasMore = len(rows) > in.Limit
		if out.HasMore {
			rows = rows[:in.Limit]
		}

		if in.After == nil {
			reverse(rows)
		}

		out.Items, err = a.visible(ctx, q, in.Auth.Session.WorkspaceID, rows)

		return err
	})

	return out, err
}

/*
Present returns messages as a visitor sees them, for callers outside a
transaction of their own.
*/
func (a *ListWidgetConversations) Present(
	ctx context.Context, ws ids.UUID, rows []models.Message,
) ([]VisibleMessage, error) {
	var out []VisibleMessage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = a.visible(ctx, models.New(tx), ws, rows)

		return err
	})

	return out, err
}

/*
visible adds each message's author and files, as a visitor sees them.
*/
func (a *ListWidgetConversations) visible(
	ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.Message,
) ([]VisibleMessage, error) {
	authors, err := a.authors(ctx, q, ws, rows)
	if err != nil {
		return nil, err
	}

	files, err := a.files(ctx, q, ws, rows)
	if err != nil {
		return nil, err
	}

	out := make([]VisibleMessage, 0, len(rows))
	for _, m := range rows {
		author := Author{Type: m.AuthorType}
		if m.AuthorID != nil && m.AuthorType == string(enums.AuthorTypeMember) {
			author = authors[*m.AuthorID]
		}

		out = append(out, VisibleMessage{Message: m, Author: author, Attachments: files[m.ID]})
	}

	return out, nil
}

/*
authors returns the members who wrote the messages, by member id.
*/
func (a *ListWidgetConversations) authors(
	ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.Message,
) (map[ids.UUID]Author, error) {
	var members []ids.UUID

	for _, m := range rows {
		if m.AuthorID != nil && m.AuthorType == string(enums.AuthorTypeMember) {
			members = append(members, *m.AuthorID)
		}
	}

	out := map[ids.UUID]Author{}
	if len(members) == 0 {
		return out, nil
	}

	found, err := q.ListWidgetMemberAuthors(ctx, models.ListWidgetMemberAuthorsParams{
		WorkspaceID: ws, MemberIds: members,
	})
	if err != nil {
		return nil, fmt.Errorf("list widget message authors: %w", err)
	}

	for _, row := range found {
		name := firstName(row.Name)
		author := Author{Type: string(enums.AuthorTypeMember), Name: &name}

		if row.AvatarPath != nil {
			if url, err := a.Disk.TemporaryURL(ctx, *row.AvatarPath, avatarLinkTTL); err == nil {
				author.AvatarURL = &url
			}
		}

		out[row.ID] = author
	}

	return out, nil
}

/*
files returns the messages' attachments by message, linked only once the
scan cleared them.
*/
func (a *ListWidgetConversations) files(
	ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.Message,
) (map[ids.UUID][]VisibleAttachment, error) {
	messageIDs := make([]ids.UUID, 0, len(rows))
	for _, m := range rows {
		messageIDs = append(messageIDs, m.ID)
	}

	found, err := q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
		WorkspaceID: ws, MessageIds: messageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list widget message files: %w", err)
	}

	out := map[ids.UUID][]VisibleAttachment{}

	for _, row := range found {
		item := VisibleAttachment{Row: row, Blocked: true}

		if row.ScanStatus == "clean" || row.ScanStatus == "skipped" {
			url, err := a.Disk.TemporaryURL(ctx, row.Path, attachmentLinkTTL)
			if err == nil {
				item.URL, item.Blocked = &url, false
			}
		}

		out[*row.MessageID] = append(out[*row.MessageID], item)
	}

	return out, nil
}

/*
activity returns when the conversation last moved, the order of the list.
*/
func activity(c models.Conversation) time.Time {
	if c.LastMessageAt != nil {
		return *c.LastMessageAt
	}

	return c.CreatedAt
}

/*
listParams returns the query of a page of the visitor's conversations, one
more than asked so the page knows whether another follows.
*/
func listParams(in ConversationQuery) models.ListWidgetConversationsParams {
	s := in.Auth.Session
	p := models.ListWidgetConversationsParams{
		WorkspaceID: s.WorkspaceID, ContactID: s.ContactID,
		WidgetID: models.PublicID(models.PrefixWidget, in.Auth.Resolved.Widget.ID), InboxID: in.Auth.Resolved.InboxID,
		IncludeImported: s.Kind == string(enums.WidgetSessionIdentified), Statuses: in.Statuses,
		RowLimit: int64(in.Limit) + 1,
	}

	if p.Statuses == nil {
		p.Statuses = []string{}
	}

	if in.After != nil {
		p.BeforeAt, p.BeforeID = &in.After.At, &in.After.ID
	}

	return p
}

/*
messageParams returns the query of a page of messages, one more than asked.
*/
func messageParams(in MessageQuery) models.ListWidgetMessagesParams {
	p := models.ListWidgetMessagesParams{
		WorkspaceID: in.Auth.Session.WorkspaceID, ConversationID: in.Conversation,
		RowLimit: int64(in.Limit) + 1,
	}

	if in.Before != nil {
		p.BeforeAt, p.BeforeID = &in.Before.At, &in.Before.ID
	}

	if in.After != nil {
		p.AfterAt, p.AfterID = &in.After.At, &in.After.ID
	}

	return p
}

/*
reverse puts the messages oldest first.
*/
func reverse(rows []models.Message) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

/*
firstName returns the first word of a teammate's name, which is all a
visitor sees of it.
*/
func firstName(name string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(name), " ")

	return first
}

/*
Message returns one message the visitor may see in their conversation.
*/
func (a *ListWidgetConversations) Message(
	ctx context.Context, auth Authenticated, conversation, id ids.UUID,
) (models.Message, error) {
	var out models.Message

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.FindWidgetConversation(ctx, scopeOf(auth, conversation)); err != nil {
			return err
		}

		var err error
		out, err = q.FindWidgetMessage(ctx, models.FindWidgetMessageParams{
			WorkspaceID: auth.Session.WorkspaceID, ConversationID: conversation, ID: id,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, exceptions.ErrWidgetConversationNotFound
	}

	if err != nil {
		return out, fmt.Errorf("find widget message: %w", err)
	}

	return out, nil
}
