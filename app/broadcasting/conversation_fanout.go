/*
|--------------------------------------------------------------------------
| Conversation Fan-out
|--------------------------------------------------------------------------
|
| This is the one place that decides who hears a change to a conversation
| list. Members who see everything share a channel of their own, and a
| restricted member is never let onto it: each of them gets a copy on
| their own channel, and only of conversations they may see. Nothing else
| publishes list events.
|
| A member who could see a conversation before a change and cannot now is
| told to drop it and loses their subscription to it, so its timeline
| stops reaching them at that moment. Most workspaces have no restricted
| members at all, so the usual cost here is one cache read.
|
| The list of restricted members only saves work: it expires on its own,
| is thrown away when access changes, and is read from the database when
| the cache is unavailable, so a lost cache never loses a copy.
|
*/

package broadcasting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EventConversationHidden tells a member a conversation is gone from what
they may see.
*/
const EventConversationHidden = "conversation.hidden"

/*
restrictedTTL is how long the list of restricted members is kept. A
team purged later, which empties lists without announcing it, shows
within this time.
*/
const restrictedTTL = time.Minute

/*
Unsubscriber takes a user off a channel, on every node.
*/
type Unsubscriber interface {
	/*
		Unsubscribe ends the user's subscriptions to the channel.
	*/
	Unsubscribe(ctx context.Context, userID, channel string) error
}

/*
RestrictedMember is a member who does not see every conversation, with
what they do see.
*/
type RestrictedMember struct {
	MemberID      ids.UUID   `json:"member_id"`
	UserID        ids.UUID   `json:"user_id"`
	Mode          string     `json:"mode"`
	TeamIDs       []ids.UUID `json:"team_ids"`
	Unassigned    bool       `json:"unassigned"`
	HiddenInboxes []ids.UUID `json:"hidden_inboxes"`
}

/*
scope returns what the member sees.
*/
func (r RestrictedMember) scope() authz.ConversationScope {
	return authz.ConversationScope{
		Mode: enums.ConversationAccess(r.Mode), MemberID: &r.MemberID, TeamIDs: r.TeamIDs, Unassigned: r.Unassigned,
		HiddenInboxes: r.HiddenInboxes,
	}
}

/*
Sees reports whether the member may see a conversation held as given.
*/
func (r RestrictedMember) Sees(h Holding) bool {
	return r.scope().Sees(h.Inbox, h.Member, h.Team)
}

/*
Holding is where a conversation sits and who holds it: a member, a
team, both or nobody.
*/
type Holding struct {
	Inbox  ids.UUID
	Member *ids.UUID
	Team   *ids.UUID
}

/*
Audience is what a list change means for the restricted members: those
who get a copy, and those who just lost sight of the conversation.
*/
type Audience struct {
	Copies []RestrictedMember
	Hidden []RestrictedMember
}

/*
AudienceOf splits the restricted members by whether they see the
conversation as it is held now and as it was held before.
*/
func AudienceOf(members []RestrictedMember, before, after Holding) Audience {
	var out Audience

	for _, m := range members {
		scope := m.scope()

		switch {
		case scope.Sees(after.Inbox, after.Member, after.Team):
			out.Copies = append(out.Copies, m)
		case scope.Sees(before.Inbox, before.Member, before.Team):
			out.Hidden = append(out.Hidden, m)
		}
	}

	return out
}

/*
Fanout finds the restricted members of a workspace and who holds a
conversation.
*/
type Fanout struct {
	DB    *database.DB
	Cache *cache.Store
}

/*
restrictedKey names a workspace's cached list.
*/
func restrictedKey(workspaceID ids.UUID) string {
	return "restricted_members:" + workspaceID.String()
}

/*
Restricted returns the workspace's restricted members, from the cache
when it can. A cache that fails only costs a read of the database.
*/
func (f *Fanout) Restricted(ctx context.Context, workspaceID ids.UUID) ([]RestrictedMember, error) {
	key := restrictedKey(workspaceID)

	cached, err := f.Cache.Get(ctx, key)
	if err == nil {
		var members []RestrictedMember
		if json.Unmarshal(cached, &members) == nil {
			return members, nil
		}
	}

	members, err := f.load(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(members)
	if err != nil {
		return nil, fmt.Errorf("encode restricted members: %w", err)
	}

	if err := f.Cache.Set(ctx, key, encoded, restrictedTTL); err != nil {
		log.From(ctx).WarnContext(ctx, "cache restricted members", log.Err(err))
	}

	return members, nil
}

/*
load reads the workspace's restricted members from the database.
*/
func (f *Fanout) load(ctx context.Context, workspaceID ids.UUID) ([]RestrictedMember, error) {
	out := []RestrictedMember{}

	err := f.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListRestrictedMembers(ctx, workspaceID)
		if err != nil || len(rows) == 0 {
			return err
		}

		memberIDs := make([]ids.UUID, len(rows))
		for i, row := range rows {
			memberIDs[i] = row.ID
		}

		teams, err := q.ListAccessTeamIDsOfMembers(ctx, models.ListAccessTeamIDsOfMembersParams{
			WorkspaceID: workspaceID, MemberIds: memberIDs,
		})
		if err != nil {
			return err
		}

		hidden, err := q.ListHiddenInboxIDsOfMembers(ctx, models.ListHiddenInboxIDsOfMembersParams{
			WorkspaceID: workspaceID, MemberIds: memberIDs,
		})
		if err != nil {
			return err
		}

		every, err := openingEvery(ctx, q, rows)
		if err != nil {
			return err
		}

		out = restrictedOf(rows, teams, hidden, every)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load restricted members: %w", err)
	}

	return out, nil
}

/*
forget throws the workspace's cached list away, so the next change reads
it again. Should that fail, the list still expires within a minute.
*/
func (f *Fanout) forget(ctx context.Context, workspaceID ids.UUID) error {
	if err := f.Cache.Delete(ctx, restrictedKey(workspaceID)); err != nil {
		log.From(ctx).WarnContext(ctx, "forget restricted members", log.Err(err))
	}

	return nil
}

/*
holding reads who holds the conversation now, and reports false when it
no longer exists.
*/
func (f *Fanout) holding(ctx context.Context, workspaceID, id ids.UUID) (Holding, bool, error) {
	var out Holding

	err := f.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		c, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: id})
		out = Holding{Inbox: c.InboxID, Member: c.AssigneeMemberID, Team: c.AssigneeTeamID}

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, false, nil
	}

	if err != nil {
		return out, false, fmt.Errorf("find fanned out conversation: %w", err)
	}

	return out, true, nil
}

/*
listenFanout keeps the cached list honest: anything that changes who is
restricted throws it away, once it commits.
*/
func (b *Broadcaster) listenFanout(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberAccessChanged) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberDeactivated) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberReactivated) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	b.listenInboxAccess(d)
}

/*
listenInboxAccess throws the cached list away whenever who may open an
inbox can have changed: the inbox's list, a team on it, or a role that
opens every inbox.
*/
func (b *Broadcaster) listenInboxAccess(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.InboxAccessChanged) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.InboxDeleted) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.TeamMemberAdded) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.TeamMemberRemoved) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberRoleChanged) error {
		return b.Fanout.forget(ctx, e.WorkspaceID)
	})
}

/*
publishList publishes a change to a conversation list: the row for
those who see everything, and for restricted members whatever the change
means to each.
*/
func (b *Broadcaster) publishList(ctx context.Context, e appevents.ConversationEvent, row any) error {
	h := e.Header()
	r := reference(models.PrefixConversation, "conversation", e.Conversation(), row)

	err := b.publishOrRefer(ctx, []string{WorkspaceConversationsChannel(h.WorkspaceID)}, message(e, row), r)
	if err != nil {
		return err
	}

	restricted, err := b.Fanout.Restricted(ctx, h.WorkspaceID)
	if err != nil || len(restricted) == 0 {
		return err
	}

	after, found, err := b.Fanout.holding(ctx, h.WorkspaceID, e.Conversation())
	if err != nil || !found {
		return err
	}

	audience := AudienceOf(restricted, beforeChange(e, after), after)

	if len(audience.Copies) > 0 {
		channels := make([]string, len(audience.Copies))
		for i, m := range audience.Copies {
			channels[i] = MemberChannel(m.MemberID)
		}

		if err := b.publishOrRefer(ctx, channels, message(e, row), r); err != nil {
			return err
		}
	}

	return b.hide(ctx, e.Conversation(), audience.Hidden)
}

/*
beforeChange returns where the conversation sat and who held it before
the event: an assignment says who, a move says which inbox, and nothing
else changes either.
*/
func beforeChange(e appevents.ConversationEvent, after Holding) Holding {
	switch e := e.(type) {
	case appevents.ConversationAssigned:
		return Holding{Inbox: after.Inbox, Member: e.FromMemberID, Team: e.FromTeamID}
	case appevents.ConversationUpdated:
		if e.FromInboxID != nil {
			return Holding{Inbox: *e.FromInboxID, Member: after.Member, Team: after.Team}
		}
	}

	return after
}

/*
hide tells each member the conversation is gone and takes them off its
channel. The notice is not kept: a screen that was away refetches.
*/
func (b *Broadcaster) hide(ctx context.Context, conversation ids.UUID, members []RestrictedMember) error {
	public := models.PublicID(models.PrefixConversation, conversation)

	for _, m := range members {
		notice := fbroadcasting.Message{
			ID: ids.Encode("evt", ids.New()), Event: EventConversationHidden,
			Data: map[string]string{"id": public}, Ephemeral: true,
		}
		if err := b.publish(ctx, []string{MemberChannel(m.MemberID)}, notice); err != nil {
			return err
		}

		user := models.PublicID(models.PrefixUser, m.UserID)
		if err := b.Unsubscribe.Unsubscribe(ctx, user, ConversationChannel(conversation)); err != nil {
			return fmt.Errorf("unsubscribe from hidden conversation: %w", err)
		}
	}

	return nil
}

/*
openingEvery returns the members who open every inbox, as those who may
manage inboxes do.
*/
func openingEvery(ctx context.Context, q *models.Queries, rows []models.WorkspaceMember) (map[ids.UUID]bool, error) {
	out := map[ids.UUID]bool{}

	for _, row := range rows {
		every, err := authz.OpensEveryInbox(ctx, q, row)
		if err != nil {
			return nil, err
		}

		out[row.ID] = every
	}

	return out, nil
}

/*
restrictedOf puts each member together with the teams their access names
and the inboxes they may not open, leaving out the inboxes of those who
open every one, and anyone who in the end sees everything.
*/
func restrictedOf(
	rows []models.WorkspaceMember, teams []models.ListAccessTeamIDsOfMembersRow,
	hidden []models.ListHiddenInboxIDsOfMembersRow, every map[ids.UUID]bool,
) []RestrictedMember {
	teamsOf := map[ids.UUID][]ids.UUID{}
	for _, row := range teams {
		teamsOf[row.MemberID] = append(teamsOf[row.MemberID], row.TeamID)
	}

	hiddenOf := map[ids.UUID][]ids.UUID{}
	for _, row := range hidden {
		if !every[row.MemberID] {
			hiddenOf[row.MemberID] = append(hiddenOf[row.MemberID], row.InboxID)
		}
	}

	out := []RestrictedMember{}

	for _, row := range rows {
		if row.ConversationAccess == string(enums.ConversationAccessAll) && len(hiddenOf[row.ID]) == 0 {
			continue
		}

		out = append(out, RestrictedMember{
			MemberID: row.ID, UserID: row.UserID, Mode: row.ConversationAccess, TeamIDs: teamsOf[row.ID],
			Unassigned: row.ConversationAccessUnassigned, HiddenInboxes: hiddenOf[row.ID],
		})
	}

	return out
}
