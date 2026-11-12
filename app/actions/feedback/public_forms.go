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
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SubmitInput is a request a signed-in customer posts, on the board with
the slug Board, or the first they may open. Sure says they saw the
similar requests and want to post theirs anyway.
*/
type SubmitInput struct {
	Center models.HelpCenter
	Reader ids.UUID
	Board  string
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
	err = p.insideTx(ctx, in.Center, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		board, similar, err := beforePosting(ctx, q, in, title)
		if err != nil || len(similar) > 0 {
			out.Similar = similar
			return err
		}
		moderation := "approved"
		if board.ApprovalRequired || helpcenters.SettingsOf(in.Center).FeedbackApproval {
			moderation = "pending"
		}
		row, err := p.insert(ctx, q, draft{
			ws: in.Center.WorkspaceID, hc: in.Center.ID, board: board.ID, title: title, body: body,
			moderation: moderation,
			contact:    &in.Reader,
		})
		if err != nil {
			return err
		}
		out.Post = &row
		cast := vote{post: row, contact: in.Reader, priority: "nice", now: p.Clock.Now()}
		if err := castVote(ctx, q, cast); err != nil {
			return err
		}
		return p.announce(ctx, tx, appevents.FeedbackPostCreated{
			Base: p.byContact(row.WorkspaceID, in.Reader), PostID: row.ID,
		})
	})
	return out, err
}

/*
beforePosting returns the board the reader posts on and, unless they
already saw them, the approved requests like theirs on the boards they
may open.
*/
func beforePosting(ctx context.Context, q *models.Queries, in SubmitInput, title string,
) (models.FeedbackBoard, []models.FeedbackPost, error) {
	boards, err := shownBoards(ctx, q, in.Center, &in.Reader)
	if err != nil {
		return models.FeedbackBoard{}, nil, err
	}
	board, ok := pick(boards, in.Board)
	if !ok {
		return board, nil, ErrBoardNotFound
	}
	if in.Sure {
		return board, nil, nil
	}
	similar, err := q.SimilarFeedbackPosts(ctx, models.SimilarFeedbackPostsParams{
		WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, BoardIds: boardIDs(boards), Query: title,
	})
	if err != nil {
		return board, nil, fmt.Errorf("find similar requests: %w", err)
	}
	return board, similar, nil
}

/*
pick returns the board with the slug, or the first board when no slug
is given.
*/
func pick(boards []models.FeedbackBoard, slug string) (models.FeedbackBoard, bool) {
	for _, b := range boards {
		if slug == "" || b.Slug == slug {
			return b, true
		}
	}
	return models.FeedbackBoard{}, false
}

/*
Similar returns the approved requests whose titles look like the text,
on the boards the reader may open.
*/
func (p *Posts) Similar(ctx context.Context, h models.HelpCenter, reader *ids.UUID, title string,
) ([]models.FeedbackPost, error) {
	var out []models.FeedbackPost
	err := p.inside(ctx, h, func(ctx context.Context, q *models.Queries) error {
		boards, err := shownBoards(ctx, q, h, reader)
		if err != nil {
			return err
		}
		out, err = q.SimilarFeedbackPosts(ctx, models.SimilarFeedbackPostsParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, BoardIds: boardIDs(boards), Query: title,
		})
		return err
	})
	return out, err
}

/*
BoardsFor returns the help center's boards the reader may open, in the
team's order.
*/
func (p *Posts) BoardsFor(ctx context.Context, h models.HelpCenter, reader *ids.UUID,
) ([]models.FeedbackBoard, error) {
	var out []models.FeedbackBoard
	err := p.inside(ctx, h, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = shownBoards(ctx, q, h, reader)
		return err
	})
	return out, err
}

/*
Ballot is where a reader's vote stands after they toggle it: whether
they now vote for the request, and how many readers do.
*/
type Ballot struct {
	Voted bool
	Votes int32
}

/*
Toggle counts the reader's vote for the request, or takes it back when
they already voted, and returns where the vote now stands. Found is
false for a request they cannot see.
*/
func (p *Posts) Toggle(ctx context.Context, h models.HelpCenter, number int32, reader ids.UUID,
) (Ballot, bool, error) {
	found, out := false, Ballot{}
	err := p.insideTx(ctx, h, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		row, err := q.FindFeedbackPostByNumber(ctx, models.FindFeedbackPostByNumberParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Number: number,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !open(row)) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find board request: %w", err)
		}
		if found, err = shown(ctx, q, row, &reader); err != nil || !found {
			return err
		}
		if out, err = p.flip(ctx, q, row, reader); err != nil || !out.Voted {
			return err
		}
		cast := vote{post: row, contact: reader, priority: "nice"}
		return p.voted(ctx, tx, p.byContact(row.WorkspaceID, reader), cast)
	})
	return out, found, err
}

/*
flip takes back the reader's vote on the request when they had one, and
casts it when they had none, then reads where it stands.
*/
func (p *Posts) flip(ctx context.Context, q *models.Queries, row models.FeedbackPost, reader ids.UUID,
) (Ballot, error) {
	removed, err := q.RemoveFeedbackVote(ctx, models.RemoveFeedbackVoteParams{
		WorkspaceID: row.WorkspaceID, PostID: row.ID, ContactID: reader,
	})
	if err != nil {
		return Ballot{}, fmt.Errorf("take back board vote: %w", err)
	}
	if removed > 0 {
		err = recount(ctx, q, row)
	}
	if removed == 0 {
		err = castVote(ctx, q, vote{post: row, contact: reader, priority: "nice", now: p.Clock.Now()})
	}
	if err != nil {
		return Ballot{}, err
	}
	after, err := q.FindFeedbackPost(ctx, models.FindFeedbackPostParams{WorkspaceID: row.WorkspaceID, ID: row.ID})
	if err != nil {
		return Ballot{}, fmt.Errorf("read board request: %w", err)
	}
	return Ballot{Voted: removed == 0, Votes: after.VoteCount}, nil
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
		if ok, err := shown(ctx, q, row, &in.Reader); err != nil || !ok {
			return cmp.Or(err, error(ErrPostNotFound))
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
