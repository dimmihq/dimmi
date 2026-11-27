/*
|--------------------------------------------------------------------------
| Changelog Subscription Page
|--------------------------------------------------------------------------
|
| The page a subscriber's link opens, to confirm a subscription or end
| one. Nothing changes until the button is pressed.
|
*/

package changelog

import (
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
SubscriptionVars is subscription, on the page a subscriber's link opens.
*/
type SubscriptionVars struct {
	// Confirm or unsubscribe.
	Purpose string
	// The address the link is for.
	Email string
	// The page's heading.
	Title string
	// What pressing the button does.
	Ask string
	// The button's label.
	Button string
	// What the page says once it is done.
	DoneText string
	// Where the button's form posts.
	Action string
	// It is done.
	Done bool
	// The link no longer works.
	Invalid bool
}

/*
SubscriptionPage is where a subscriber's link lands.
*/
func SubscriptionPage(c helpcenter.Center, s Subscription) view.Component {
	m := helpcenter.Meta{Title: subscriptionTitle(c, s), Section: "changelog", NoIndex: true}
	return helpcenter.Render(c, "changelog_subscription", m, map[string]any{"subscription": SubscriptionVars{
		Purpose: s.Purpose, Email: s.Email, Title: subscriptionTitle(c, s), Ask: subscriptionAsk(c, s),
		Button: subscriptionButton(
			c, s,
		), DoneText: subscriptionDone(c, s), Action: s.Action, Done: s.Done, Invalid: s.Invalid,
	}})
}
