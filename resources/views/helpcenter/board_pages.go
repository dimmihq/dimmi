/*
|--------------------------------------------------------------------------
| Feature Request Pages
|--------------------------------------------------------------------------
|
| The board of feature requests, a request's own page and the form to
| post one.
|
*/

package helpcenter

import (
	"net/url"

	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/view"
)

/*
RequestViewVars is request, on a feature request's page.
*/
type RequestViewVars struct {
	// The request.
	Idea IdeaVars
	// Where it stands.
	Status StatusVars
	// Its details as HTML.
	Details liquid.HTML
	// Who posted it.
	Author string
	// When.
	Posted string
	// Its comments.
	Comments []CommentVars
	// The changelog entry that shipped it, empty for none.
	UpdateURL string
	// It waits for approval; only its author sees it.
	Pending bool
	// The reader may comment.
	CanAct bool
	// Where to sign in to comment, empty when they need not.
	SignInURL string
	// The reader just commented.
	Commented bool
	// Where the comment form posts, with body.
	CommentURL string
}

/*
IdeaFormVars is form, on the page to post a request.
*/
type IdeaFormVars struct {
	// The boards the reader may post on.
	Boards []BoardTab
	// The slug of the board picked.
	Board string
	// The title typed.
	Title string
	// The details typed.
	Details string
	// Similar requests found, shown before posting.
	Similar []IdeaVars
	// What was wrong, empty for nothing.
	Error string
}

/*
FeedbackPage lists feature requests as a roadmap or one list.
*/
func FeedbackPage(c Center, board Board) view.Component {
	m := Meta{
		Title: c.T("feature_requests") + " | " + c.Product, Description: c.T(
			"feedback_description", "product", c.Product,
		),
		Section: "feedback", Trail: []Crumb{{Title: c.T("feature_requests"), Path: "/feedback"}},
	}
	v := BoardVars{
		Boards: board.Boards, Active: board.Active, Sort: board.Sort, Query: board.Query,
		Listed: board.Sort != "" || board.Query != "", List: c.ideas(board.List),
		Submitted: board.Submitted, Pending: board.Pending, NewURL: c.url("/feedback/new"),
	}
	if board.Active != "" {
		v.NewURL = c.url("/feedback/new?board=" + url.QueryEscape(board.Active))
	}
	for _, s := range board.Statuses {
		v.Statuses = append(v.Statuses, c.status(s))
	}
	v.Tabs = append(
		v.Tabs,
		LinkVars{Name: c.T("all_boards"), URL: c.url(boardPath("", board.Sort)), Current: board.Active == ""},
	)
	for _, b := range board.Boards {
		v.Tabs = append(
			v.Tabs,
			LinkVars{Name: b.Name, URL: c.url(boardPath(b.Slug, board.Sort)), Current: board.Active == b.Slug},
		)
	}
	for _, s := range []struct{ key, word string }{
		{"", "sort_roadmap"}, {"trending", "sort_trending"}, {"top", "sort_top"}, {"new", "sort_new"},
	} {
		v.Sorts = append(v.Sorts, LinkVars{
			Name: c.T(s.word), URL: c.url(boardPath(board.Active, s.key)),
			Current: board.Sort == s.key && (s.key != "" || board.Query == ""),
		})
	}
	return Render(c, "feature_requests", m, map[string]any{"board": v})
}

/*
NewIdeaPage is the form a reader suggests a feature with.
*/
func NewIdeaPage(c Center, form IdeaForm) view.Component {
	m := Meta{
		Title: c.T("submit_an_idea") + " | " + c.Product, Section: "feedback",
		Trail: []Crumb{
			{Title: c.T("feature_requests"), Path: "/feedback"},
			{Title: c.T("submit_an_idea"), Path: "/feedback/new"},
		}, NoIndex: true,
	}
	return Render(c, "feature_request_new", m, map[string]any{"form": IdeaFormVars{
		Boards: form.Boards, Board: form.Board, Title: form.Title, Details: form.Details,
		Similar: c.ideas(form.Similar), Error: form.Error,
	}})
}

/*
IdeaPage is one feature request on its own page.
*/
func IdeaPage(c Center, v IdeaView) view.Component {
	m := Meta{
		Title: v.Idea.Title + " | " + c.Product, Description: v.Idea.Details, Section: "feedback",
		Trail: []Crumb{
			{Title: c.T("feature_requests"), Path: "/feedback"}, {Title: v.Idea.Title, Path: v.Idea.Path},
		}, NoIndex: v.Pending,
	}
	r := RequestViewVars{
		Idea: c.idea(v.Idea), Status: c.status(v.Status), Details: liquid.HTML(v.Details),
		Author: v.Author, Posted: v.Posted, Comments: comments(v.Comments), Pending: v.Pending,
		CanAct: v.CanAct, SignInURL: v.SignIn, Commented: v.Commented,
		CommentURL: c.url(v.Idea.Path + "/comments"),
	}
	if v.Update != "" {
		r.UpdateURL = c.url(v.Update)
	}
	return Render(c, "feature_request", m, map[string]any{"request": r})
}
