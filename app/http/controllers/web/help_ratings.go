/*
|--------------------------------------------------------------------------
| Article Ratings On The Portal
|--------------------------------------------------------------------------
|
| Readers answer "Was this article helpful?" and, after a no, say what
| was missing. The portal's script sends both in the background and moves
| the box on in place; without scripts the forms post and the reader comes
| back to the article on the step the address names.
|
*/

package web

import (
	"errors"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
Rate keeps the answer to "Was this article helpful?" and returns the
visitor to the article, thanked after a yes and asked what was missing
after a no. The sample help center keeps nothing.
*/
func (c *HelpCenterController) Rate(w fhttp.ResponseWriter, r *fhttp.Request) {
	answer := "yes"
	if r.FormValue("helpful") == "no" {
		answer = "no"
	}
	to := strings.TrimSuffix(r.URL.Path, "/feedback") + "?helpful=" + answer
	err := c.rating(w, r, func(in helpcenters.Rating) error {
		in.Helpful = answer == "yes"
		return c.Ratings.Rate(r.Context(), in)
	})
	c.afterRating(w, r, to, err)
}

/*
Comment keeps what a reader who said no was looking for, and thanks
them. A comment with no words leaves them where they were.
*/
func (c *HelpCenterController) Comment(w fhttp.ResponseWriter, r *fhttp.Request) {
	to := strings.TrimSuffix(r.URL.Path, "/feedback/comment") + "?helpful=no"
	err := c.rating(w, r, func(in helpcenters.Rating) error {
		in.Comment = r.FormValue("comment")
		return c.Ratings.Comment(r.Context(), in)
	})
	if errors.Is(err, helpcenters.ErrRatingCommentEmpty) && !inPlace(r) {
		back(w, to+"#feedback")
		return
	}
	if err == nil {
		to += "&commented=1"
	}
	c.afterRating(w, r, to, err)
}

/*
inPlace says the portal's script sent the form in the background and
updates the page itself, so it wants a status rather than a page.
*/
func inPlace(r *fhttp.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

/*
rating hands the reader's rating on the real help center to keep, naming
the reader by their sign-in or their address.
*/
func (c *HelpCenterController) rating(w fhttp.ResponseWriter, r *fhttp.Request,
	keep func(helpcenters.Rating) error,
) error {
	s, live, ok := c.live(w, r)
	if !ok {
		return errHandled
	}
	if !live || c.Ratings == nil {
		return nil
	}
	rater := "ip:" + fhttp.ClientIP(r)
	if s.reader != nil {
		rater = "contact:" + s.reader.String()
	}
	return keep(helpcenters.Rating{Center: s.help, Slug: r.PathValue("slug"), Rater: rater, Contact: s.reader})
}

/*
afterRating returns the reader to the article's feedback, saying so when
they sent too much, and shows any other failure. A form sent in the
background gets only a status: no content when kept, too many requests
when held back, unprocessable for an empty note.
*/
func (c *HelpCenterController) afterRating(w fhttp.ResponseWriter, r *fhttp.Request, to string, err error) {
	if inPlace(r) && !errors.Is(err, errHandled) {
		w.WriteHeader(ratingStatus(err))
		return
	}
	switch {
	case errors.Is(err, errHandled):
	case errors.Is(err, helpcenters.ErrRatingLimited):
		back(w, to+"&limited=1#feedback")
	case err != nil:
		c.Errors.Render(w, r, err)
	default:
		back(w, to+"#feedback")
	}
}

/*
errHandled says a step already answered the request.
*/
var errHandled = errors.New("request already answered")

/*
ratingStatus is the status a background rating gets for its outcome.
*/
func ratingStatus(err error) int {
	switch {
	case err == nil:
		return fhttp.StatusNoContent
	case errors.Is(err, helpcenters.ErrRatingLimited):
		return fhttp.StatusTooManyRequests
	case errors.Is(err, helpcenters.ErrRatingCommentEmpty):
		return fhttp.StatusUnprocessableEntity
	default:
		return fhttp.StatusInternalServerError
	}
}
