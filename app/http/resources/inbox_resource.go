/*
|--------------------------------------------------------------------------
| Inbox Resource
|--------------------------------------------------------------------------
|
| An inbox as the API shows it: its name, whether it is the default
| inbox, its default team, the schedule its office hours are kept in
| and its settings with their defaults filled in. Teams and schedules
| appear by their public ids, never by internal ones.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Inbox is one inbox of the workspace. Default marks the inbox where
conversations land when nothing routes them elsewhere; Access says who
may open it; ScheduleID names the schedule of its own office hours,
null when it follows the workspace's default.
*/
type Inbox struct {
	ID            string        `json:"id"`
	Object        string        `json:"object"`
	Name          string        `json:"name"`
	Default       bool          `json:"default"`
	DefaultTeamID *string       `json:"default_team_id"`
	ScheduleID    *string       `json:"schedule_id"`
	Access        InboxAccess   `json:"access"`
	Settings      InboxSettings `json:"settings"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

/*
InboxAccess is who may open an inbox: everyone, or the teams and
teammates on its list.
*/
type InboxAccess struct {
	Mode    string   `json:"mode"`
	Teams   []string `json:"team_ids"`
	Members []string `json:"member_ids"`
}

/*
InboxSettings are an inbox's settings: how long a reply reopens a
closed conversation, whether a reply takes a conversation nobody holds,
the inbox's color and how it assigns new conversations.
*/
type InboxSettings struct {
	ReopenWindowDays int             `json:"reopen_window_days"`
	AssignOnReply    bool            `json:"assign_on_reply"`
	Color            string          `json:"color"`
	Assignment       InboxAssignment `json:"assignment"`
}

/*
InboxAssignment is how an inbox assigns new conversations: the mode,
whether away members take them, the overflow team, the order waiting
conversations are taken in, and the default capacities.
*/
type InboxAssignment struct {
	Mode              string         `json:"mode"`
	IncludeAway       bool           `json:"include_away"`
	OverflowTeamID    *string        `json:"overflow_team_id"`
	QueueOrder        string         `json:"queue_order"`
	DefaultCapacity   *int           `json:"default_capacity"`
	CapacityByChannel map[string]int `json:"capacity_by_channel"`
}

/*
NewInbox shapes the inbox.
*/
func NewInbox(i inboxes.Inbox) Inbox {
	s := i.ReadSettings()

	return Inbox{
		ID: models.PublicID(models.PrefixInbox, i.ID), Object: "inbox", Name: i.Name, Default: i.Default,
		DefaultTeamID: teamID(i.DefaultTeamID), ScheduleID: scheduleID(i.ScheduleID),
		CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
		Access: InboxAccess{
			Mode: i.Access.Mode, Teams: publicIDs(models.PrefixTeam, i.Access.Teams),
			Members: publicIDs(models.PrefixMember, i.Access.Members),
		},
		Settings: InboxSettings{
			ReopenWindowDays: s.ReopenWindowDays, AssignOnReply: s.AssignOnReply, Color: s.Color,
			Assignment: InboxAssignment{
				Mode: s.Assignment.Mode, IncludeAway: s.Assignment.IncludeAway,
				OverflowTeamID: teamID(s.Assignment.OverflowTeamID), QueueOrder: s.Assignment.QueueOrder,
				DefaultCapacity: s.Assignment.DefaultCapacity, CapacityByChannel: s.Assignment.CapacityByChannel,
			},
		},
	}
}

/*
NewInboxes shapes a list of inboxes, in the order given.
*/
func NewInboxes(rows []inboxes.Inbox) []Inbox {
	out := make([]Inbox, 0, len(rows))
	for _, i := range rows {
		out = append(out, NewInbox(i))
	}

	return out
}

/*
teamID returns a team's public id, nil for no team.
*/
func teamID(id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	return new(models.PublicID(models.PrefixTeam, *id))
}

/*
scheduleID returns a schedule's public id, nil for none.
*/
func scheduleID(id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	return new(models.PublicID(models.PrefixSchedule, *id))
}
