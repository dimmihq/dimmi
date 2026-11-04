/*
|--------------------------------------------------------------------------
| Team Availability
|--------------------------------------------------------------------------
|
| What a visitor is told about the team: whether someone is there, how fast
| they usually answer and a few faces. Someone counts as there only while
| they chose to be available and have a live connection, read from realtime
| presence, so a teammate who closed the laptop never promises a quick
| answer. A visitor only ever learns a name and an avatar, never an email.
|
*/

package widget

import (
	"context"
	"fmt"
	"slices"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxTeamFaces is the most teammates a visitor sees.
*/
const maxTeamFaces = 3

/*
avatarLinkTTL is how long a teammate's avatar address works.
*/
const avatarLinkTTL = time.Hour

/*
Presence lists who holds a live connection on a channel.
*/
type Presence interface {
	/*
		Presence returns the connections listening on the channel.
	*/
	Presence(ctx context.Context, channel string) ([]fbroadcasting.Member, error)
}

/*
TeamFace is one teammate a visitor sees.
*/
type TeamFace struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
Availability is what a visitor is told about the team.
*/
type Availability struct {
	Online      bool
	WithinHours bool
	ReplyTime   string
	Team        []TeamFace
}

/*
ComputeTeamAvailability works out a widget's availability. Disk holds the
teammates' avatars, whose private files a visitor sees through a short-lived
address.
*/
type ComputeTeamAvailability struct {
	DB       *database.DB
	Clock    clock.Clock
	Presence Presence
	Disk     filesystem.Filesystem
}

/*
Handle returns the widget's availability.
*/
func (a *ComputeTeamAvailability) Handle(ctx context.Context, r Resolved) (Availability, error) {
	out := Availability{WithinHours: true, Team: []TeamFace{}}

	rows, online, err := a.team(ctx, r.Workspace.ID)
	if err != nil {
		return out, err
	}

	out.Online, out.Team = online, a.faces(ctx, rows)
	out.ReplyTime = out.replyTime(r.Widget.ReplyTime)

	return out, nil
}

/*
TeamOnline reports whether someone answers a workspace's visitors right now.
*/
func (a *ComputeTeamAvailability) TeamOnline(ctx context.Context, ws ids.UUID) (bool, error) {
	_, online, err := a.team(ctx, ws)

	return online, err
}

/*
team reads the workspace's active members and whether one is there.
*/
func (a *ComputeTeamAvailability) team(ctx context.Context, ws ids.UUID) ([]models.ListWidgetTeamRow, bool, error) {
	var rows []models.ListWidgetTeamRow

	err := a.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListWidgetTeam(ctx, ws)

		return err
	})
	if err != nil {
		return nil, false, fmt.Errorf("list team: %w", err)
	}

	present, err := a.Presence.Presence(ctx, appbroadcasting.WorkspaceChannel(ws))
	if err != nil {
		return nil, false, fmt.Errorf("read team presence: %w", err)
	}

	return rows, anyoneOnline(rows, present), nil
}

/*
anyoneOnline reports whether an available teammate holds a connection.
*/
func anyoneOnline(rows []models.ListWidgetTeamRow, present []fbroadcasting.Member) bool {
	return slices.ContainsFunc(rows, func(row models.ListWidgetTeamRow) bool {
		if row.Availability != string(enums.AvailabilityOnline) {
			return false
		}

		user := models.PublicID(models.PrefixUser, row.UserID)

		return slices.ContainsFunc(present, func(m fbroadcasting.Member) bool { return m.UserID == user })
	})
}

/*
replyTime resolves auto from whether someone is there.
*/
func (a Availability) replyTime(setting string) string {
	if setting != string(enums.WidgetReplyAuto) {
		return setting
	}

	if a.Online {
		return string(enums.WidgetReplyMinutes)
	}

	return string(enums.WidgetReplyHours)
}

/*
faces returns up to three teammates with an avatar. An avatar the disk cannot
give an address for just leaves that teammate out.
*/
func (a *ComputeTeamAvailability) faces(ctx context.Context, rows []models.ListWidgetTeamRow) []TeamFace {
	out := []TeamFace{}

	for _, row := range rows {
		if row.AvatarPath == nil || len(out) == maxTeamFaces {
			continue
		}

		address, err := a.Disk.TemporaryURL(ctx, *row.AvatarPath, avatarLinkTTL)
		if err != nil {
			continue
		}

		out = append(out, TeamFace{Name: row.Name, AvatarURL: &address})
	}

	return out
}

/*
Online reports whether someone on the widget's team is there now.
*/
func (a *ComputeTeamAvailability) Online(ctx context.Context, r Resolved) (bool, error) {
	_, online, err := a.team(ctx, r.Workspace.ID)

	return online, err
}
