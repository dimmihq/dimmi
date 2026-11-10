/*
|--------------------------------------------------------------------------
| Broadcasts
|--------------------------------------------------------------------------
|
| A broadcast is written as a draft and changes freely until it starts
| sending; after that only its status moves. Reading takes
| broadcasts:read, writing a draft broadcasts:write, and anything that
| reaches customers broadcasts:send, so a teammate can prepare a
| broadcast that an admin sends.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
Channels a broadcast can go out on.
*/
const (
	ChannelEmail = "email"
	ChannelChat  = "chat"
	ChannelPost  = "post"
	ChannelSlack = "slack"
)

/*
Statuses a broadcast moves through.
*/
const (
	StatusDraft     = "draft"
	StatusScheduled = "scheduled"
	StatusSending   = "sending"
	StatusPaused    = "paused"
	StatusSent      = "sent"
	StatusStopped   = "stopped"
)

/*
Errors a broadcast's API returns.
*/
var (
	ErrNotDraft = fault.New(fault.Conflict, "broadcast_not_draft",
		"This broadcast has started sending, so it can no longer be changed.")
	ErrChannel = fault.New(fault.Invalid, "broadcast_channel_invalid",
		"Choose email, chat, post or Slack.")
	ErrName = fault.New(fault.Invalid, "broadcast_name_invalid",
		"Give the broadcast a name of up to 120 characters.")
)

/*
Broadcasts holds every broadcast action. DB stores them, Clock dates
them, Queue runs their jobs, Events tells open screens, Audiences
resolves who matches, Config sets pace and limits, Senders reaches each
channel, and Links signs the public links in what is sent.
*/
type Broadcasts struct {
	DB        *database.DB
	Clock     clock.Clock
	Queue     *queue.Queue
	Events    *events.Dispatcher
	Audiences *audiences.Audiences
	Config    config.BroadcastsConfig
	Senders   map[string]Sender
	Links     Links
}

/*
Draft is everything the composer can change. Room ids pick Slack
channels by hand; Rules apply when no saved audience is named.
*/
type Draft struct {
	Name             string
	AudienceID       *ids.UUID
	Rules            json.RawMessage
	RoomIDs          []ids.UUID
	TopicID          *ids.UUID
	Service          bool
	Subject          string
	Preview          string
	Body             string
	SenderMemberID   *ids.UUID
	EmailAddressID   *ids.UUID
	SlackAs          string
	ReplyTeamID      *ids.UUID
	ReplyMemberID    *ids.UUID
	AIReplies        bool
	TrackClicks      bool
	SkipFrequencyCap bool
	ScheduleKind     string
	ScheduledAt      *time.Time
	Goal             json.RawMessage
}

/*
ListInput narrows the list of broadcasts.
*/
type ListInput struct {
	Membership authz.Membership
	Status     *string
	Channel    *string
	Search     *string
	Limit      int
	Offset     int
}

/*
List returns a page of the workspace's broadcasts, newest first.
*/
func (b *Broadcasts) List(ctx context.Context, in ListInput) ([]models.Broadcast, error) {
	if !in.Membership.Can("broadcasts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Broadcast

	err := b.inWorkspace(ctx, in.Membership.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.ListBroadcastPage(ctx, models.ListBroadcastPageParams{
			WorkspaceID: in.Membership.Workspace.ID, Status: in.Status, Channel: in.Channel, Search: in.Search,
			RowLimit: int64(max(min(in.Limit, 100), 1)), RowOffset: int64(max(in.Offset, 0)),
		})

		return wrap("list broadcasts", err)
	})

	return out, err
}

/*
Show returns one broadcast.
*/
func (b *Broadcasts) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	if !m.Can("broadcasts:read") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	var out models.Broadcast

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = find(ctx, q, m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Create starts a draft on a channel, sent by the teammate who wrote it.
*/
func (b *Broadcasts) Create(ctx context.Context, m authz.Membership, name, channel string) (models.Broadcast, error) {
	if !m.Can("broadcasts:write") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 120 {
		return models.Broadcast{}, ErrName
	}

	if !validChannel(channel) {
		return models.Broadcast{}, ErrChannel
	}

	var out models.Broadcast

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.CreateBroadcast(ctx, models.CreateBroadcastParams{
			WorkspaceID: m.Workspace.ID, Name: name, Channel: channel, MemberID: &m.Member.ID, Now: b.Clock.Now(),
		})

		return wrap("create broadcast", err)
	})

	return out, err
}

/*
Delete removes a draft.
*/
func (b *Broadcasts) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("broadcasts:write") {
		return exceptions.ErrForbidden
	}

	return b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if row.Status != StatusDraft {
			return ErrNotDraft
		}

		_, err = q.DeleteBroadcast(ctx, models.DeleteBroadcastParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: b.Clock.Now(),
		})

		return wrap("delete broadcast", err)
	})
}

/*
Duplicate copies a broadcast's content and settings into a new draft.
*/
func (b *Broadcasts) Duplicate(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	if !m.Can("broadcasts:write") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	var out models.Broadcast

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		name := []rune("Copy of " + row.Name)
		out, err = q.CreateBroadcast(ctx, models.CreateBroadcastParams{
			WorkspaceID: m.Workspace.ID, Name: string(name[:min(len(name), 120)]), Channel: row.Channel,
			MemberID: &m.Member.ID, Now: b.Clock.Now(),
		})
		if err != nil {
			return wrap("duplicate broadcast", err)
		}

		params := saveParams(out, DraftOf(row), b.Clock.Now())
		params.ScheduleKind, params.ScheduledAt = "now", nil
		out, err = q.SaveBroadcastDraft(ctx, params)

		return wrap("copy broadcast", err)
	})

	return out, err
}

/*
inWorkspace runs fn in a transaction scoped to the workspace.
*/
func (b *Broadcasts) inWorkspace(
	ctx context.Context, ws ids.UUID, fn func(context.Context, *models.Queries) error,
) error {
	return b.DB.Transaction(withTenant(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
find returns one broadcast, or not found.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Broadcast, error) {
	row, err := q.FindBroadcast(ctx, models.FindBroadcastParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	return row, wrap("find broadcast", err)
}

/*
validChannel reports whether a broadcast can go out on the channel.
*/
func validChannel(channel string) bool {
	switch channel {
	case ChannelEmail, ChannelChat, ChannelPost, ChannelSlack:
		return true
	}

	return false
}

/*
wrap names what was being attempted when err is not nil.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
