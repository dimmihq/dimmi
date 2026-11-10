/*
|--------------------------------------------------------------------------
| Slack Triage
|--------------------------------------------------------------------------
|
| A customer room can name a triage channel, or the connection names one
| for all its rooms. Each new conversation is posted there once, as a
| card saying who waits, on what, and where it stands, and the card is
| brought up to date as the conversation moves: assigned, closed,
| reopened, snoozed, made urgent, handed to the team. Changes in a burst
| update it once. A card someone deleted in Slack is posted again, once,
| and the new one is remembered.
|
| The thread under the card is the team's: what teammates write there
| becomes notes, and only a message they start with >> reaches the
| customer, as their own reply.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
triageChannel is the channel the room's conversations are triaged in:
the room's own choice, else the connection's, empty for none.
*/
func triageChannel(conn models.ChannelConnection, room *models.ChannelRoom) string {
	if channel := settingOf(room, settingTriageChannel); channel != "" {
		return channel
	}

	var channel string
	connectionSetting(conn, settingTriageChannel, &channel)

	return channel
}

/*
syncTriage posts the conversation's triage card, or brings the posted
one up to date.
*/
func (s *Slack) syncTriage(ctx context.Context, conn models.ChannelConnection, conv models.Conversation) error {
	room, ok, err := s.roomOfConversation(ctx, conv)
	if err != nil || !ok || !customerRoom(&room) {
		return err
	}

	channel := triageChannel(conn, &room)
	if channel == "" || channel == room.ExternalID {
		return nil
	}

	card, err := s.triageCard(ctx, conv, room)
	if err != nil {
		return err
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	post, found, err := s.triagePost(ctx, conv)
	if err != nil {
		return err
	}

	if found {
		err = api.UpdateTeamCard(ctx, post.ExternalTs, slackdriver.TriageCardOf(post.ExternalChannel, card))
		if slackdriver.Code(err) != slackdriver.CodeMessageNotFound {
			return err
		}
	}

	ts, err := api.PostTeamCard(ctx, slackdriver.TriageCardOf(channel, card))
	if err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.SaveTriagePost(ctx, models.SaveTriagePostParams{
			WorkspaceID: conv.WorkspaceID, ChannelConnectionID: conn.ID, ConversationID: conv.ID,
			ExternalChannel: channel, ExternalTs: ts, Now: s.Clock.Now(),
		})

		return err
	})
}

/*
triagePost returns the conversation's triage post, reporting false
when it has none yet.
*/
func (s *Slack) triagePost(ctx context.Context, conv models.Conversation) (models.ChannelConversationPost, bool,
	error,
) {
	var post models.ChannelConversationPost

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		post, err = q.FindTriagePost(ctx, models.FindTriagePostParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return post, false, nil
	}

	if err != nil {
		return post, false, fmt.Errorf("find triage post: %w", err)
	}

	return post, true, nil
}

/*
triageCard reads what the card says about the conversation.
*/
func (s *Slack) triageCard(ctx context.Context, conv models.Conversation, room models.ChannelRoom) (
	slackdriver.TriageCard, error,
) {
	card := slackdriver.TriageCard{
		ConversationID: models.PublicID(models.PrefixConversation, conv.ID), Status: conv.Status,
		Priority: string(enums.ConversationPriorityOf(conv.Priority)), AppLink: s.conversationLink(conv.ID),
		HandedOff: conv.AiState == "handed_off",
		Solved:    conv.Status == string(enums.ConversationStatusClosed) && conv.FirstReplyAt != nil,
		Customer:  "A customer",
	}

	if thread, ok := threadOf(conv); ok && !thread.Direct() {
		card.ThreadLink = "https://slack.com/archives/" + room.ExternalID + "/p" + strings.ReplaceAll(thread.RootTS, ".", "")
	}

	err := s.read(ctx, func(q *models.Queries) error { return fillCard(ctx, q, conv, &card) })
	if err != nil {
		return card, fmt.Errorf("read triage card: %w", err)
	}

	return card, nil
}

/*
fillCard reads the customer, their company, the assignee and the first
words the customer wrote.
*/
func fillCard(ctx context.Context, q *models.Queries, conv models.Conversation, card *slackdriver.TriageCard) error {
	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: conv.WorkspaceID, ID: conv.ContactID})
	if err != nil {
		return err
	}

	if contact.Name != nil && *contact.Name != "" {
		card.Customer = *contact.Name
	}

	if contact.CompanyID != nil {
		if company, err := q.FindCompany(ctx, models.FindCompanyParams{
			WorkspaceID: conv.WorkspaceID, ID: *contact.CompanyID,
		}); err == nil {
			card.Company = company.Name
		}
	}

	if conv.AssigneeMemberID != nil {
		if member, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
			WorkspaceID: conv.WorkspaceID, ID: *conv.AssigneeMemberID,
		}); err == nil {
			card.Assignee = member.User.Name
		}
	}

	return firstWords(ctx, q, conv, card)
}

/*
firstWords puts the customer's first message on the card, and marks it
gone when the customer deleted it in Slack.
*/
func firstWords(ctx context.Context, q *models.Queries, conv models.Conversation, card *slackdriver.TriageCard) error {
	messages, err := q.ListMessages(ctx, models.ListMessagesParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, RowLimit: 20,
	})
	if err != nil {
		return err
	}

	for _, m := range messages {
		if m.AuthorType != string(enums.AuthorTypeContact) || m.Type != string(enums.MessageTypeMessage) {
			continue
		}

		if m.Body != nil {
			card.Excerpt = *m.Body
		}

		var meta struct {
			SourceDeleted json.RawMessage `json:"source_deleted"`
		}

		_ = json.Unmarshal(m.Metadata, &meta)
		card.Gone = len(meta.SourceDeleted) > 0 && conv.Status == string(enums.ConversationStatusClosed)

		return nil
	}

	return nil
}

/*
threadOf returns where a conversation's thread is on Slack, reporting
false for a conversation without one.
*/
func threadOf(conv models.Conversation) (slackdriver.Thread, bool) {
	if conv.ExternalThreadKey == nil {
		return slackdriver.Thread{}, false
	}

	return slackdriver.ParseThreadKey(*conv.ExternalThreadKey)
}
