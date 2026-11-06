/*
|--------------------------------------------------------------------------
| Agent Handoff Settings
|--------------------------------------------------------------------------
|
| Where a handed-off conversation goes and when the agent must hand off.
| The settings page writes them and every agent turn reads them through
| the same reader, so both always agree on what is stored. A value that
| cannot be read means the defaults: the agent answers in full and no
| tag forces a handoff.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Handoff is the agent's handoff settings as stored: the team a
handed-off conversation goes to, whether its priority rises, the
agent's role (full or triage only) and the tags whose presence always
sends the conversation to a person.
*/
type Handoff struct {
	TargetTeamID  *ids.UUID           `json:"target_team_id,omitempty"`
	RaisePriority bool                `json:"raise_priority,omitempty"`
	Mode          enums.AIHandoffMode `json:"mode,omitempty"`
	AlwaysTags    []ids.UUID          `json:"always_tags,omitempty"`
}

/*
ReadHandoff reads the stored handoff settings. A value it cannot read
counts as none, and a missing or unknown mode reads as full.
*/
func ReadHandoff(raw json.RawMessage) Handoff {
	var h Handoff
	_ = json.Unmarshal(raw, &h)
	if !slices.Contains(enums.AIHandoffModes(), h.Mode) {
		h.Mode = enums.AIHandoffModeFull
	}
	return h
}

/*
mergeHandoff folds the handoff fields the caller sent into the stored
settings, keeping every field it left out.
*/
func mergeHandoff(raw json.RawMessage, p AgentPatch) json.RawMessage {
	if !p.HandoffTeamID.Set && p.RaisePrio == nil && p.HandoffMode == nil && p.AlwaysHandoffTags == nil {
		return raw
	}

	h := ReadHandoff(raw)
	setOptional(&h.TargetTeamID, p.HandoffTeamID)
	set(&h.RaisePriority, p.RaisePrio)
	set(&h.Mode, p.HandoffMode)
	if p.AlwaysHandoffTags != nil {
		h.AlwaysTags = distinct(*p.AlwaysHandoffTags)
	}

	out, err := json.Marshal(h)
	if err != nil {
		return raw
	}
	return out
}

/*
distinct returns the ids once each, in their first order.
*/
func distinct(list []ids.UUID) []ids.UUID {
	out := make([]ids.UUID, 0, len(list))
	for _, id := range list {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

/*
checkTags finds every tag that always hands off, when the caller named
some; one the workspace does not have refuses the save.
*/
func checkTags(ctx context.Context, tx *database.Tx, workspaceID ids.UUID, list *[]ids.UUID) error {
	if list == nil || len(*list) == 0 {
		return nil
	}

	want := distinct(*list)
	found, err := models.New(tx).TagsByIDs(ctx, models.TagsByIDsParams{WorkspaceID: workspaceID, Ids: want})
	if err != nil {
		return fmt.Errorf("find handoff tags: %w", err)
	}
	if len(found) != len(want) {
		return fault.New(fault.Invalid, "tag_not_found", "the tag does not exist").OnField("always_handoff_tag_ids")
	}
	return nil
}
