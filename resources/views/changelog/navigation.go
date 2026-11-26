/*
|--------------------------------------------------------------------------
| Changelog Navigation
|--------------------------------------------------------------------------
|
| How a visitor moves through the changelog besides scrolling: the strip
| of the year's weeks with a mark for every release, and an entry's
| neighbors before and after it.
|
*/

package changelog

import (
	"time"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
Week is a week of the release strip: the month it opens, when it is the
first week of one, zero otherwise, and the entry published in it, if
any.
*/
type Week struct {
	Month time.Month
	Title string
	Path  string
}

/*
Page is a single entry with the entries before and after it, the
changelog's title, Changelog when empty, and whether it hides
subscribing.
*/
type Page struct {
	Entry       Entry
	Previous    *Entry
	Next        *Entry
	Title       string
	NoSubscribe bool
}

/*
Subscription is a page a subscriber's link opens: what the link does
(confirm or unsubscribe), the address it is for, the form that does it,
and whether it is done or the link no longer works.
*/
type Subscription struct {
	Purpose string
	Email   string
	Action  string
	Done    bool
	Invalid bool
}

func subscriptionTitle(c helpcenter.Center, s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return c.T("unsubscribe_title")
	}
	return c.T("confirm_subscription_title")
}

func subscriptionAsk(c helpcenter.Center, s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return c.T("unsubscribe_ask", "email", s.Email)
	}
	return c.T("subscribe_ask", "email", s.Email)
}

func subscriptionButton(c helpcenter.Center, s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return c.T("unsubscribe")
	}
	return c.T("confirm_subscription")
}

func subscriptionDone(c helpcenter.Center, s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return c.T("unsubscribed", "email", s.Email)
	}
	return c.T("subscribed", "email", s.Email)
}
