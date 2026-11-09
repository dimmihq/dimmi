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

/*
Week is a week of the release strip: the month it opens, when it is the
first week of one, and the entry published in it, if any.
*/
type Week struct {
	Month string
	Title string
	Path  string
}

/*
Page is a single entry with the entries before and after it.
*/
type Page struct {
	Entry    Entry
	Previous *Entry
	Next     *Entry
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

func subscriptionTitle(s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return "Unsubscribe from updates"
	}
	return "Confirm your subscription"
}

func subscriptionAsk(s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return s.Email + " will stop getting product updates."
	}
	return "Start getting product updates at " + s.Email + "."
}

func subscriptionButton(s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return "Unsubscribe"
	}
	return "Confirm subscription"
}

func subscriptionDone(s Subscription) string {
	if s.Purpose == "unsubscribe" {
		return "Done. " + s.Email + " will not get any more updates."
	}
	return "You're subscribed. New updates will reach " + s.Email + "."
}
