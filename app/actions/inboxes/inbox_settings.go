/*
|--------------------------------------------------------------------------
| Inbox Settings Changes
|--------------------------------------------------------------------------
|
| A change to an inbox's settings is a patch on what is stored: keys it
| names replace theirs, keys it sends as null go back to their default,
| and everything else stays. The result is checked as a whole before it
| is written, so an inbox never holds a setting it cannot act on, such as
| an overflow team that is its own default team.
|
*/

package inboxes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on inbox settings.
*/
const (
	maxReopenWindowDays = 365
	maxCapacity         = 500
)

/*
color is the shape of an inbox's color.
*/
var color = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

/*
mergeSettings applies the patch to the stored settings, as a JSON merge
patch, and reads the result with its defaults.
*/
func mergeSettings(stored, patch json.RawMessage) (models.InboxSettings, json.RawMessage, error) {
	base := map[string]any{}
	_ = json.Unmarshal(stored, &base)

	changes := map[string]any{}
	if err := json.Unmarshal(patch, &changes); err != nil {
		return models.InboxSettings{}, nil, exceptions.ErrInboxSettingInvalid.OnField("settings")
	}

	merged, err := json.Marshal(merge(base, changes))
	if err != nil {
		return models.InboxSettings{}, nil, fmt.Errorf("encode inbox settings: %w", err)
	}

	var typed models.InboxSettings
	if err := json.Unmarshal(merged, &typed); err != nil {
		return models.InboxSettings{}, nil, exceptions.ErrInboxSettingInvalid.OnField("settings")
	}

	return models.ReadInboxSettings(merged), merged, nil
}

/*
merge applies the changes to the base: null removes a key, an object
merges into the one it replaces, anything else replaces.
*/
func merge(base, changes map[string]any) map[string]any {
	for key, value := range changes {
		if value == nil {
			delete(base, key)

			continue
		}

		nested, ok := value.(map[string]any)
		current, wasMap := base[key].(map[string]any)

		if ok && wasMap {
			base[key] = merge(current, nested)

			continue
		}

		base[key] = value
	}

	return base
}

/*
checkSettings refuses settings the inbox cannot act on.
*/
func checkSettings(s models.InboxSettings) error {
	if s.ReopenWindowDays < 0 || s.ReopenWindowDays > maxReopenWindowDays {
		return exceptions.ErrInboxSettingInvalid.OnField("settings.reopen_window_days")
	}

	if s.Color != "" && !color.MatchString(s.Color) {
		return exceptions.ErrInboxSettingInvalid.OnField("settings.color")
	}

	return checkAssignment(s.Assignment)
}

/*
checkAssignment refuses assignment settings outside what each accepts.
*/
func checkAssignment(a models.AssignmentSettings) error {
	modes := []string{models.AssignmentManual, models.AssignmentRoundRobin, models.AssignmentBalanced}
	if !slices.Contains(modes, a.Mode) {
		return exceptions.ErrInboxSettingInvalid.OnField("settings.assignment.mode")
	}

	if !slices.Contains([]string{models.QueueOldestWaiting, models.QueuePriorityThenOldest}, a.QueueOrder) {
		return exceptions.ErrInboxSettingInvalid.OnField("settings.assignment.queue_order")
	}

	if a.DefaultCapacity != nil && (*a.DefaultCapacity < 1 || *a.DefaultCapacity > maxCapacity) {
		return exceptions.ErrInboxSettingInvalid.OnField("settings.assignment.default_capacity")
	}

	channels := []string{string(enums.ChannelTypeWidget), string(enums.ChannelTypeEmail), string(enums.ChannelTypeAPI)}

	for channel, limit := range a.CapacityByChannel {
		if !slices.Contains(channels, channel) || limit < 1 || limit > maxCapacity {
			return exceptions.ErrInboxSettingInvalid.OnField("settings.assignment.capacity_by_channel")
		}
	}

	return nil
}

/*
checkTeams refuses a default or overflow team the inbox's workspace
does not have, and an overflow team that is the default team.
*/
func checkTeams(ctx context.Context, q *models.Queries, inbox models.Inbox, s models.InboxSettings) error {
	team, overflow := inbox.DefaultTeamID, s.Assignment.OverflowTeamID

	if overflow != nil && team != nil && *overflow == *team {
		return exceptions.ErrOverflowIsDefaultTeam
	}

	fields := map[string]*ids.UUID{"default_team_id": team, "settings.assignment.overflow_team_id": overflow}

	for field, id := range fields {
		if id == nil {
			continue
		}

		_, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: inbox.WorkspaceID, ID: *id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrInboxTeamNotFound.OnField(field)
		}

		if err != nil {
			return fmt.Errorf("find inbox team: %w", err)
		}
	}

	return nil
}
