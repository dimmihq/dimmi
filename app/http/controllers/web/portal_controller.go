/*
|--------------------------------------------------------------------------
| Portal Controller
|--------------------------------------------------------------------------
|
| The parts of the public portal beside the help center and changelog:
| feature requests and the visitor's own conversations with the team,
| behind a sign-in. A real help center's feature requests are its own;
| the sample portal signs anyone in who asks, with a cookie that only
| says so, and keeps nothing a visitor submits: every form returns to
| the page it belongs to, so the whole portal can be tried end to end.
|
*/

package web

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
visitorCookie marks a browser signed in to the sample portal.
*/
const visitorCookie = "dimmi_portal_visitor"

/*
PortalController serves feature requests, sign-in and conversations. It
shares the help center's library, styles and not found page.
*/
type PortalController struct {
	*HelpCenterController
}

/*
Feedback lists the feature requests by status.
*/
func (c PortalController) Feedback(w fhttp.ResponseWriter, r *fhttp.Request) {
	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realBoard(w, r, site)
		}

		return
	}

	center, ok := c.center(w, r)
	if !ok {
		return
	}

	board := helpcenter.Board{Statuses: c.Library.Feedback(), Submitted: r.URL.Query().Has("submitted")}
	c.respond(w, r, fhttp.StatusOK, helpcenter.FeedbackPage(center, board, c.styles()))
}

/*
NewIdea shows the form to suggest a feature.
*/
func (c PortalController) NewIdea(w fhttp.ResponseWriter, r *fhttp.Request) {
	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realNewIdea(w, r, site)
		}

		return
	}

	center, ok := c.center(w, r)
	if !ok {
		return
	}

	page := helpcenter.NewIdeaPage(center, helpcenter.IdeaForm{}, c.styles())
	c.respond(w, r, fhttp.StatusOK, page)
}

/*
SubmitIdea takes a suggestion and thanks the visitor on the list.
*/
func (c PortalController) SubmitIdea(w fhttp.ResponseWriter, r *fhttp.Request) {
	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realSubmit(w, r, site)
		}

		return
	}

	back(w, r.URL.Path+"?submitted")
}

/*
Vote takes a vote for a request, or takes it back, and returns to the
page the visitor voted from.
*/
func (c PortalController) Vote(w fhttp.ResponseWriter, r *fhttp.Request) {
	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realVote(w, r, site)
		}

		return
	}

	idea := "/" + r.PathValue("idea") + "/votes"
	back(w, strings.TrimSuffix(r.URL.Path, idea))
}

/*
SignIn shows the sign-in form, or the visitor's conversations when they
are signed in already.
*/
func (c PortalController) SignIn(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	if center.Visitor != nil {
		back(w, center.Base+"/conversations")

		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.SignInPage(center, c.Library.Collections(), false, c.styles()))
}

/*
StartSession signs the visitor in and opens their conversations. The
real portal emails a link instead; the sample lets them straight in.
*/
func (c PortalController) StartSession(w fhttp.ResponseWriter, r *fhttp.Request) {
	base := strings.TrimSuffix(r.URL.Path, "/sign-in")
	cookie := visitor(r, base)
	cookie.Value = "1"
	w.Header().Add("Set-Cookie", cookie.String())
	back(w, base+"/conversations")
}

/*
EndSession signs the visitor out and returns to the help center.
*/
func (c PortalController) EndSession(w fhttp.ResponseWriter, r *fhttp.Request) {
	base := strings.TrimSuffix(r.URL.Path, "/sign-out")
	cookie := visitor(r, base)
	cookie.MaxAge = -1
	w.Header().Add("Set-Cookie", cookie.String())
	back(w, base)
}

/*
Conversations lists the signed-in visitor's conversations, sending
anyone else to sign in.
*/
func (c PortalController) Conversations(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.signedIn(w, r)
	if !ok {
		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.ConversationsPage(center, c.Library.Conversations(), c.styles()))
}

/*
Conversation shows one of the signed-in visitor's conversations.
*/
func (c PortalController) Conversation(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.signedIn(w, r)
	if !ok {
		return
	}

	conv, found := c.Library.Conversation(r.PathValue("conversation"))
	if !found {
		c.NotFound(w, r)

		return
	}

	page := helpcenter.ConversationPage(center, c.Library.Conversations(), conv, c.styles())
	c.respond(w, r, fhttp.StatusOK, page)
}

/*
NewConversation shows the form to write to the team, which asks for an
email when the visitor is not signed in.
*/
func (c PortalController) NewConversation(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	var conversations []helpcenter.Conversation
	if center.Visitor != nil {
		conversations = c.Library.Conversations()
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.NewConversationPage(center, conversations, c.styles()))
}

/*
StartConversation takes a new message to the team and opens the
visitor's conversations.
*/
func (c PortalController) StartConversation(w fhttp.ResponseWriter, r *fhttp.Request) {
	back(w, r.URL.Path)
}

/*
Reply takes a reply in a conversation and returns to it.
*/
func (c PortalController) Reply(w fhttp.ResponseWriter, r *fhttp.Request) {
	back(w, strings.TrimSuffix(r.URL.Path, "/messages"))
}

/*
signedIn finds the portal for a page only signed-in visitors see,
sending anyone else to sign in.
*/
func (c PortalController) signedIn(w fhttp.ResponseWriter, r *fhttp.Request) (helpcenter.Center, bool) {
	center, ok := c.center(w, r)
	if !ok {
		return center, false
	}

	if center.Visitor == nil {
		back(w, center.Base+"/sign-in")

		return center, false
	}

	return center, true
}

/*
visitor returns the cookie that marks the browser signed in to the
portal under base, empty and lasting the browser session.
*/
func visitor(r *fhttp.Request, base string) *fhttp.Cookie {
	return &fhttp.Cookie{
		Name: visitorCookie, Path: base, HttpOnly: true, Secure: r.TLS != nil, SameSite: fhttp.SameSiteLaxMode,
	}
}
