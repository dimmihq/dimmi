/*
|--------------------------------------------------------------------------
| Broadcast Visitor Typing
|--------------------------------------------------------------------------
|
| A teammate sees the visitor typing, the way any chat shows the other side,
| so they wait for the question instead of answering half of it. Nothing is
| stored and nothing is replayed after a reconnect, and a busy keyboard is
| thinned to one notice every two seconds per conversation, so a visitor's
| browser can never flood a teammate's screen.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastVisitorTyping tells teammates a visitor is typing. Throttle is how
often a start passes on.

Events: [appevents.ContactTyping].
*/
type BroadcastVisitorTyping struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Cache    *cache.Store
	Throttle time.Duration
}

/*
Handle passes the visitor's typing on, or quietly drops it inside the
throttle window.
*/
func (a *BroadcastVisitorTyping) Handle(
	ctx context.Context, auth Authenticated, conversation ids.UUID, started bool,
) error {
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).FindWidgetConversation(ctx, scopeOf(auth, conversation))

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrWidgetConversationNotFound
	}

	if err != nil {
		return fmt.Errorf("find widget conversation: %w", err)
	}

	pass, err := a.admit(ctx, auth.Session.ID, conversation, started)
	if err != nil || !pass {
		return err
	}

	return a.Events.Dispatch(ctx, nil, appevents.ContactTyping{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: auth.Session.WorkspaceID, OccurredAt: a.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeContact), ID: auth.Session.ContactID},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: conversation},
		ContactID:       auth.Session.ContactID, Started: started,
	})
}

/*
admit lets one start per window through and opens the window again on a
stop.
*/
func (a *BroadcastVisitorTyping) admit(
	ctx context.Context, session, conversation ids.UUID, started bool,
) (bool, error) {
	key := "widget:typing:" + session.String() + ":" + conversation.String()

	if !started {
		if err := a.Cache.Delete(ctx, key); err != nil {
			return false, fmt.Errorf("reset visitor typing throttle: %w", err)
		}

		return true, nil
	}

	added, err := a.Cache.Add(ctx, key, []byte{1}, a.Throttle)
	if err != nil {
		return false, fmt.Errorf("throttle visitor typing: %w", err)
	}

	return added, nil
}
