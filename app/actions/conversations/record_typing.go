/*
|--------------------------------------------------------------------------
| Record Typing
|--------------------------------------------------------------------------
|
| A teammate typing a reply is shown to the customer, the way any chat
| shows the other side typing, so they wait for the answer instead of
| leaving. Every client reports typing through the API; none publishes
| on a channel itself. Nothing is stored, and teammates are not told:
| who else is looking at the conversation is what keeps two people from
| answering at once.
|
| Clients report every few seconds while someone types, and the reports
| are thinned to one every two seconds per member and conversation, so a
| fast typist cannot flood the customer's screen. A note is internal,
| so typing one is never shown to anybody. Only channels that can show
| typing, such as the messenger widget, ever hear about it.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Typing timings: one report per member and conversation is relayed
every TypingThrottle, and a customer's screen forgets a report after
TypingExpiry without a new one.
*/
const (
	TypingThrottle = 2 * time.Second
	TypingExpiry   = 6 * time.Second
)

/*
The typing states and composers a report names.
*/
const (
	TypingStart   = "start"
	TypingStop    = "stop"
	ComposerReply = "reply"
	ComposerNote  = "note"
)

/*
widgetChannel is the channel type of the messenger widget, the one
channel that shows a teammate typing without a provider in between.
*/
const widgetChannel = "widget"

/*
RecordTypingInput is who types, where and what: State is start or stop,
Composer reply or note.
*/
type RecordTypingInput struct {
	Membership     authz.Membership
	ConversationID ids.UUID
	State          string
	Composer       string
}

/*
TypingIndicator is the report as accepted: its state and when the
customer's screen stops showing it.
*/
type TypingIndicator struct {
	State     string
	ExpiresAt time.Time
}

/*
RecordTyping relays a teammate typing to the customer's side.

Rules:
  - The caller needs conversations:write and must see the conversation.
  - A note is never relayed.
  - Starts are relayed at most once every two seconds per member and
    conversation; a stop is always relayed and resets that window.
  - Only the messenger widget is told, with the teammate's name and
    avatar, never their email or id. Other channels are told nothing.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database, cache or broadcast error.
*/
type RecordTyping struct {
	DB        *database.DB
	Cache     *cache.Store
	Clock     clock.Clock
	Scope     appbroadcasting.ConversationScope
	Publisher appbroadcasting.Publisher
}

/*
Handle accepts the report and relays it where it belongs.
*/
func (a *RecordTyping) Handle(ctx context.Context, in RecordTypingInput) (TypingIndicator, error) {
	if !in.Membership.Can("conversations:write") {
		return TypingIndicator{}, exceptions.ErrForbidden
	}

	indicator := TypingIndicator{State: in.State, ExpiresAt: a.Clock.Now().Add(TypingExpiry)}

	target, err := a.load(ctx, in)
	if err != nil || in.Composer == ComposerNote || target.channel.Type != widgetChannel {
		return indicator, err
	}

	relay, err := a.admit(ctx, in)
	if err != nil || !relay {
		return indicator, err
	}

	return indicator, a.relay(ctx, in, target)
}

/*
typingTarget is what a relay needs: the conversation, its channel and
the teammate typing.
*/
type typingTarget struct {
	conversation models.Conversation
	channel      models.ChannelConnection
	user         models.User
}

/*
load reads the conversation the caller may see, its channel and the
caller's account.
*/
func (a *RecordTyping) load(ctx context.Context, in RecordTypingInput) (typingTarget, error) {
	var t typingTarget

	workspaceID := in.Membership.Workspace.ID

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		q := models.New(tx)

		t.conversation, err = q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: workspaceID, ID: in.ConversationID,
		})
		if err != nil {
			return err
		}

		if !a.Scope.Contains(ctx, in.Membership, t.conversation) {
			return database.ErrNotFound
		}

		t.channel, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: workspaceID, ID: t.conversation.ChannelConnectionID,
		})
		if err != nil {
			return err
		}

		t.user, err = q.FindUser(ctx, in.Membership.Member.UserID)

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return t, exceptions.ErrNotFound
	}

	if err != nil {
		return t, fmt.Errorf("load typing conversation: %w", err)
	}

	return t, nil
}

/*
admit thins starts to one per window and opens the window again on a
stop.
*/
func (a *RecordTyping) admit(ctx context.Context, in RecordTypingInput) (bool, error) {
	key := "typing:" + in.ConversationID.String() + ":" + in.Membership.Member.ID.String()

	if in.State == TypingStop {
		if err := a.Cache.Delete(ctx, key); err != nil {
			return false, fmt.Errorf("reset typing throttle: %w", err)
		}

		return true, nil
	}

	added, err := a.Cache.Add(ctx, key, []byte{1}, TypingThrottle)
	if err != nil {
		return false, fmt.Errorf("throttle typing: %w", err)
	}

	return added, nil
}

/*
typingRelay is what the customer's widget learns about the teammate.
*/
type typingRelay struct {
	ConversationID string  `json:"conversation_id"`
	Name           string  `json:"name"`
	AvatarURL      *string `json:"avatar_url"`
}

/*
relay tells the contact's widget, without keeping it in history.
*/
func (a *RecordTyping) relay(ctx context.Context, in RecordTypingInput, t typingTarget) error {
	event := "typing.started"
	if in.State == TypingStop {
		event = "typing.stopped"
	}

	m := fbroadcasting.Message{
		ID:    ids.Encode("evt", ids.New()),
		Event: event,
		Data: typingRelay{
			ConversationID: models.PublicID(models.PrefixConversation, t.conversation.ID),
			Name:           t.user.Name,
		},
		Ephemeral: true,
	}

	channel := appbroadcasting.WidgetContactChannel(t.conversation.ContactID)
	if err := a.Publisher.Publish(ctx, []string{channel}, m); err != nil {
		return fmt.Errorf("relay typing: %w", err)
	}

	return nil
}
