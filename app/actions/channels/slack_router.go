/*
|--------------------------------------------------------------------------
| Slack Router
|--------------------------------------------------------------------------
|
| One Slack install serves several features, and all of Slack's
| deliveries arrive at one place. The router hands each one to the
| feature that claims it. Events are claimed by kind: a message by the
| kind of place it was written in, such as message.im for a direct
| message, before plain message; anything else by its event type. Button
| presses are claimed by the prefix of their action id, the longest
| prefix winning, and shortcuts and form submissions by callback id.
|
| Handlers of one kind are asked in the order they registered. The first
| that says it handled the delivery ends the search, so a feature can
| take what is meant for it and leave the rest to the next one.
|
*/

package channels

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/framework/ids"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackEvent is one event on a connection, inside its envelope, with the
workspace and the installation it came through, nil for a workspace's
own app. An installation made without channels has no connection of
its own: Connection then has a zero id and only carries the
installation's workspace, team, app and bot, so its token still opens,
and Connected reports false.
*/
type SlackEvent struct {
	Connection     models.ChannelConnection
	WorkspaceID    ids.UUID
	InstallationID *ids.UUID
	Envelope       slackdriver.Envelope
	Event          slackdriver.Event
}

/*
Connected reports whether the event came on a channel connection.
*/
func (ev SlackEvent) Connected() bool {
	return ev.Connection.ID != ids.Nil()
}

/*
SlackInteraction is one interaction on a connection, with the workspace
and installation as on an event, Connection likewise without an id for
an installation made without channels. Action is the button or option
it was claimed by, for a block action.
*/
type SlackInteraction struct {
	Connection     models.ChannelConnection
	WorkspaceID    ids.UUID
	InstallationID *ids.UUID
	Interaction    slackdriver.Interaction
	Action         slackdriver.BlockAction
}

/*
Connected reports whether the interaction came on a channel connection.
*/
func (in SlackInteraction) Connected() bool {
	return in.Connection.ID != ids.Nil()
}

/*
EventHandler handles an event, reporting whether it did.
*/
type EventHandler func(ctx context.Context, ev SlackEvent) (bool, error)

/*
InteractionHandler handles an interaction, reporting whether it did.
*/
type InteractionHandler func(ctx context.Context, in SlackInteraction) (bool, error)

/*
prefixed is an action handler and the action id prefix it claims.
*/
type prefixed struct {
	prefix  string
	handler InteractionHandler
}

/*
Router hands Slack's deliveries to the features that claim them.
*/
type Router struct {
	mu        sync.RWMutex
	events    map[string][]EventHandler
	actions   []prefixed
	shortcuts map[string][]InteractionHandler
	views     map[string][]InteractionHandler
}

/*
NewRouter returns a router nothing has claimed anything on yet.
*/
func NewRouter() *Router {
	return &Router{
		events: map[string][]EventHandler{}, shortcuts: map[string][]InteractionHandler{},
		views: map[string][]InteractionHandler{},
	}
}

/*
OnEvent claims events of the kind: an event type such as app_mention or
reaction_added, or a message kind such as message.im, message.channels,
message.groups or message.mpim, or message for every message.
*/
func (r *Router) OnEvent(kind string, h EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[kind] = append(r.events[kind], h)
}

/*
OnAction claims block actions whose action id starts with the prefix.
*/
func (r *Router) OnAction(prefix string, h InteractionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actions = append(r.actions, prefixed{prefix: prefix, handler: h})
	slices.SortStableFunc(r.actions, func(a, b prefixed) int { return len(b.prefix) - len(a.prefix) })
}

/*
OnShortcut claims message and global shortcuts with the callback id.
*/
func (r *Router) OnShortcut(callbackID string, h InteractionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.shortcuts[callbackID] = append(r.shortcuts[callbackID], h)
}

/*
OnView claims form submissions with the callback id.
*/
func (r *Router) OnView(callbackID string, h InteractionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.views[callbackID] = append(r.views[callbackID], h)
}

/*
messageKinds names the kind of place each channel type of a message is.
*/
func messageKinds() map[string]string {
	return map[string]string{
		"im": "message.im", "channel": "message.channels", "group": "message.groups",
		"mpim": "message.mpim",
	}
}

/*
DispatchEvent hands the event to the first handler that takes it,
reporting whether one did.
*/
func (r *Router) DispatchEvent(ctx context.Context, ev SlackEvent) (bool, error) {
	kinds := []string{ev.Event.Type}
	if ev.Event.Type == "message" {
		if specific, ok := messageKinds()[ev.Event.ChannelType]; ok {
			kinds = []string{specific, "message"}
		}
	}

	r.mu.RLock()

	var handlers []EventHandler
	for _, kind := range kinds {
		handlers = append(handlers, r.events[kind]...)
	}

	r.mu.RUnlock()

	for _, h := range handlers {
		handled, err := h(ctx, ev)
		if err != nil || handled {
			return handled, err
		}
	}

	return false, nil
}

/*
DispatchInteraction hands the interaction to the first handler that
takes it: each block action by its action id, a shortcut or a form by
its callback id.
*/
func (r *Router) DispatchInteraction(ctx context.Context, in SlackInteraction) (bool, error) {
	switch in.Interaction.Type {
	case slackdriver.InteractionBlockActions:
		return r.dispatchActions(ctx, in)
	case slackdriver.InteractionMessageAction, slackdriver.InteractionShortcut:
		return r.dispatchTo(ctx, r.handlersOf(r.shortcuts, in.Interaction.CallbackID), in)
	case slackdriver.InteractionViewSubmit:
		return r.dispatchTo(ctx, r.handlersOf(r.views, in.Interaction.CallbackID), in)
	default:
		return false, nil
	}
}

/*
dispatchActions hands each pressed action to the handler of its prefix.
*/
func (r *Router) dispatchActions(ctx context.Context, in SlackInteraction) (bool, error) {
	r.mu.RLock()
	actions := slices.Clone(r.actions)
	r.mu.RUnlock()

	handledAny := false

	for _, action := range in.Interaction.Actions {
		in.Action = action

		for _, p := range actions {
			if !strings.HasPrefix(action.ActionID, p.prefix) {
				continue
			}

			handled, err := p.handler(ctx, in)
			if err != nil {
				return handledAny, err
			}

			if handled {
				handledAny = true

				break
			}
		}
	}

	return handledAny, nil
}

/*
handlersOf returns the handlers registered under the callback id.
*/
func (r *Router) handlersOf(from map[string][]InteractionHandler, callbackID string) []InteractionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Clone(from[callbackID])
}

/*
dispatchTo hands the interaction to the first of the handlers that
takes it.
*/
func (r *Router) dispatchTo(ctx context.Context, handlers []InteractionHandler, in SlackInteraction) (bool, error) {
	for _, h := range handlers {
		handled, err := h(ctx, in)
		if err != nil || handled {
			return handled, err
		}
	}

	return false, nil
}
