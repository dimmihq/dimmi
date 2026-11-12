/*
|--------------------------------------------------------------------------
| Feature Requests
|--------------------------------------------------------------------------
|
| A post is one request, numbered per workspace. Customers post them from
| the portal, signed in; teammates post them too, for a customer or for
| themselves. When the help center asks to see posts first, a customer's
| post waits for approval before anyone else sees it. The team lists
| them by status, approval, title and order: trending, top or new.
|
*/

package feedback

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing feature requests.
*/
var (
	ErrPostNotFound = fault.New(fault.NotFound, "feedback_post_not_found",
		"No feature request of this workspace has this id.")
	ErrTitleInvalid = fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
	ErrBodyTooLong  = fault.New(fault.Invalid, "body_too_long",
		"Details are at most 64 KB.").OnField("body_markdown")
	ErrStatusInvalid = fault.New(fault.Invalid, "status_invalid",
		"The status is the key of one of the workspace's statuses.").OnField("status")
)

/*
TotalCap is the most a count of posts goes up to.
*/
const TotalCap = 10_000

/*
Posts manages feature requests. Mail tells voters when a request moves
and Addresses says where its portal lives.
*/
type Posts struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *fmail.Manager
	Addresses helpcenters.Addresses
	Renderer  *markdown.Renderer
	Events    *events.Dispatcher
}

/*
NewPosts returns the posts action, rendering details as comments are:
plain Markdown without raw HTML.
*/
func NewPosts(db *database.DB, clk clock.Clock, mail *fmail.Manager, addresses helpcenters.Addresses) *Posts {
	return &Posts{
		DB: db, Clock: clk, Mail: mail, Addresses: addresses,
		Renderer: markdown.New(markdown.Options{GFM: true, HardWraps: true, Highlight: true}),
	}
}

/*
Post is a request with the name of who posted it.
*/
type Post struct {
	models.FeedbackPost
	Author string
}

/*
PostInput is a request to post or a change to one; nil leaves a field.
Contact is the customer a teammate posts it for, and Board the board it
goes on, the help center's first when nil.
*/
type PostInput struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	ID           ids.UUID
	Title        *string
	Body         *string
	Contact      *ids.UUID
	Board        *ids.UUID
}

/*
PostFilter narrows a page of a help center's requests, to one board
when Board is set. Sort is trending, top or new.
*/
type PostFilter struct {
	HelpCenterID ids.UUID
	Board        *ids.UUID
	Status       string
	Moderation   string
	Query        string
	Sort         string
	Limit        int
	Offset       int
}

/*
PostPage is a page of requests, how many match up to TotalCap, and how
many approved requests are in each status, with those waiting for
approval under pending.
*/
type PostPage struct {
	Posts    []Post
	Total    int64
	ByStatus map[string]int32
}

/*
List returns a page of a help center's requests.
*/
func (p *Posts) List(ctx context.Context, m authz.Membership, f PostFilter) (PostPage, error) {
	if !m.Can("feedback:read") {
		return PostPage{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	status, moderation, query := nonEmpty(f.Status), nonEmpty(f.Moderation), nonEmpty(strings.TrimSpace(f.Query))
	var out PostPage
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		rows, err := q.ListFeedbackPosts(ctx, models.ListFeedbackPostsParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, BoardID: f.Board, Status: status, Moderation: moderation,
			Query: query, Sort: sortOf(f.Sort), RowLimit: int32Of(f.Limit), RowOffset: int32Of(f.Offset),
		})
		if err != nil {
			return fmt.Errorf("list feature requests: %w", err)
		}
		if out.Posts, err = named(ctx, q, ws, rows); err != nil {
			return err
		}
		if out.Total, err = q.CountFeedbackPosts(ctx, models.CountFeedbackPostsParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, BoardID: f.Board, Status: status, Moderation: moderation,
			Query: query,
		}); err != nil {
			return fmt.Errorf("count feature requests: %w", err)
		}
		out.ByStatus, err = countByStatus(ctx, q, ws, f)
		return err
	})
	return out, err
}

func countByStatus(ctx context.Context, q *models.Queries, ws ids.UUID, f PostFilter) (map[string]int32, error) {
	rows, err := q.CountFeedbackByStatus(ctx, models.CountFeedbackByStatusParams{
		WorkspaceID: ws, HelpCenterID: f.HelpCenterID, BoardID: f.Board,
	})
	if err != nil {
		return nil, fmt.Errorf("count feature requests by status: %w", err)
	}
	statuses, err := ordered(ctx, q, ws)
	if err != nil {
		return nil, err
	}
	out := map[string]int32{"pending": 0}
	for _, s := range statuses {
		out[s.Key] = 0
	}
	for _, r := range rows {
		out[r.Bucket] = r.Posts
	}
	return out, nil
}

/*
Show returns one request.
*/
func (p *Posts) Show(ctx context.Context, m authz.Membership, id ids.UUID) (Post, error) {
	if !m.Can("feedback:read") {
		return Post{}, exceptions.ErrForbidden
	}
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := findPost(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		out, err = one(ctx, q, m.Workspace.ID, row)
		return err
	})
	return out, err
}

/*
Create posts a request from the team, for a customer when one is named;
it needs no approval.
*/
func (p *Posts) Create(ctx context.Context, in PostInput) (Post, error) {
	if !in.Membership.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	title, body, err := checked(in.Title, in.Body)
	if err != nil {
		return Post{}, err
	}
	ws := in.Membership.Workspace.ID
	var out Post
	err = p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		at := models.FindHelpCenterParams{WorkspaceID: ws, ID: in.HelpCenterID}
		center, err := q.FindHelpCenter(ctx, at)
		if err != nil {
			return missing(err, helpcenters.ErrHelpCenterNotFound)
		}
		board, err := boardOf(ctx, q, center, in.Board)
		if err != nil {
			return err
		}
		row, err := p.insert(ctx, q, draft{
			ws: ws, hc: in.HelpCenterID, board: board.ID, title: title, body: body, moderation: "approved",
			contact: in.Contact, member: memberOf(in.Membership),
		})
		if err != nil {
			return err
		}
		if in.Contact != nil {
			if err := castVote(ctx, q, vote{
				post: row, contact: *in.Contact, member: memberOf(in.Membership),
				priority: "nice", now: p.Clock.Now(),
			}); err != nil {
				return err
			}
		}
		created := appevents.FeedbackPostCreated{Base: p.byMember(ws, in.Membership.Member.ID), PostID: row.ID}
		if err := p.announce(ctx, tx, created); err != nil {
			return err
		}
		out, err = one(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
draft is a request about to be stored.
*/
type draft struct {
	ws, hc      ids.UUID
	board       ids.UUID
	title, body string
	moderation  string
	contact     *ids.UUID
	member      *ids.UUID
}

func (p *Posts) insert(ctx context.Context, q *models.Queries, d draft) (models.FeedbackPost, error) {
	number, err := q.NextFeedbackNumber(ctx, d.ws)
	if err != nil {
		return models.FeedbackPost{}, fmt.Errorf("number feature request: %w", err)
	}
	slug := helpcenters.Slugify(d.title, 120)
	if slug == "" {
		slug = "request"
	}
	statuses, err := ordered(ctx, q, d.ws)
	if err != nil {
		return models.FeedbackPost{}, err
	}
	row, err := q.CreateFeedbackPost(ctx, models.CreateFeedbackPostParams{
		WorkspaceID: d.ws, HelpCenterID: d.hc, BoardID: &d.board, Number: number, Title: d.title, Slug: slug,
		BodyMarkdown: d.body, Status: startingStatus(statuses), Moderation: d.moderation, AuthorContactID: d.contact,
		AuthorMemberID: d.member, Now: p.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("create feature request: %w", err)
	}
	return row, nil
}

/*
Update rewrites a request's title and details, and moves it to another
board of its help center when Board is set.
*/
func (p *Posts) Update(ctx context.Context, in PostInput) (Post, error) {
	if !in.Membership.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	ws := in.Membership.Workspace.ID
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockPost(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}
		title, body := current.Title, current.BodyMarkdown
		if in.Title != nil {
			title = *in.Title
		}
		if in.Body != nil {
			body = *in.Body
		}
		if title, body, err = checked(&title, &body); err != nil {
			return err
		}
		row, err := q.UpdateFeedbackPost(ctx, models.UpdateFeedbackPostParams{
			Title: title, Slug: slugOr(title, current.Slug), BodyMarkdown: body, Now: p.Clock.Now(),
			WorkspaceID: ws, ID: current.ID,
		})
		if err != nil {
			return fmt.Errorf("update feature request: %w", err)
		}
		if row, err = p.moveBoard(ctx, q, row, in.Board); err != nil {
			return err
		}
		out, err = one(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
moveBoard puts the request on another board of its help center, when
one is named.
*/
func (p *Posts) moveBoard(ctx context.Context, q *models.Queries, row models.FeedbackPost, board *ids.UUID,
) (models.FeedbackPost, error) {
	if board == nil {
		return row, nil
	}
	b, err := q.FindFeedbackBoard(ctx, models.FindFeedbackBoardParams{WorkspaceID: row.WorkspaceID, ID: *board})
	if err != nil || b.HelpCenterID != row.HelpCenterID {
		return row, ErrBoardNotFound
	}
	out, err := q.SetFeedbackPostBoard(ctx, models.SetFeedbackPostBoardParams{
		BoardID: &b.ID, Now: p.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return row, fmt.Errorf("move feature request to board: %w", err)
	}
	return out, nil
}

/*
Delete deletes a request.
*/
func (p *Posts) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("feedback:delete") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := lockPost(ctx, q, ws, id); err != nil {
			return err
		}
		if err := q.DeleteFeedbackPost(ctx, models.DeleteFeedbackPostParams{
			Now: new(p.Clock.Now()), WorkspaceID: ws, ID: id,
		}); err != nil {
			return fmt.Errorf("delete feature request: %w", err)
		}
		return nil
	})
}

/*
checked trims a title and details and refuses ones out of bounds.
*/
func checked(title, body *string) (string, string, error) {
	t := strings.TrimSpace(deref(title))
	if t == "" || len([]rune(t)) > 150 {
		return "", "", ErrTitleInvalid
	}
	b := strings.TrimSpace(deref(body))
	if len(b) > 65536 {
		return "", "", ErrBodyTooLong
	}
	return t, b, nil
}

func slugOr(title, fallback string) string {
	if s := helpcenters.Slugify(title, 120); s != "" {
		return s
	}
	return fallback
}

/*
named adds the names of the requests' authors.
*/
func named(ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.FeedbackPost) ([]Post, error) {
	postIDs := make([]ids.UUID, len(rows))
	for i, r := range rows {
		postIDs[i] = r.ID
	}
	names, err := q.FindFeedbackAuthorNames(ctx, models.FindFeedbackAuthorNamesParams{
		WorkspaceID: ws, PostIds: postIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("name feature request authors: %w", err)
	}
	byID := make(map[ids.UUID]string, len(names))
	for _, n := range names {
		byID[n.ID] = n.AuthorName
	}
	out := make([]Post, len(rows))
	for i, r := range rows {
		out[i] = Post{FeedbackPost: r, Author: byID[r.ID]}
	}
	return out, nil
}

func one(ctx context.Context, q *models.Queries, ws ids.UUID, row models.FeedbackPost) (Post, error) {
	out, err := named(ctx, q, ws, []models.FeedbackPost{row})
	if err != nil {
		return Post{}, err
	}
	return out[0], nil
}

func findPost(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.FeedbackPost, error) {
	out, err := q.FindFeedbackPost(ctx, models.FindFeedbackPostParams{WorkspaceID: ws, ID: id})
	return out, missing(err, ErrPostNotFound)
}

func lockPost(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.FeedbackPost, error) {
	out, err := q.LockFeedbackPost(ctx, models.LockFeedbackPostParams{WorkspaceID: ws, ID: id})
	return out, missing(err, ErrPostNotFound)
}

/*
missing maps a missing row to the feature's own failure.
*/
func missing(err, notFound error) error {
	if errors.Is(err, database.ErrNotFound) {
		return notFound
	}
	if err != nil {
		return fmt.Errorf("read feature requests: %w", err)
	}
	return nil
}

func sortOf(s string) string {
	if s == "top" || s == "new" {
		return s
	}
	return "trending"
}

func memberOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}
	return &m.Member.ID
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}
