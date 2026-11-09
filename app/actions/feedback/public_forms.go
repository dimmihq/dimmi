/*
|--------------------------------------------------------------------------
| Posting, Voting and Commenting
|--------------------------------------------------------------------------
|
| What a signed-in customer does on the board: post a request, after
| seeing the ones that look alike, vote for one or take the vote back,
| and comment under it.
|
*/

package feedback

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SubmitInput is a request a signed-in customer posts. Sure says they saw
the similar requests and want to post theirs anyway.
*/
type SubmitInput struct {
	Center models.HelpCenter
	Reader ids.UUID
	Title  string
	Body   string
	Sure   bool
}

/*
Submitted is what posting gives back: the request when it was posted,
or the similar ones to look at first.
*/
type Submitted struct {
	Post    *models.FeedbackPost
	Similar []models.FeedbackPost
}

/*
Submit posts a customer's request with their vote on it, unless similar
ones exist and they have not seen them yet. It waits for approval when
the help center asks for that.
*/
func (p *Posts) Submit(ctx context.Context, in SubmitInput) (Submitted, error) {
	title, body, err := checked(&in.Title, &in.Body)
	if err != nil {
		return Submitted{}, err
	}
	var out Submitted
	err = p.inside(ctx, in.Center, func(ctx context.Context, q *models.Queries) error {
		if !in.Sure {
			similar, err := q.SimilarFeedbackPosts(ctx, models.SimilarFeedbackPostsParams{
				WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Query: title,
			})
			if err != nil {
				return fmt.Errorf("find similar requests: %w", err)
			}
			if len(similar) > 0 {
				out.Similar = similar
				return nil
			}
		}
		moderation := "approved"
		if helpcenters.SettingsOf(in.Center).FeedbackApproval {
			moderation = "pending"
		}
		row, err := p.insert(ctx, q, draft{
			ws: in.Center.WorkspaceID, hc: in.Center.ID, title: title, body: body, moderation: moderation,
			contact: &in.Reader,
		})
		if err != nil {
			return err
		}
		out.Post = &row
		return castVote(ctx, q, vote{post: row, contact: in.Reader, priority: "nice", now: p.Clock.Now()})
	})
	return out, err
}

/*
Similar returns the approved requests whose titles look like the text.
*/
func (p *Posts) Similar(ctx context.Context, h models.HelpCenter, title string) ([]models.FeedbackPost, error) {
	var out []models.FeedbackPost
	err := p.inside(ctx, h, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.SimilarFeedbackPosts(ctx, models.SimilarFeedbackPostsParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Query: title,
		})
		return err
	})
	return out, err
}

/*
Toggle counts the reader's vote for the request, or takes it back when
they already voted. Found is false for a request they cannot see.
*/
func (p *Posts) Toggle(ctx context.Context, h models.HelpCenter, number int32, reader ids.UUID) (bool, error) {
	found := false
	err := p.inside(ctx, h, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindFeedbackPostByNumber(ctx, models.FindFeedbackPostByNumberParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Number: number,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !open(row)) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find board request: %w", err)
		}
		found = true
		removed, err := q.RemoveFeedbackVote(ctx, models.RemoveFeedbackVoteParams{
			WorkspaceID: h.WorkspaceID, PostID: row.ID, ContactID: reader,
		})
		if err != nil {
			return fmt.Errorf("take back board vote: %w", err)
		}
		if removed > 0 {
			return recount(ctx, q, row)
		}
		return castVote(ctx, q, vote{post: row, contact: reader, priority: "nice", now: p.Clock.Now()})
	})
	return found, err
}

/*
ReaderComment is a signed-in customer's comment on a request.
*/
type ReaderComment struct {
	Center models.HelpCenter
	Reader ids.UUID
	Number int32
	Parent *ids.UUID
	Body   string
}

/*
Reply leaves a customer's comment on an approved request.
*/
func (p *Posts) Reply(ctx context.Context, in ReaderComment) error {
	return p.inside(ctx, in.Center, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindFeedbackPostByNumber(ctx, models.FindFeedbackPostByNumberParams{
			WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Number: in.Number,
		})
		if err == nil && !open(row) {
			return ErrPostNotFound
		}
		if err != nil {
			return missing(err, ErrPostNotFound)
		}
		by := author{ws: in.Center.WorkspaceID, contact: &in.Reader}
		_, err = p.addComment(ctx, q, by, CommentInput{Post: row.ID, Parent: in.Parent, Body: in.Body})
		return err
	})
}

/*
open says whether customers may vote and comment on the request: it is
approved and was not merged away.
*/
func open(row models.FeedbackPost) bool {
	return row.Moderation == "approved" && row.MergedIntoID == nil
}
