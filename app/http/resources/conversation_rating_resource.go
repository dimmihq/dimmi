/*
|--------------------------------------------------------------------------
| Conversation Rating Resources
|--------------------------------------------------------------------------
|
| How customers rated a conversation, as the team reads it beside the
| conversation; the workspace's settings for asking; and the ask as the
| customer sees it, in the messenger or on the page an email links to.
| Scores run from 1 to 5, worst first, each with its face and word.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/ratings"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationRating is one answered ask: its score, face and word, the
customer's comment, where it was asked, and the teammate it rated.
*/
type ConversationRating struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	ConversationID string     `json:"conversation_id"`
	Score          int        `json:"score"`
	Face           string     `json:"face"`
	Label          string     `json:"label"`
	Comment        *string    `json:"comment"`
	Channel        string     `json:"channel"`
	Member         *Named     `json:"member"`
	AskedAt        *time.Time `json:"asked_at"`
	RatedAt        *time.Time `json:"rated_at"`
}

/*
NewConversationRatings shapes a conversation's answered asks.
*/
func NewConversationRatings(rows []models.ListConversationRatingsRow) []ConversationRating {
	out := make([]ConversationRating, 0, len(rows))
	for _, r := range rows {
		if r.Score == nil {
			continue
		}
		score := int(*r.Score)
		item := ConversationRating{
			ID: models.PublicID(models.PrefixConversationRating, r.ID), Object: "conversation_rating",
			ConversationID: models.PublicID(models.PrefixConversation, r.ConversationID), Score: score,
			Face: ratings.Faces()[score-1], Label: ratings.Labels()[score-1], Comment: r.Comment, Channel: r.Channel,
			AskedAt: r.AskedAt, RatedAt: r.RatedAt,
		}
		if r.MemberID != nil {
			name := ""
			if r.MemberName != nil {
				name = *r.MemberName
			}
			item.Member = &Named{ID: models.PublicID(models.PrefixMember, *r.MemberID), Object: "member", Name: name}
		}
		out = append(out, item)
	}

	return out
}

/*
ConversationRatingSettings is how the workspace asks for ratings.
*/
type ConversationRatingSettings struct {
	Object             string `json:"object"`
	Enabled            bool   `json:"enabled"`
	DelayMinutes       int    `json:"delay_minutes"`
	IncludeAgentClosed bool   `json:"include_agent_closed"`
	Messenger          bool   `json:"messenger"`
	Email              bool   `json:"email"`
}

/*
NewConversationRatingSettings shapes the workspace's settings.
*/
func NewConversationRatingSettings(s ratings.Settings) ConversationRatingSettings {
	return ConversationRatingSettings{
		Object: "conversation_rating_settings", Enabled: s.Enabled, DelayMinutes: s.DelayMinutes,
		IncludeAgentClosed: s.IncludeAgentClosed, Messenger: s.Messenger, Email: s.Email,
	}
}

/*
RatingFace is one face the customer can pick.
*/
type RatingFace struct {
	Score int    `json:"score"`
	Face  string `json:"face"`
	Label string `json:"label"`
}

/*
CustomerRating is an ask as its customer sees it: the faces to pick
from, what they picked and wrote, and whether it still takes changes.
Workspace names who asks, on the page an email links to.
*/
type CustomerRating struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"`
	Workspace string       `json:"workspace,omitempty"`
	Score     *int         `json:"score"`
	Comment   *string      `json:"comment"`
	Editable  bool         `json:"editable"`
	Faces     []RatingFace `json:"faces"`
}

/*
NewCustomerRating shapes an ask for its customer.
*/
func NewCustomerRating(r ratings.Rating) CustomerRating {
	out := CustomerRating{
		ID: models.PublicID(models.PrefixConversationRating, r.Row.ID), Object: "conversation_rating",
		Workspace: r.Workspace, Comment: r.Row.Comment, Editable: r.Editable, Faces: make([]RatingFace, 5),
	}
	if r.Row.Score != nil {
		score := int(*r.Row.Score)
		out.Score = &score
	}
	for i := range out.Faces {
		out.Faces[i] = RatingFace{Score: i + 1, Face: ratings.Faces()[i], Label: ratings.Labels()[i]}
	}

	return out
}
