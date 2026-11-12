/*
|--------------------------------------------------------------------------
| Conversation Rating Routes
|--------------------------------------------------------------------------
|
| Customers rate closed conversations. The team reads the ratings beside
| each conversation and chooses, in settings, whether and how to ask.
| The customer answers in the messenger, as the visitor of the
| conversation, or on the page an email links to, with the signed link
| as the only proof.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
)

/*
conversationRatingRoutes declares the team's routes on ratings, behind
the same guards as the inbox.
*/
func conversationRatingRoutes(r *routing.Router, a *app.App) {
	if a.Actions.ConversationRatings == nil {
		return
	}

	r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

	c := &v1.ConversationRatingController{Ratings: a.Actions.ConversationRatings, IDs: a.IDs}

	routing.Get(r, "/conversation-rating-settings", "conversation-rating-settings.show", c.Settings).
		Ability("settings:read")
	routing.Patch(r, "/conversation-rating-settings", "conversation-rating-settings.update", c.UpdateSettings).
		Ability("settings:write")
	routing.Get(r, "/conversations/{conversation}/ratings", "conversations.ratings.index", c.Index).
		Ability("conversations:read")
}

/*
conversationRatingPageRoutes declares the rating page's API, reached
without signing in.
*/
func conversationRatingPageRoutes(guest *routing.Router, a *app.App) {
	if a.Actions.ConversationRatings == nil {
		return
	}

	pc := &v1.ConversationRatingPageController{Ratings: a.Actions.ConversationRatings}

	routing.Get(guest, "/conversation-ratings/{token}", "conversation-ratings.show", pc.Show)
	routing.Patch(guest, "/conversation-ratings/{token}", "conversation-ratings.update", pc.Update)
}

/*
widgetRatingRoutes declares the rating card of a visitor's
conversation.
*/
func widgetRatingRoutes(visitor *routing.Router, a *app.App, one string) {
	if a.Actions.ConversationRatings == nil {
		return
	}

	c := &widget.RatingController{
		Read: a.Actions.Widget.Conversations, Ratings: a.Actions.ConversationRatings, IDs: a.IDs,
	}

	routing.Get(visitor, one+"/rating", "widget.conversations.rating.show", c.Show)
	routing.Patch(visitor, one+"/rating", "widget.conversations.rating.update", c.Update)
}
