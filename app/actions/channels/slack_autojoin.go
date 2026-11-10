/*
|--------------------------------------------------------------------------
| Slack Auto-Join
|--------------------------------------------------------------------------
|
| A team that opens a channel per customer should not have to invite the
| app every time. A connection's auto-join rules match channels by name,
| by prefix, suffix or anything it contains, and say how the room is
| watched once joined: its mode, its trigger and whether the agent
| answers. New and renamed channels are checked as they happen, newly
| shared ones too, and a scan checks every existing channel a page at a
| time, at the pace Slack allows, resuming where it stopped.
|
| A channel the app cannot join by itself, a private one or one another
| organization owns, is listed as a suggestion to invite the app, with
| no error.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Pacing of a scan: channels checked per page, and the wait between
pages, within the join method's limit.
*/
const (
	scanPage      = 40
	scanPageEvery = time.Minute
	maxSuggested  = 200
)

/*
JoinRule is one auto-join rule: how a channel's name matches, against
what, and how the joined room is watched. AgentEnabled nil leaves the
agent on.
*/
type JoinRule struct {
	Match        string `json:"match"`
	Value        string `json:"value"`
	Mode         string `json:"mode"`
	Trigger      string `json:"trigger"`
	AgentEnabled *bool  `json:"agent_enabled"`
}

/*
Suggestion is a channel a rule matched that the app could not join.
*/
type Suggestion struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
matches reports whether the channel's name matches the rule.
*/
func (r JoinRule) matches(name string) bool {
	name, value := strings.ToLower(strings.TrimPrefix(name, "#")), strings.ToLower(r.Value)
	if value == "" {
		return false
	}

	switch r.Match {
	case "prefix":
		return strings.HasPrefix(name, value)
	case "suffix":
		return strings.HasSuffix(name, value)
	case "contains":
		return strings.Contains(name, value)
	default:
		return false
	}
}

/*
ruleFor returns the first of the connection's rules the name matches.
*/
func ruleFor(conn models.ChannelConnection, name string) (JoinRule, bool) {
	var rules []JoinRule
	connectionSetting(conn, settingAutoJoin, &rules)

	for _, rule := range rules {
		if rule.matches(name) {
			return rule, true
		}
	}

	return JoinRule{}, false
}

/*
onChannelSeen checks a channel created, renamed or shared against the
rules. It never claims the event, so the channel's own handling of a
rename or a share still runs.
*/
func (s *Slack) onChannelSeen(ctx context.Context, ev SlackEvent) (bool, error) {
	channel := slackdriver.Channel{ID: ev.Event.ChannelID(), Name: ev.Event.ChannelName()}
	if channel.ID == "" {
		return false, nil
	}

	if channel.Name == "" {
		api, err := s.api(ctx, ev.Connection)
		if err != nil {
			return false, err
		}

		if channel, err = api.Channel(ctx, channel.ID); err != nil {
			return false, ignoreUnreadable(err)
		}
	}

	joined, err := s.autoJoin(ctx, ev.Connection, channel)

	return joined && ev.Event.Type == "channel_created", err
}

/*
autoJoin joins the channel when a rule matches it, or suggests inviting
the app when it cannot join, reporting whether it joined.
*/
func (s *Slack) autoJoin(ctx context.Context, conn models.ChannelConnection, channel slackdriver.Channel) (
	bool, error,
) {
	rule, ok := ruleFor(conn, channel.Name)
	if !ok {
		return false, nil
	}

	if _, watched, err := s.watchedRoom(ctx, conn, channel.ID); err != nil || watched {
		return false, err
	}

	if channel.IsPrivate {
		return false, s.suggest(ctx, conn, channel)
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return false, err
	}

	joined, err := api.Join(ctx, channel.ID)
	if err != nil {
		return false, s.suggest(ctx, conn, channel)
	}

	return true, s.read(ctx, func(q *models.Queries) error {
		if err := s.upsertRoom(ctx, q, conn, joined); err != nil {
			return err
		}

		return s.applyRule(ctx, q, joinedRoom{conn: conn, channel: joined.ID, rule: rule})
	})
}

/*
joinedRoom is a channel the app joined by a rule, on its connection.
*/
type joinedRoom struct {
	conn    models.ChannelConnection
	channel string
	rule    JoinRule
}

/*
applyRule watches the joined room the way the rule says.
*/
func (s *Slack) applyRule(ctx context.Context, q *models.Queries, j joinedRoom) error {
	conn, rule, now := j.conn, j.rule, s.Clock.Now()

	room, err := q.FindChannelRoomByExternalID(ctx, models.FindChannelRoomByExternalIDParams{
		WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalID: j.channel,
	})
	if err != nil {
		return fmt.Errorf("find joined room: %w", err)
	}

	mode := ruleValue(rule.Mode, room.Mode, []string{"shared", "community", "internal", "helpdesk"})
	trigger := ruleValue(rule.Trigger, room.Trigger, []string{
		"author_window", "every_message", "emoji", "mention", "ai", "off",
	})
	if mode == string(enums.RoomModeInternal) {
		trigger = string(enums.RoomTriggerOff)
	}

	agent := room.AgentEnabled
	if rule.AgentEnabled != nil {
		agent = *rule.AgentEnabled
	}

	_, err = q.UpdateChannelRoom(ctx, models.UpdateChannelRoomParams{
		Status: room.Status, Mode: mode, Trigger: trigger, AgentEnabled: agent, Settings: room.Settings,
		CompanyID: room.CompanyID, Now: now, WorkspaceID: room.WorkspaceID, ID: room.ID,
	})
	if err != nil {
		return fmt.Errorf("watch joined room: %w", err)
	}

	return nil
}

/*
ruleValue is the rule's value when it is one of the allowed, else the
room's own.
*/
func ruleValue(value, current string, allowed []string) string {
	if slices.Contains(allowed, value) {
		return value
	}

	return current
}

/*
suggest lists the channel among those to invite the app to, once.
*/
func (s *Slack) suggest(ctx context.Context, conn models.ChannelConnection, channel slackdriver.Channel) error {
	fresh, err := s.connection(ctx, conn.WorkspaceID, conn.ID)
	if err != nil {
		return err
	}

	var list []Suggestion
	connectionSetting(fresh, "auto_join_suggestions", &list)

	known := slices.ContainsFunc(list, func(sg Suggestion) bool { return sg.ID == channel.ID })
	if known || len(list) >= maxSuggested {
		return nil
	}

	list = append(list, Suggestion{ID: channel.ID, Name: "#" + channel.Name})

	return s.patchConfig(ctx, conn, map[string]any{"auto_join_suggestions": list})
}

/*
patchConfig merges the keys into the connection's config.
*/
func (s *Slack) patchConfig(ctx context.Context, conn models.ChannelConnection, patch map[string]any) error {
	raw, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("encode connection config: %w", err)
	}

	return s.read(ctx, func(q *models.Queries) error {
		return q.PatchChannelConnectionConfig(ctx, models.PatchChannelConnectionConfigParams{
			Patch: raw, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
	})
}

/*
ScanChannels checks every existing channel of the connection against
its auto-join rules, in the background.
*/
func (s *Slack) ScanChannels(ctx context.Context, workspaceID, connectionID ids.UUID) error {
	return s.Queue.Push(tenancy.With(ctx, workspaceID.String()), jobs.JoinSlackChannelsArgs{
		WorkspaceID: workspaceID, ConnectionID: connectionID,
	})
}

/*
JoinChannels checks one page of the connection's channels and queues
the next a minute later, recording how far the scan got.
*/
func (s *Slack) JoinChannels(ctx context.Context, args jobs.JoinSlackChannelsArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	conn, err := s.connection(ctx, args.WorkspaceID, args.ConnectionID)
	var rules []JoinRule
	if err != nil || conn.DeletedAt != nil || conn.Status == "disconnected" ||
		!connectionSetting(conn, settingAutoJoin, &rules) || len(rules) == 0 {
		return ignoreMissing(err)
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	page, next, err := api.ChannelPage(ctx, args.Cursor, scanPage)
	if err != nil {
		return err
	}

	for _, channel := range page {
		if _, err := s.autoJoin(ctx, conn, channel); err != nil {
			return err
		}
	}

	if err := s.patchConfig(ctx, conn, map[string]any{
		"auto_join_scan": map[string]any{"cursor": next, "done": next == "", "at": s.Clock.Now()},
	}); err != nil || next == "" {
		return err
	}

	args.Cursor = next

	return s.Queue.Push(ctx, args, queue.At(s.Clock.Now().Add(scanPageEvery)))
}
