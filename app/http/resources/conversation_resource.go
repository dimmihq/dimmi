/*
|--------------------------------------------------------------------------
| Conversation Resources
|--------------------------------------------------------------------------
|
| A conversation as a list row and as the open conversation show it: its
| number, subject and where it stands, who holds it, the customer, the
| channel and inbox, the latest message and whether the reader has seen
| everything in it. The short form names a conversation in other
| answers, such as a neighbor or a search hit.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Conversation is one conversation with what is shown beside it. Viewers
are the teammates viewing it, present only when the list asked for them
and they arrived in time.
*/
type Conversation struct {
	ID                     string                           `json:"id"`
	Object                 string                           `json:"object"`
	Number                 int64                            `json:"number"`
	Subject                *string                          `json:"subject"`
	Status                 string                           `json:"status"`
	Spam                   bool                             `json:"spam"`
	Priority               string                           `json:"priority"`
	Unread                 bool                             `json:"unread"`
	WaitingSince           *time.Time                       `json:"waiting_since"`
	SnoozedUntil           *time.Time                       `json:"snoozed_until"`
	LastMessageAt          *time.Time                       `json:"last_message_at"`
	LastActivityAt         *time.Time                       `json:"last_activity_at"`
	FirstCustomerMessageAt *time.Time                       `json:"first_customer_message_at"`
	FirstReplyAt           *time.Time                       `json:"first_reply_at"`
	ClosedAt               *time.Time                       `json:"closed_at"`
	MessageCount           int32                            `json:"message_count"`
	Channel                Channel                          `json:"channel"`
	Inbox                  Named                            `json:"inbox"`
	Contact                ContactSummary                   `json:"contact"`
	Assignee               *MemberSummary                   `json:"assignee"`
	Team                   *Named                           `json:"team"`
	LastMessage            *MessageSummary                  `json:"last_message"`
	Viewers                *[]appbroadcasting.ViewerPayload `json:"viewers,omitempty"`
	CreatedAt              time.Time                        `json:"created_at"`
	UpdatedAt              time.Time                        `json:"updated_at"`
}

/*
MemberSummary is the short form of a member.
*/
type MemberSummary struct {
	ID        string  `json:"id"`
	Object    string  `json:"object"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
MessageSummary is the short form of a message: who wrote it and how it
starts.
*/
type MessageSummary struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Type      string    `json:"type"`
	Author    Author    `json:"author"`
	Excerpt   string    `json:"excerpt"`
	CreatedAt time.Time `json:"created_at"`
}

/*
NewConversation shapes the conversation.
*/
func NewConversation(l conversations.Loaded) Conversation {
	c := l.Conversation
	out := Conversation{
		ID: models.PublicID(models.PrefixConversation, c.ID), Object: "conversation", Number: c.Number,
		Subject: c.Subject, Status: c.Status, Spam: c.Spam, Unread: l.Unread,
		Priority:     string(enums.ConversationPriorityOf(c.Priority)),
		WaitingSince: c.WaitingSince, SnoozedUntil: c.SnoozedUntil, LastMessageAt: c.LastMessageAt,
		LastActivityAt:         c.LastActivityAt,
		FirstCustomerMessageAt: c.FirstCustomerMessageAt, FirstReplyAt: c.FirstReplyAt, ClosedAt: c.ClosedAt,
		MessageCount: c.MessageCount, Channel: NewChannel(l.Connection),
		Inbox:   Named{ID: models.PublicID(models.PrefixInbox, l.Inbox.ID), Object: "inbox", Name: l.Inbox.Name},
		Contact: NewContactSummary(l.Contact), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}

	if l.Assignee != nil {
		out.Assignee = new(NewMemberSummary(*l.Assignee))
	}

	if l.Team != nil {
		out.Team = &Named{ID: models.PublicID(models.PrefixTeam, l.Team.ID), Object: "team", Name: l.Team.Name}
	}

	if l.LastMessage != nil {
		out.LastMessage = &MessageSummary{
			ID: models.PublicID(models.PrefixMessage, l.LastMessage.ID), Object: "message", Type: l.LastMessage.Type,
			Author:  author(*l.LastMessage, l.Contact, l.LastAuthor),
			Excerpt: excerpt(l.LastMessage), CreatedAt: l.LastMessage.CreatedAt,
		}
	}

	return out
}

/*
NewConversations shapes a list of conversations, in the order given.
*/
func NewConversations(rows []conversations.Loaded) []Conversation {
	out := make([]Conversation, 0, len(rows))
	for _, l := range rows {
		out = append(out, NewConversation(l))
	}

	return out
}

/*
NewMemberSummary shapes the short form of the member.
*/
func NewMemberSummary(m conversations.Member) MemberSummary {
	return MemberSummary{
		ID: models.PublicID(models.PrefixMember, m.Member.ID), Object: "member", Name: m.User.Name,
	}
}

/*
ConversationRef is the shortest form of a conversation: enough to link
to it.
*/
type ConversationRef struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Number  int64   `json:"number"`
	Subject *string `json:"subject"`
	Status  string  `json:"status"`
}

/*
NewConversationRef shapes the shortest form of the conversation.
*/
func NewConversationRef(c models.Conversation) ConversationRef {
	return ConversationRef{
		ID: models.PublicID(models.PrefixConversation, c.ID), Object: "conversation", Number: c.Number,
		Subject: c.Subject, Status: c.Status,
	}
}
