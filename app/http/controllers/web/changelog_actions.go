/*
|--------------------------------------------------------------------------
| Changelog Pages and Forms
|--------------------------------------------------------------------------
|
| What a real changelog answers: its pages, their Markdown twins, the RSS
| feed, and the plain forms readers post. Subscribing never says whether
| an address was already subscribed: every address gets the same answer.
| Reactions count once per reader, known by their sign-in when they have
| one and otherwise by where they connect from. A subscriber's link opens
| a page with a button, and only pressing it changes anything.
|
*/

package web

import (
	"errors"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/view/feed"

	cl "github.com/dimmihq/dimmi/app/actions/changelog"
	views "github.com/dimmihq/dimmi/resources/views/changelog"
)

/*
realIndex shows a page of a real changelog, or not found past the end
or for a label it does not have.
*/
func (c ChangelogController) realIndex(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite, page int) {
	label := r.PathValue("label")
	p, found, err := c.Changelog.Index(r.Context(), cl.PageRequest{
		Center: s.help, Label: label, Page: page, Size: changelogPerPage,
	})
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if !found || (len(p.Entries) == 0 && label == "") {
		c.NotFound(w, r)
		return
	}
	index := indexView(p, label, page)
	index.Subscribed = r.URL.Query().Has("subscribed")
	c.respond(w, r, fhttp.StatusOK, views.IndexPage(s.center, index, c.styles()))
}

/*
realEntry shows a published entry, or its Markdown when the path ends
in .md.
*/
func (c ChangelogController) realEntry(w fhttp.ResponseWriter, r *fhttp.Request, s liveSite) {
	slug, markdown := strings.CutSuffix(r.PathValue("slug"), ".md")
	one, found, err := c.Changelog.Entry(r.Context(), s.help, slug)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if !found {
		c.NotFound(w, r)
		return
	}
	if markdown {
		writeMarkdown(w, "# "+deref(one.PublishedTitle)+"\n\n> "+deref(one.PublishedSummary)+"\n\n"+
			deref(one.PublishedBodyMarkdown))
		return
	}
	labels, err := c.labelsOf(r, s)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	c.respond(w, r, fhttp.StatusOK, views.EntryPage(s.center, labels, singleView(one), c.styles()))
}

func (c ChangelogController) labelsOf(r *fhttp.Request, s liveSite) ([]views.Label, error) {
	p, _, err := c.Changelog.Index(r.Context(), cl.PageRequest{Center: s.help, Page: 1, Size: 1})
	return labelViews(p.Labels), err
}

/*
Feed answers the changelog's newest entries as RSS, all of them or
those under a label.
*/
func (c ChangelogController) Feed(w fhttp.ResponseWriter, r *fhttp.Request) {
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	if !live {
		c.NotFound(w, r)
		return
	}
	label := r.PathValue("label")
	p, found, err := c.Changelog.Index(r.Context(), cl.PageRequest{Center: s.help, Label: label, Page: 1, Size: 30})
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	if !found {
		c.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(fhttp.StatusOK)
	_ = feed.RSS(w, feedOf(s, p.Entries))
}

/*
feedOf is the changelog's feed of entries, with links that work away
from the site.
*/
func feedOf(s liveSite, entries []cl.Published) feed.Feed {
	root := s.center.Origin + s.center.Base
	out := feed.Feed{
		Title: s.help.Name + " changelog", Link: root + "/changelog",
		Description: "New features, improvements and fixes in " + s.help.Name + ".",
	}
	for _, e := range entries {
		out.Items = append(out.Items, feed.Item{
			ID: "urn:uuid:" + e.ID.String(), Title: deref(e.PublishedTitle), Link: root + "/changelog/" + e.Slug,
			Description: deref(e.PublishedSummary), Content: absoluteHTML(deref(e.PublishedBodyHtml), s.center.Origin),
			Published: *e.PublishedAt, Updated: e.UpdatedAt,
		})
	}
	if len(out.Items) > 0 {
		out.Updated = out.Items[0].Published
	}
	return out
}

func absoluteHTML(html, origin string) string {
	return strings.NewReplacer(`src="/`, `src="`+origin+`/`, `href="/`, `href="`+origin+`/`).Replace(html)
}

/*
Subscribe takes an email address for the changelog's updates, sends the
confirmation, and shows the changelog asking the visitor to check their
inbox. A filled hidden field marks a bot, which gets the same answer.
*/
func (c ChangelogController) Subscribe(w fhttp.ResponseWriter, r *fhttp.Request) {
	to := strings.TrimSuffix(r.URL.Path, "/subscriptions") + "?subscribed#main"
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	if !live || r.FormValue("website") != "" || c.Subscribers == nil {
		back(w, to)
		return
	}
	_, err := c.Subscribers.Subscribe(r.Context(), cl.SubscribeInput{Center: s.help, Email: r.FormValue("email")})
	if errors.Is(err, cl.ErrEmailInvalid) {
		back(w, strings.TrimSuffix(r.URL.Path, "/subscriptions")+"#main")
		return
	}
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	back(w, to)
}

/*
React takes a reaction to an entry and returns to the entry.
*/
func (c ChangelogController) React(w fhttp.ResponseWriter, r *fhttp.Request) {
	to := strings.TrimSuffix(r.URL.Path, "/reactions")
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	if !live {
		back(w, to)
		return
	}
	name := nameOfEmoji(r.FormValue("emoji"))
	reactor := "ip:" + clientIP(r)
	if s.reader != nil {
		reactor = "contact:" + s.reader.String()
	}
	_, err := c.Changelog.React(r.Context(), cl.Reaction{
		Center: s.help, Slug: r.PathValue("slug"), Emoji: name, Reactor: reactor,
	})
	if err != nil && !errors.Is(err, cl.ErrEmojiInvalid) {
		c.Errors.Render(w, r, err)
		return
	}
	back(w, to)
}

/*
nameOfEmoji is the stored name of a reaction posted as its character.
*/
func nameOfEmoji(posted string) string {
	for name, r := range emojiOf() {
		if r.Emoji == posted || name == posted {
			return name
		}
	}
	return posted
}

func clientIP(r *fhttp.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

/*
SubscriberLink shows the page a subscriber's link opens, asking to
confirm or to unsubscribe.
*/
func (c ChangelogController) SubscriberLink(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.subscriberPage(w, r, false)
}

/*
SubscriberAction confirms or unsubscribes, from the page's button or a
mail app's one-click unsubscribe.
*/
func (c ChangelogController) SubscriberAction(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.subscriberPage(w, r, true)
}

func (c ChangelogController) subscriberPage(w fhttp.ResponseWriter, r *fhttp.Request, act bool) {
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}
	purpose := r.PathValue("purpose")
	id, err := ids.Parse(r.PathValue("subscriber"))
	if !live || err != nil || (purpose != cl.PurposeConfirm && purpose != cl.PurposeUnsubscribe) {
		c.NotFound(w, r)
		return
	}
	link := cl.LinkRequest{Center: s.help, ID: id, Purpose: purpose, Token: r.PathValue("token")}
	page := views.Subscription{Purpose: purpose, Action: r.URL.Path}
	sub, err := c.Subscribers.Check(r.Context(), link)
	switch {
	case errors.Is(err, cl.ErrLinkInvalid):
		page.Invalid = true
	case err != nil:
		c.Errors.Render(w, r, err)
		return
	case act && purpose == cl.PurposeConfirm:
		sub, err = c.Subscribers.Confirm(r.Context(), link)
		page.Done = err == nil
	case act:
		sub, err = c.Subscribers.Unsubscribe(r.Context(), link)
		page.Done = err == nil
	}
	if err != nil && !errors.Is(err, cl.ErrLinkInvalid) {
		c.Errors.Render(w, r, err)
		return
	}
	page.Email = sub.Email
	w.Header().Set("Cache-Control", "private, no-store")
	c.respond(w, r, fhttp.StatusOK, views.SubscriptionPage(s.center, page, c.styles()))
}
