/*
|--------------------------------------------------------------------------
| Votes
|--------------------------------------------------------------------------
|
| Each customer votes once for a request. Customers vote from the portal,
| signed in; a teammate can vote for a customer they talked to, saying
| how much it matters to them, so what support hears counts too. Every
| vote recounts the request, its trending score included.
|
*/

package feedback

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrPriorityInvalid refuses a priority that is not one of Priorities.
*/
var ErrPriorityInvalid = fault.New(fault.Invalid, "priority_invalid",
	"The priority is nice, important or must.").OnField("priority")

/*
Priorities returns how much a request can matter to a customer.
*/
func Priorities() []string { return []string{"nice", "important", "must"} }

/*
vote is one customer's vote on a request, cast by a teammate when member
is set.
*/
type vote struct {
	post     models.FeedbackPost
	contact  ids.UUID
	member   *ids.UUID
	priority string
	now      time.Time
}

func castVote(ctx context.Context, q *models.Queries, v vote) error {
	if _, err := q.AddFeedbackVote(ctx, models.AddFeedbackVoteParams{
		PostID: v.post.ID, ContactID: v.contact, WorkspaceID: v.post.WorkspaceID, MemberID: v.member,
		Priority: v.priority, Now: v.now,
	}); err != nil {
		return fmt.Errorf("vote for feature request: %w", err)
	}
	return recount(ctx, q, v.post)
}

func recount(ctx context.Context, q *models.Queries, p models.FeedbackPost) error {
	if _, err := q.RecountFeedbackPost(ctx, models.RecountFeedbackPostParams{
		WorkspaceID: p.WorkspaceID, ID: p.ID,
	}); err != nil {
		return fmt.Errorf("recount feature request: %w", err)
	}
	return nil
}

/*
VoteInput is a teammate's vote for a customer.
*/
type VoteInput struct {
	Membership authz.Membership
	Post       ids.UUID
	Contact    ids.UUID
	Priority   string
}

/*
VoteFor counts a vote for the customer, cast by the teammate.
*/
func (p *Posts) VoteFor(ctx context.Context, in VoteInput) (Post, error) {
	if !in.Membership.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	if in.Priority == "" {
		in.Priority = "nice"
	}
	if !slices.Contains(Priorities(), in.Priority) {
		return Post{}, ErrPriorityInvalid
	}
	ws := in.Membership.Workspace.ID
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := lockPost(ctx, q, ws, in.Post)
		if err != nil {
			return err
		}
		if err := castVote(ctx, q, vote{
			post: row, contact: in.Contact, member: memberOf(in.Membership),
			priority: in.Priority, now: p.Clock.Now(),
		}); err != nil {
			return err
		}
		out, err = p.reload(ctx, q, ws, row.ID)
		return err
	})
	return out, err
}

/*
Unvote takes a customer's vote back, for the teammate who asks.
*/
func (p *Posts) Unvote(ctx context.Context, m authz.Membership, post, contact ids.UUID) (Post, error) {
	if !m.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := lockPost(ctx, q, m.Workspace.ID, post)
		if err != nil {
			return err
		}
		if _, err := q.RemoveFeedbackVote(ctx, models.RemoveFeedbackVoteParams{
			WorkspaceID: row.WorkspaceID, PostID: row.ID, ContactID: contact,
		}); err != nil {
			return fmt.Errorf("remove feature request vote: %w", err)
		}
		if err := recount(ctx, q, row); err != nil {
			return err
		}
		out, err = p.reload(ctx, q, row.WorkspaceID, row.ID)
		return err
	})
	return out, err
}

/*
Voters returns who voted for a request, newest first.
*/
func (p *Posts) Voters(ctx context.Context, m authz.Membership, post ids.UUID) ([]models.ListFeedbackVotersRow, error) {
	if !m.Can("feedback:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListFeedbackVotersRow
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findPost(ctx, q, m.Workspace.ID, post); err != nil {
			return err
		}
		var err error
		out, err = q.ListFeedbackVoters(ctx, models.ListFeedbackVotersParams{WorkspaceID: m.Workspace.ID, PostID: post})
		return err
	})
	return out, err
}

func (p *Posts) reload(ctx context.Context, q *models.Queries, ws, id ids.UUID) (Post, error) {
	row, err := findPost(ctx, q, ws, id)
	if err != nil {
		return Post{}, err
	}
	return one(ctx, q, ws, row)
}
