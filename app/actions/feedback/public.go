/*
|--------------------------------------------------------------------------
| The Portal's Feature Requests
|--------------------------------------------------------------------------
|
| What customers see and do on their help center's portal: approved
| requests grouped by status as a roadmap, or one list sorted by
| trending, top or new and narrowed by a search, across the boards they
| may see or on one of them; each request's page with
| its public comments; and, signed in, posting, voting and commenting.
| Before a new request is posted, customers see the ones that look alike
| and can vote for one instead. A customer's own request waiting for
| approval is shown to them alone.
|
*/

package feedback

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
boardLimit is how many requests the board shows in each status.
*/
const boardLimit = 100

/*
Listed is a request as the board lists it, with whether the reader
voted for it.
*/
type Listed struct {
	models.FeedbackPost
	Voted bool
}

/*
Column is one status of the board with its requests.
*/
type Column struct {
	Status Status
	Posts  []Listed
}

/*
BoardRequest asks for a help center's portal as a reader sees it: Sort
and Query make it one list instead of the roadmap, and Board, a board's
slug, keeps it to that board.
*/
type BoardRequest struct {
	Center models.HelpCenter
	Reader *ids.UUID
	Sort   string
	Query  string
	Board  string
}

/*
Portal is what the reader sees: the boards they may open, the one shown
when they picked one, and its roadmap columns, or the sorted list when
one was asked for.
*/
type Portal struct {
	Boards  []models.FeedbackBoard
	Active  *models.FeedbackBoard
	Columns []Column
	List    []Listed
}

/*
Read returns the help center's portal. It fails with ErrBoardNotFound
when the board asked for is not one the reader may open.
*/
func (p *Posts) Read(ctx context.Context, in BoardRequest) (Portal, error) {
	var out Portal
	err := p.inside(ctx, in.Center, func(ctx context.Context, q *models.Queries) error {
		var err error
		if out.Boards, err = shownBoards(ctx, q, in.Center, in.Reader); err != nil {
			return err
		}
		var scope []ids.UUID
		if out.Active, scope, err = narrowed(out.Boards, in.Board); err != nil {
			return err
		}
		if in.Sort != "" || strings.TrimSpace(in.Query) != "" {
			out.List, err = p.listed(ctx, q, listing{BoardRequest: in, boards: scope})
			return err
		}
		out.Columns, err = p.roadmap(ctx, q, in, scope)
		return err
	})
	return out, err
}

/*
narrowed returns the board the slug names among those the reader may
open, and the boards to list: that one, or all of them for no slug.
*/
func narrowed(boards []models.FeedbackBoard, slug string) (*models.FeedbackBoard, []ids.UUID, error) {
	if slug == "" {
		return nil, boardIDs(boards), nil
	}
	i := slices.IndexFunc(boards, func(b models.FeedbackBoard) bool { return b.Slug == slug })
	if i < 0 {
		return nil, nil, ErrBoardNotFound
	}
	return &boards[i], []ids.UUID{boards[i].ID}, nil
}

/*
roadmap returns a column for each status the roadmap shows, its
requests on the boards given, most voted first.
*/
func (p *Posts) roadmap(ctx context.Context, q *models.Queries, in BoardRequest, scope []ids.UUID,
) ([]Column, error) {
	statuses, err := ordered(ctx, q, in.Center.WorkspaceID)
	if err != nil {
		return nil, err
	}
	var out []Column
	for _, s := range statuses {
		if !s.OnRoadmap() {
			continue
		}
		top := BoardRequest{Center: in.Center, Reader: in.Reader, Sort: "top"}
		posts, err := p.listed(ctx, q, listing{BoardRequest: top, boards: scope, status: &s.Key})
		if err != nil {
			return nil, err
		}
		out = append(out, Column{Status: s, Posts: posts})
	}
	return out, nil
}

/*
shownBoards returns the help center's boards the reader may open.
*/
func shownBoards(ctx context.Context, q *models.Queries, h models.HelpCenter, reader *ids.UUID,
) ([]models.FeedbackBoard, error) {
	all, err := boardsOf(ctx, q, h)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(b models.FeedbackBoard) bool { return !readable(b, reader != nil) }), nil
}

/*
boardIDs returns the boards' ids, an empty list rather than none, so a
reader who may open no board sees no request.
*/
func boardIDs(boards []models.FeedbackBoard) []ids.UUID {
	out := make([]ids.UUID, 0, len(boards))
	for _, b := range boards {
		out = append(out, b.ID)
	}
	return out
}

/*
shown says whether the reader may see the request's board. A request
from before boards has none and is shown.
*/
func shown(ctx context.Context, q *models.Queries, row models.FeedbackPost, reader *ids.UUID) (bool, error) {
	if row.BoardID == nil {
		return true, nil
	}
	b, err := q.FindFeedbackBoard(ctx, models.FindFeedbackBoardParams{WorkspaceID: row.WorkspaceID, ID: *row.BoardID})
	if err != nil {
		return false, missing(err, ErrBoardNotFound)
	}
	return readable(b, reader != nil), nil
}

/*
listing is a list the portal reads: the request, the boards it covers
and the status it is kept to, if any.
*/
type listing struct {
	BoardRequest
	boards []ids.UUID
	status *string
}

func (p *Posts) listed(ctx context.Context, q *models.Queries, in listing) ([]Listed, error) {
	rows, err := q.ListFeedbackPosts(ctx, models.ListFeedbackPostsParams{
		WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, BoardIds: in.boards, Status: in.status,
		Moderation: new("approved"), Query: nonEmpty(strings.TrimSpace(in.Query)), Sort: sortOf(in.Sort),
		RowLimit: boardLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list board requests: %w", err)
	}
	return votedOn(ctx, q, in.Reader, rows)
}

/*
votedOn marks the requests the reader voted for.
*/
func votedOn(ctx context.Context, q *models.Queries, reader *ids.UUID, rows []models.FeedbackPost) ([]Listed, error) {
	out := make([]Listed, len(rows))
	for i, r := range rows {
		out[i] = Listed{FeedbackPost: r}
	}
	if reader == nil || len(rows) == 0 {
		return out, nil
	}
	postIDs := make([]ids.UUID, len(rows))
	for i, r := range rows {
		postIDs[i] = r.ID
	}
	voted, err := q.ListVotedFeedbackPosts(ctx, models.ListVotedFeedbackPostsParams{
		WorkspaceID: rows[0].WorkspaceID, ContactID: *reader, PostIds: postIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list reader votes: %w", err)
	}
	for i := range out {
		for _, v := range voted {
			if v == out[i].ID {
				out[i].Voted = true
			}
		}
	}
	return out, nil
}

/*
Page is one request as its page shows it: the request, its rendered
details, where it stands, its public comments, where it was merged to,
and the slug of the changelog update that shipped it.
*/
type Page struct {
	Listed
	Stage    Status
	Author   string
	Details  string
	Comments []models.ListFeedbackCommentsRow
	MergedTo *models.FeedbackPost
	Update   string
}

/*
Open returns the request with the number as the reader sees it. Found
is false for one that does not exist, waits for approval and is not
theirs, or was marked spam.
*/
func (p *Posts) Open(ctx context.Context, h models.HelpCenter, number int32, reader *ids.UUID) (Page, bool, error) {
	var (
		out   Page
		found bool
	)
	err := p.inside(ctx, h, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindFeedbackPostByNumber(ctx, models.FindFeedbackPostByNumberParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Number: number,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !visible(row, reader)) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find board request: %w", err)
		}
		if found, err = shown(ctx, q, row, reader); err != nil || !found {
			return err
		}
		out, err = p.page(ctx, q, row, reader)
		return err
	})
	return out, found, err
}

func visible(row models.FeedbackPost, reader *ids.UUID) bool {
	if row.Moderation == "approved" {
		return true
	}
	mine := reader != nil && row.AuthorContactID != nil && *row.AuthorContactID == *reader
	return row.Moderation == "pending" && mine
}

func (p *Posts) page(ctx context.Context, q *models.Queries, row models.FeedbackPost, reader *ids.UUID) (Page, error) {
	listed, err := votedOn(ctx, q, reader, []models.FeedbackPost{row})
	if err != nil {
		return Page{}, err
	}
	named, err := one(ctx, q, row.WorkspaceID, row)
	if err != nil {
		return Page{}, err
	}
	out := Page{Listed: listed[0], Author: named.Author}
	statuses, err := ordered(ctx, q, row.WorkspaceID)
	if err != nil {
		return Page{}, err
	}
	if s, ok := statusIn(statuses, row.Status); ok {
		out.Stage = s
	}
	if html, err := p.Renderer.HTML([]byte(row.BodyMarkdown)); err == nil {
		out.Details = html.HTML
	}
	if out.Comments, err = q.ListFeedbackComments(ctx, models.ListFeedbackCommentsParams{
		WorkspaceID: row.WorkspaceID, PostID: row.ID,
	}); err != nil {
		return out, fmt.Errorf("list board comments: %w", err)
	}
	if row.MergedIntoID != nil {
		if target, err := findPost(ctx, q, row.WorkspaceID, *row.MergedIntoID); err == nil {
			out.MergedTo = &target
		}
	}
	if row.ChangelogEntryID != nil {
		entry, err := q.FindChangelogEntry(ctx, models.FindChangelogEntryParams{
			WorkspaceID: row.WorkspaceID, ID: *row.ChangelogEntryID,
		})
		if err == nil && entry.Status == "published" {
			out.Update = entry.Slug
		}
	}
	return out, nil
}

/*
inside runs work in the help center's workspace.
*/
func (p *Posts) inside(ctx context.Context, h models.HelpCenter,
	fn func(context.Context, *models.Queries) error,
) error {
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	return p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}
