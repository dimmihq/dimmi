/*
|--------------------------------------------------------------------------
| Viewers
|--------------------------------------------------------------------------
|
| Who is looking at a conversation right now, read from the presence of
| its channel, so a teammate sees from the list that someone already has
| it open. Presence knows users; the inbox shows members, so each user
| is looked up as a member of the conversation's workspace, and someone
| with several tabs open counts once.
|
| Viewers are a courtesy. A list waits for them only briefly: when the
| presence store does not answer in time, the list goes out without them.
| A change in who views is announced at most once a second for each
| conversation, so people flicking through the inbox do not flood every
| screen.
|
*/

package broadcasting

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ViewersDeadline is how long a list waits for its viewers.
*/
const ViewersDeadline = 100 * time.Millisecond

/*
viewersWindow is how often a conversation's viewers are announced at
most.
*/
const viewersWindow = time.Second

/*
PresenceReader reads who listens on channels.
*/
type PresenceReader interface {
	/*
		PresenceMany returns the connections listening on each channel.
	*/
	PresenceMany(ctx context.Context, channels []string) (map[string][]fbroadcasting.Member, error)
}

/*
Viewer is a member viewing a conversation.
*/
type Viewer struct {
	MemberID ids.UUID
	Name     string
}

/*
Viewers finds who views conversations.
*/
type Viewers struct {
	Presence PresenceReader
	DB       *database.DB
	IDs      *ids.Registry
}

/*
Of returns the members viewing each of the workspace's conversations,
ordered by name. It gives up with the context's error at the deadline.
*/
func (v *Viewers) Of(
	ctx context.Context, workspace ids.UUID, conversations []ids.UUID,
) (map[ids.UUID][]Viewer, error) {
	channels := make([]string, len(conversations))
	for i, id := range conversations {
		channels[i] = ConversationChannel(id)
	}

	lists, err := v.Presence.PresenceMany(ctx, channels)
	if err != nil {
		return nil, fmt.Errorf("read viewers: %w", err)
	}

	users := map[ids.UUID]bool{}

	for _, members := range lists {
		for _, m := range members {
			if id, err := v.IDs.Decode(string(models.PrefixUser), m.UserID); err == nil {
				users[id] = true
			}
		}
	}

	people, err := v.members(ctx, workspace, slices.Collect(maps.Keys(users)))
	if err != nil {
		return nil, err
	}

	out := make(map[ids.UUID][]Viewer, len(conversations))
	for i, id := range conversations {
		out[id] = viewersOf(v.IDs, lists[channels[i]], people)
	}

	return out, nil
}

/*
members returns the workspace's active members among the users, by
user.
*/
func (v *Viewers) members(ctx context.Context, workspace ids.UUID, users []ids.UUID) (map[ids.UUID]Viewer, error) {
	out := map[ids.UUID]Viewer{}
	if len(users) == 0 {
		return out, nil
	}

	err := v.DB.Transaction(tenancy.With(ctx, workspace.String()), func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ActiveMembersOfUsers(ctx, models.ActiveMembersOfUsersParams{
			WorkspaceID: workspace, UserIds: users,
		})
		if err != nil {
			return fmt.Errorf("find viewing members: %w", err)
		}

		for _, row := range rows {
			out[row.UserID] = Viewer{MemberID: row.ID, Name: row.Name}
		}

		return nil
	})

	return out, err
}

/*
viewersOf turns a channel's listeners into its viewers, each member
once, ordered by name.
*/
func viewersOf(registry *ids.Registry, listeners []fbroadcasting.Member, people map[ids.UUID]Viewer) []Viewer {
	seen := map[ids.UUID]bool{}
	out := []Viewer{}

	for _, l := range listeners {
		user, err := registry.Decode(string(models.PrefixUser), l.UserID)
		if err != nil {
			continue
		}

		viewer, ok := people[user]
		if !ok || seen[viewer.MemberID] {
			continue
		}

		seen[viewer.MemberID] = true
		out = append(out, viewer)
	}

	slices.SortFunc(out, func(a, b Viewer) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.MemberID.String(), b.MemberID.String()))
	})

	return out
}

/*
ViewerPayload is a viewer as broadcasts and lists show them.
*/
type ViewerPayload struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
NewViewerPayloads shapes the viewers.
*/
func NewViewerPayloads(viewers []Viewer) []ViewerPayload {
	out := make([]ViewerPayload, len(viewers))
	for i, v := range viewers {
		out[i] = ViewerPayload{ID: models.PublicID(models.PrefixMember, v.MemberID), Name: v.Name}
	}

	return out
}

/*
ViewerAnnouncer tells the workspace's lists and the conversation's own
screens who views a conversation each time that changes, at most once a
second per conversation. Locate finds the workspace of a conversation a
connection was allowed to subscribe to.
*/
type ViewerAnnouncer struct {
	Viewers *Viewers
	Server  Publisher
	Clock   clock.Clock
	Locate  func(ctx context.Context, caller *appauth.Principal, conversation ids.UUID) (ids.UUID, bool, error)

	mu      sync.Mutex
	pending map[ids.UUID]bool
}

/*
Changed is the conversation pattern's hook: a connection started or
stopped listening on the channel. The announcement goes out after the
window, carrying whoever views then.
*/
func (a *ViewerAnnouncer) Changed(ctx context.Context, p fbroadcasting.Principal, channel string) {
	caller, ok := p.Subject.(*appauth.Principal)
	if !ok {
		return
	}

	id, err := a.Viewers.IDs.Decode(string(models.PrefixConversation), channel[len("conversation:"):])
	if err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pending == nil {
		a.pending = map[ids.UUID]bool{}
	}

	if a.pending[id] {
		return
	}

	a.pending[id] = true
	ctx = context.WithoutCancel(ctx)

	a.Clock.AfterFunc(viewersWindow, func() { a.announce(ctx, caller, id) })
}

/*
announce publishes who views the conversation now. A failure is logged:
screens catch up when they next load the list.
*/
func (a *ViewerAnnouncer) announce(ctx context.Context, caller *appauth.Principal, id ids.UUID) {
	a.mu.Lock()
	delete(a.pending, id)
	a.mu.Unlock()

	if err := a.publish(ctx, caller, id); err != nil {
		log.From(ctx).WarnContext(ctx, "announce conversation viewers", log.Err(err))
	}
}

func (a *ViewerAnnouncer) publish(ctx context.Context, caller *appauth.Principal, id ids.UUID) error {
	workspace, found, err := a.Locate(ctx, caller, id)
	if err != nil || !found {
		return err
	}

	viewers, err := a.Viewers.Of(ctx, workspace, []ids.UUID{id})
	if err != nil {
		return err
	}

	m := fbroadcasting.Message{
		ID: ids.Encode("evt", ids.New()), Event: "conversation.viewers_changed", Ephemeral: true,
		Data: map[string]any{
			"id": models.PublicID(models.PrefixConversation, id), "viewers": NewViewerPayloads(viewers[id]),
		},
	}

	channels := []string{WorkspaceConversationsChannel(workspace), ConversationChannel(id)}
	if err := a.Server.Publish(ctx, channels, m); err != nil {
		return fmt.Errorf("broadcast conversation viewers: %w", err)
	}

	return nil
}
