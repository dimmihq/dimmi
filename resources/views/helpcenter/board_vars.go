/*
|--------------------------------------------------------------------------
| Theme Variables: Feature Requests
|--------------------------------------------------------------------------
|
| The board of feature requests as its pages read it.
|
*/

package helpcenter

import (
	"net/url"

	"github.com/dimmihq/dimmi/framework/liquid"
)

/*
IdeaVars is a feature request.
*/
type IdeaVars struct {
	// Its slug.
	Slug string
	// What is asked for.
	Title string
	// Its details as plain text.
	Details string
	// Its icon's name, lightbulb by default.
	Icon string
	// How many voted for it.
	Votes int
	// The reader voted for it.
	Voted bool
	// How many comments it has.
	Comments int
	// Its own page, empty where it has none.
	URL string
	// Where its vote form posts; posting again takes the vote back.
	VoteURL string
}

/*
StatusVars is a stage feature requests move through.
*/
type StatusVars struct {
	// Its name.
	Name string
	// Its slug: planned, in-progress, shipped, closed or another.
	Slug string
	// Its color as a hex code.
	Color string
	// How many requests are in it.
	Count int
	// Its requests, most voted first.
	Ideas []IdeaVars
}

/*
LinkVars is a tab or chip that narrows a page.
*/
type LinkVars struct {
	// Its label.
	Name string
	// Where it leads.
	URL string
	// It is the one shown.
	Current bool
	// Its color, where it has one.
	Color string
	// Its count, where it has one.
	Count int
}

/*
BoardVars is board, on the feature requests page.
*/
type BoardVars struct {
	// The boards the reader may open.
	Boards []BoardTab
	// All boards, then each board, as tabs.
	Tabs []LinkVars
	// The slug of the board shown, empty for all.
	Active string
	// Roadmap, Trending, Top and New.
	Sorts []LinkVars
	// The order picked, empty for the roadmap.
	Sort string
	// What the reader searched for.
	Query string
	// The page shows one sorted list rather than the roadmap.
	Listed bool
	// The list, when listed.
	List []IdeaVars
	// The roadmap's columns, when not listed.
	Statuses []StatusVars
	// The reader just posted a request.
	Submitted bool
	// It waits for the team's approval.
	Pending bool
	// The form to post a request, on the board shown.
	NewURL string
}

/*
CommentVars is a reply under a request.
*/
type CommentVars struct {
	// Who wrote it.
	Author string
	// The team wrote it.
	FromTeam bool
	// A reader's comment, plain text.
	Body string
	// The team's comment, sanitized HTML.
	HTML liquid.HTML
	// When, such as 2d.
	At string
	// The replies to it.
	Replies []CommentVars
}

func (c Center) idea(i Idea) IdeaVars {
	v := IdeaVars{
		Slug: i.Slug, Title: i.Title, Details: i.Details, Icon: i.Icon, Votes: i.Votes, Voted: i.Voted,
		Comments: i.Comments, VoteURL: c.url("/feedback/" + i.Slug + "/votes"),
	}
	if v.Icon == "" {
		v.Icon = "lightbulb"
	}
	if i.Path != "" {
		v.URL = c.url(i.Path)
	}
	return v
}

func (c Center) ideas(list []Idea) []IdeaVars {
	out := make([]IdeaVars, 0, len(list))
	for _, i := range list {
		out = append(out, c.idea(i))
	}
	return out
}

func (c Center) status(s Status) StatusVars {
	return StatusVars{Name: s.Name, Slug: s.Slug, Color: s.Color, Count: s.Count, Ideas: c.ideas(s.Ideas)}
}

/*
boardPath is the board page narrowed to a board and in an order.
*/
func boardPath(board, sort string) string {
	values := url.Values{}
	if board != "" {
		values.Set("board", board)
	}
	if sort != "" {
		values.Set("sort", sort)
	}
	if len(values) == 0 {
		return "/feedback"
	}
	return "/feedback?" + values.Encode()
}

func comments(list []Comment) []CommentVars {
	out := make([]CommentVars, 0, len(list))
	for _, cm := range list {
		out = append(
			out,
			CommentVars{
				Author: cm.Author, FromTeam: cm.FromTeam, Body: cm.Body, HTML: liquid.HTML(cm.HTML),
				At: cm.At, Replies: comments(cm.Replies),
			},
		)
	}
	return out
}
