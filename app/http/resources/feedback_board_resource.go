/*
|--------------------------------------------------------------------------
| Feedback Board Resources
|--------------------------------------------------------------------------
|
| How a help center's boards read over the API: each with who may see
| it, whether it holds new posts for approval, its place in the order
| and how many requests it holds; and the conversations linked to a
| request, with their customers.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FeedbackBoard is one board.
*/
type FeedbackBoard struct {
	ID               string    `json:"id"`
	Object           string    `json:"object"`
	HelpCenterID     string    `json:"help_center_id"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Description      string    `json:"description"`
	Visibility       string    `json:"visibility"`
	ApprovalRequired bool      `json:"approval_required"`
	Position         int32     `json:"position"`
	PostCount        int32     `json:"post_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

/*
NewFeedbackBoard shapes one board.
*/
func NewFeedbackBoard(b feedback.Board) FeedbackBoard {
	return FeedbackBoard{
		ID: models.PublicID(models.PrefixFeedbackBoard, b.ID), Object: "feedback_board",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, b.HelpCenterID), Name: b.Name, Slug: b.Slug,
		Description: b.Description, Visibility: b.Visibility, ApprovalRequired: b.ApprovalRequired,
		Position: b.Position, PostCount: b.Posts, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

/*
NewFeedbackBoards shapes a help center's boards.
*/
func NewFeedbackBoards(list []feedback.Board) []FeedbackBoard {
	out := make([]FeedbackBoard, len(list))
	for i, b := range list {
		out[i] = NewFeedbackBoard(b)
	}
	return out
}

/*
FeedbackConversation is a conversation linked to a request, with its
customer and whether a teammate linked it.
*/
type FeedbackConversation struct {
	Object             string    `json:"object"`
	ConversationID     string    `json:"conversation_id"`
	ConversationNumber int64     `json:"conversation_number"`
	Subject            *string   `json:"subject"`
	ConversationStatus string    `json:"conversation_status"`
	ContactID          *string   `json:"contact_id"`
	Name               *string   `json:"name"`
	Email              *string   `json:"email"`
	Company            *string   `json:"company"`
	LinkedByTeam       bool      `json:"linked_by_team"`
	LinkedAt           time.Time `json:"linked_at"`
}

/*
NewFeedbackConversations shapes a request's linked conversations.
*/
func NewFeedbackConversations(rows []models.ListFeedbackPostConversationsRow) []FeedbackConversation {
	out := make([]FeedbackConversation, len(rows))
	for i, r := range rows {
		out[i] = FeedbackConversation{
			Object:             "feedback_conversation",
			ConversationID:     models.PublicID(models.PrefixConversation, r.ConversationID),
			ConversationNumber: r.Number, Subject: r.Subject, ConversationStatus: r.ConversationStatus,
			Name: r.ContactName, Email: r.ContactEmail, Company: r.Company, LinkedByTeam: r.MemberID != nil,
			LinkedAt: r.CreatedAt,
		}
		if r.ContactID != nil {
			out[i].ContactID = new(models.PublicID(models.PrefixContact, *r.ContactID))
		}
	}
	return out
}
