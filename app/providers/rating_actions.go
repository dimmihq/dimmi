/*
|--------------------------------------------------------------------------
| Conversation Rating Wiring
|--------------------------------------------------------------------------
|
| Asking customers to rate closed conversations: every close becomes a
| job that may ask, a second job sends a delayed ask, a third emails
| the ask through the customer stream from the conversation's address,
| and the links in email asks are signed with a key of their own.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/actions/ratings"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
ratingLinksPurpose names the key rating links are signed with.
*/
const ratingLinksPurpose = "conversation-rating-links"

/*
bootRatings builds the rating actions, hears closes and registers the
jobs that ask.
*/
func bootRatings(a *app.App) error {
	r := &ratings.Ratings{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue,
		Links: ratings.Links{BaseURL: a.Application.Config.App.URL, Key: a.DerivedKey(ratingLinksPurpose)},
		Email: &email.SendRatingEmail{
			DB: a.DB, Clock: a.Clock, Mail: a.Mail, Config: a.Config.Channels.Email, Senders: a.EmailDomains,
			Stamps: inbound.NewStamps(a.DerivedKey(stampsKeyPurpose)), Transport: customerTransport(a),
		},
	}
	r.Listen(a.Events)
	a.Actions.ConversationRatings = r

	return errors.Join(
		queue.Register(a.Queue, &jobs.AskConversationRating{Asker: r}),
		queue.Register(a.Queue, &jobs.SendConversationRating{Asker: r}),
		queue.Register(a.Queue, &jobs.MailConversationRating{Asker: r}),
	)
}
