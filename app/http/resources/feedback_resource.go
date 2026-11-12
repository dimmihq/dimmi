/*
|--------------------------------------------------------------------------
| Feature Request Resources
|--------------------------------------------------------------------------
|
| How feature requests read over the API: each with its number, status,
| approval, counts and who posted it; their voters with the company each
| belongs to and how much it matters to them; and their comments and
| internal notes.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FeedbackPost is one feature request.
*/
type FeedbackPost struct {
	ID               string     `json:"id"`
	Object           string     `json:"object"`
	HelpCenterID     string     `json:"help_center_id"`
	BoardID          *string    `json:"board_id"`
	Number           int32      `json:"number"`
	Title            string     `json:"title"`
	Slug             string     `json:"slug"`
	BodyMarkdown     string     `json:"body_markdown"`
	Status           string     `json:"status"`
	Moderation       string     `json:"moderation"`
	Author           string     `json:"author"`
	AuthorContactID  *string    `json:"author_contact_id"`
	VoteCount        int32      `json:"vote_count"`
	CommentCount     int32      `json:"comment_count"`
	MergedIntoID     *string    `json:"merged_into_id"`
	ChangelogEntryID *string    `json:"changelog_entry_id"`
	Path             string     `json:"path"`
	StatusChangedAt  *time.Time `json:"status_changed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

/*
NewFeedbackPost shapes one request.
*/
func NewFeedbackPost(p feedback.Post) FeedbackPost {
	out := FeedbackPost{
		ID: models.PublicID(models.PrefixFeedbackPost, p.ID), Object: "feedback_post",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, p.HelpCenterID), Number: p.Number, Title: p.Title,
		Slug: p.Slug, BodyMarkdown: p.BodyMarkdown, Status: p.Status, Moderation: p.Moderation, Author: p.Author,
		VoteCount: p.VoteCount, CommentCount: p.CommentCount, Path: feedback.PostPath(p.FeedbackPost),
		StatusChangedAt: p.StatusChangedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if p.BoardID != nil {
		out.BoardID = new(models.PublicID(models.PrefixFeedbackBoard, *p.BoardID))
	}
	if p.AuthorContactID != nil {
		out.AuthorContactID = new(models.PublicID(models.PrefixContact, *p.AuthorContactID))
	}
	if p.MergedIntoID != nil {
		out.MergedIntoID = new(models.PublicID(models.PrefixFeedbackPost, *p.MergedIntoID))
	}
	if p.ChangelogEntryID != nil {
		out.ChangelogEntryID = new(models.PublicID(models.PrefixChangelogEntry, *p.ChangelogEntryID))
	}
	return out
}

/*
NewFeedbackPosts shapes a page of requests.
*/
func NewFeedbackPosts(rows []feedback.Post) []FeedbackPost {
	out := make([]FeedbackPost, len(rows))
	for i, r := range rows {
		out[i] = NewFeedbackPost(r)
	}
	return out
}

/*
FeedbackVoter is a customer who voted for a request.
*/
type FeedbackVoter struct {
	ContactID string    `json:"contact_id"`
	Name      *string   `json:"name"`
	Email     *string   `json:"email"`
	AvatarURL *string   `json:"avatar_url"`
	Company   *string   `json:"company"`
	Priority  string    `json:"priority"`
	ByTeam    bool      `json:"by_team"`
	CreatedAt time.Time `json:"created_at"`
}

/*
NewFeedbackVoters shapes a request's voters.
*/
func NewFeedbackVoters(rows []models.ListFeedbackVotersRow) []FeedbackVoter {
	out := make([]FeedbackVoter, len(rows))
	for i, r := range rows {
		out[i] = FeedbackVoter{
			ContactID: models.PublicID(models.PrefixContact, r.ContactID), Name: r.Name, Email: r.Email,
			AvatarURL: r.AvatarUrl, Company: r.Company, Priority: r.Priority, ByTeam: r.MemberID != nil,
			CreatedAt: r.CreatedAt,
		}
	}
	return out
}

/*
FeedbackComment is a comment or internal note on a request.
*/
type FeedbackComment struct {
	ID         string    `json:"id"`
	ParentID   *string   `json:"parent_id"`
	Body       string    `json:"body"`
	Internal   bool      `json:"internal"`
	AuthorName string    `json:"author_name"`
	FromTeam   bool      `json:"from_team"`
	CreatedAt  time.Time `json:"created_at"`
}

/*
NewFeedbackComments shapes a request's comments.
*/
func NewFeedbackComments(rows []models.ListFeedbackCommentsRow) []FeedbackComment {
	out := make([]FeedbackComment, len(rows))
	for i, r := range rows {
		out[i] = FeedbackComment{
			ID: models.PublicID(models.PrefixFeedbackComment, r.ID), Body: r.Body, Internal: r.Internal,
			AuthorName: r.AuthorName, FromTeam: r.AuthorMemberID != nil, CreatedAt: r.CreatedAt,
		}
		if r.ParentID != nil {
			out[i].ParentID = new(models.PublicID(models.PrefixFeedbackComment, *r.ParentID))
		}
	}
	return out
}
