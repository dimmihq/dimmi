/*
|--------------------------------------------------------------------------
| Real Feature Requests
|--------------------------------------------------------------------------
|
| A real help center's board, once the team opens it: the roadmap or a
| sorted, searched list, each request's page, and the forms customers
| post, vote and comment with. Reading needs nothing; acting needs the
| reader signed in, and anyone else is sent to sign in and brought back
| to the page they were on. Every form answers with a redirect, so a
| reload never posts it twice.
|
*/

package web

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
open says whether the help center's board is open to customers,
answering not found when it is not.
*/
func (c PortalController) open(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) bool {
	if c.Board == nil || !helpcenters.SettingsOf(s.help).FeedbackBoard {
		c.NotFound(w, r)
		return false
	}
	return true
}

/*
signInFor is where a reader signs in to act on the board and comes back
to the page at path.
*/
func signInFor(s liveSite, path string) string {
	settings := helpcenters.SettingsOf(s.help)
	next := s.center.Base + path
	if settings.EmailSignInOn() {
		return s.center.Base + "/sso/sign-in?next=" + url.QueryEscape(next)
	}
	return productLogin(settings.LoginURL, s.center, next)
}

func statusView(s feedback.Status, count int) helpcenter.Status {
	return helpcenter.Status{Name: s.Name, Slug: s.Slug, Color: s.Color, Count: count}
}

func ideaView(p feedback.Listed) helpcenter.Idea {
	details := strings.TrimSpace(p.BodyMarkdown)
	if first, _, cut := strings.Cut(details, "\n"); cut {
		details = first
	}
	return helpcenter.Idea{
		Slug: numbered(p.FeedbackPost), Title: p.Title, Details: details, Votes: int(p.VoteCount), Voted: p.Voted,
		Comments: int(p.CommentCount), Path: feedback.PostPath(p.FeedbackPost),
	}
}

func numbered(p models.FeedbackPost) string {
	return strconv.Itoa(int(p.Number)) + "-" + p.Slug
}

/*
realBoard shows the board as a roadmap, or one list when sorted or
searched.
*/
func (c PortalController) realBoard(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) {
	if !c.open(w, r, s) {
		return
	}
	query := r.URL.Query()
	board, err := c.Board.Read(r.Context(), feedback.BoardRequest{
		Center: s.help, Reader: s.reader, Sort: query.Get("sort"), Query: query.Get("q"), Board: query.Get("board"),
	})
	if errors.Is(err, feedback.ErrBoardNotFound) {
		c.NotFound(w, r)
		return
	}
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	statuses, err := c.columns(r, s, query.Get("board"))
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	out := helpcenter.Board{
		Boards: boardTabs(board.Boards), Statuses: statuses, Sort: query.Get("sort"), Query: query.Get("q"),
		Submitted: query.Has("submitted"), Pending: query.Get("submitted") == "pending",
	}
	if board.Active != nil {
		out.Active = board.Active.Slug
	}
	for _, p := range board.List {
		out.List = append(out.List, ideaView(p))
	}
	if s.reader == nil {
		s.center.SignInURL = signInFor(s, "/feedback")
	}
	w.Header().Set("Cache-Control", "private, no-store")
	c.respond(w, r, fhttp.StatusOK, helpcenter.FeedbackPage(s.center, out, c.styles()))
}

/*
boardTabs shapes the boards a reader may open as the page's tabs.
*/
func boardTabs(boards []models.FeedbackBoard) []helpcenter.BoardTab {
	out := make([]helpcenter.BoardTab, len(boards))
	for i, b := range boards {
		out[i] = helpcenter.BoardTab{Name: b.Name, Slug: b.Slug, Description: b.Description}
	}
	return out
}

/*
columns reads the roadmap: each status it shows with its requests, on
one board when slug names one.
*/
func (c PortalController) columns(r *fhttp.Request, s liveSite, slug string) ([]helpcenter.Status, error) {
	board, err := c.Board.Read(r.Context(), feedback.BoardRequest{Center: s.help, Reader: s.reader, Board: slug})
	if err != nil {
		return nil, err
	}
	out := make([]helpcenter.Status, 0, len(board.Columns))
	for _, col := range board.Columns {
		v := statusView(col.Status, len(col.Posts))
		for _, p := range col.Posts {
			v.Ideas = append(v.Ideas, ideaView(p))
		}
		out = append(out, v)
	}
	return out, nil
}

/*
realNewIdea shows the form to post a request, or sends the reader to
sign in first.
*/
func (c PortalController) realNewIdea(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) {
	if !c.open(w, r, s) {
		return
	}
	if s.reader == nil {
		c.signInFirst(w, s, "/feedback/new")
		return
	}
	query := r.URL.Query()
	form := helpcenter.IdeaForm{Title: query.Get("title"), Details: query.Get("details"), Board: query.Get("board")}
	boards, err := c.Board.BoardsFor(r.Context(), s.help, s.reader)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	form.Boards = boardTabs(boards)
	if query.Has("error") {
		form.Error = "Give your idea a title of up to 150 characters."
	}
	if query.Has("similar") && form.Title != "" {
		similar, err := c.Board.Similar(r.Context(), s.help, s.reader, form.Title)
		if err != nil {
			c.Errors.Render(w, r, err)
			return
		}
		for _, p := range similar {
			form.Similar = append(form.Similar, ideaView(feedback.Listed{FeedbackPost: p}))
		}
	}
	c.ideaForm(w, r, s, form)
}

/*
again sends the reader back to the form with what they typed, and why.
*/
func again(w fhttp.ResponseWriter, s liveSite, form helpcenter.IdeaForm, why string) {
	values := url.Values{"title": {form.Title}, "details": {form.Details}, "board": {form.Board}, why: {"1"}}
	back(w, s.center.Base+"/feedback/new?"+values.Encode())
}

/*
ideaForm shows the form to post a request.
*/
func (c PortalController) ideaForm(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite, form helpcenter.IdeaForm) {
	w.Header().Set("Cache-Control", "private, no-store")
	c.respond(w, r, fhttp.StatusOK, helpcenter.NewIdeaPage(s.center, form, c.styles()))
}

func (c PortalController) signInFirst(w fhttp.ResponseWriter, s liveSite, path string) {
	back(w, signInTarget(s, path))
}

/*
signInTarget is where a reader signs in before acting on the page at
path, or the page itself when the help center takes no sign-in.
*/
func signInTarget(s liveSite, path string) string {
	if to := signInFor(s, path); to != "" {
		return to
	}
	return s.center.Base + path
}

/*
realSubmit posts a signed-in reader's request, showing the similar ones
first when there are any.
*/
func (c PortalController) realSubmit(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) {
	if !c.open(w, r, s) {
		return
	}
	if s.reader == nil {
		c.signInFirst(w, s, "/feedback/new")
		return
	}
	form := helpcenter.IdeaForm{
		Title: r.FormValue("title"), Details: r.FormValue("details"), Board: r.FormValue("board"),
	}
	done, err := c.Board.Submit(r.Context(), feedback.SubmitInput{
		Center: s.help, Reader: *s.reader, Board: form.Board, Title: form.Title, Body: form.Details,
		Sure: r.FormValue("sure") != "",
	})
	if errors.Is(err, feedback.ErrBoardNotFound) {
		c.NotFound(w, r)
		return
	}
	if errors.Is(err, feedback.ErrTitleInvalid) || errors.Is(err, feedback.ErrBodyTooLong) {
		again(w, s, form, "error")
		return
	}
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if done.Post == nil {
		again(w, s, form, "similar")
		return
	}
	if done.Post.Moderation == "pending" {
		back(w, s.center.Base+"/feedback?submitted=pending")
		return
	}
	back(w, s.center.Base+feedback.PostPath(*done.Post))
}

/*
realVote counts or takes back a signed-in reader's vote, returning to
the page they voted from. Sent in the background, it answers where the
vote stands, or where to sign in first.
*/
func (c PortalController) realVote(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) {
	if !c.open(w, r, s) {
		return
	}
	number, ok := numberOf(r.PathValue("idea"))
	from := returnPath(r, s, "/feedback")
	if !ok {
		c.NotFound(w, r)
		return
	}
	if s.reader == nil && inPlace(r) {
		fhttp.WriteJSON(w, fhttp.StatusUnauthorized, map[string]string{"sign_in": signInTarget(s, from)})
		return
	}
	if s.reader == nil {
		c.signInFirst(w, s, from)
		return
	}
	ballot, _, err := c.Board.Toggle(r.Context(), s.help, number, *s.reader)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if inPlace(r) {
		fhttp.WriteJSON(w, fhttp.StatusOK, map[string]any{"voted": ballot.Voted, "votes": ballot.Votes})
		return
	}
	back(w, s.center.Base+from)
}

/*
returnPath is the board page a form came from, when the browser says,
or the fallback.
*/
func returnPath(r *fhttp.Request, s liveSite, fallback string) string {
	ref, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || ref.Host != r.Host || !strings.HasPrefix(ref.Path, s.center.Base+"/feedback") {
		return fallback
	}
	out := strings.TrimPrefix(ref.Path, s.center.Base)
	if ref.RawQuery != "" {
		out += "?" + ref.RawQuery
	}
	return out
}

func numberOf(slug string) (int32, bool) {
	head, _, _ := strings.Cut(slug, "-")
	n, err := strconv.ParseInt(head, 10, 32)
	return int32(n), err == nil && n > 0
}

/*
Idea shows one request's page, sending a merged request's readers on
to the one kept.
*/
func (c PortalController) Idea(w fhttp.ResponseWriter, r *fhttp.Request) {
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	number, valid := numberOf(r.PathValue("idea"))
	if !live || !valid || !c.open(w, r, s) {
		if !live || !valid {
			c.NotFound(w, r)
		}
		return
	}
	page, found, err := c.Board.Open(r.Context(), s.help, number, s.reader)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if !found {
		c.NotFound(w, r)
		return
	}
	if page.MergedTo != nil {
		w.Header().Set("Location", s.center.Base+feedback.PostPath(*page.MergedTo))
		w.WriteHeader(fhttp.StatusMovedPermanently)
		return
	}
	c.ideaPage(w, r, s, page)
}

func (c PortalController) ideaPage(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite, page feedback.Page) {
	path := feedback.PostPath(page.FeedbackPost)
	v := helpcenter.IdeaView{
		Idea: ideaView(page.Listed), Status: statusView(page.Stage, 0), Details: page.Details, Author: page.Author,
		Posted: page.CreatedAt.Format("January 2, 2006"), Comments: threads(page.Comments),
		Pending: page.Moderation == "pending", CanAct: s.reader != nil, Commented: r.URL.Query().Has("commented"),
	}
	if page.Update != "" {
		v.Update = "/changelog/" + page.Update
	}
	if s.reader == nil {
		v.SignIn = signInFor(s, path)
		s.center.SignInURL = v.SignIn
	}
	w.Header().Set("Cache-Control", "private, no-store")
	c.respond(w, r, fhttp.StatusOK, helpcenter.IdeaPage(s.center, v, c.styles()))
}

/*
threads nests replies under the comments they answer.
*/
func threads(rows []models.ListFeedbackCommentsRow) []helpcenter.Comment {
	var out []helpcenter.Comment
	at := map[ids.UUID]int{}
	for _, row := range rows {
		cm := helpcenter.Comment{
			Author: row.AuthorName, FromTeam: row.AuthorMemberID != nil, Body: row.Body,
			At: row.CreatedAt.Format("Jan 2, 2006"),
		}
		if row.ParentID != nil {
			if i, ok := at[*row.ParentID]; ok {
				out[i].Replies = append(out[i].Replies, cm)
				continue
			}
		}
		at[row.ID] = len(out)
		out = append(out, cm)
	}
	return out
}

/*
Comment takes a signed-in reader's comment on a request.
*/
func (c PortalController) Comment(w fhttp.ResponseWriter, r *fhttp.Request) {
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	number, valid := numberOf(r.PathValue("idea"))
	if !live || !valid || !c.open(w, r, s) {
		if !live || !valid {
			c.NotFound(w, r)
		}
		return
	}
	path := "/feedback/" + r.PathValue("idea")
	if s.reader == nil {
		c.signInFirst(w, s, path)
		return
	}
	err := c.Board.Reply(r.Context(), feedback.ReaderComment{
		Center: s.help, Reader: *s.reader, Number: number, Body: r.FormValue("body"),
	})
	if errors.Is(err, feedback.ErrCommentInvalid) {
		back(w, s.center.Base+path)
		return
	}
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	back(w, s.center.Base+path+"?commented#comments-title")
}
