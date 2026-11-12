/*
|--------------------------------------------------------------------------
| Workspace Ticket Settings
|--------------------------------------------------------------------------
|
| The switches a workspace sets once: whether incidents
| opened in an engineering tool are mirrored as dimmi incidents and from
| which severity, and how many conversations about one thing in the
| spike window suggest an incident. They live in the workspace's ticket
| settings beside switches other parts of tickets keep there, which a
| change leaves as they are.
|
*/

package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
mirrorLevels are the severity levels an incident must reach to be
mirrored.
*/
func mirrorLevels() []string {
	return []string{"all", "minor", "major", "critical"}
}

/*
Failures of the workspace's settings.
*/
var (
	ErrMirrorLevel = fault.New(fault.Invalid, "incident_mirror_severity_invalid",
		"The severity is all, minor, major or critical.").OnField("incident_mirror_severity")
	ErrSpikeThreshold = fault.New(fault.Invalid, "spike_threshold_invalid",
		"The spike threshold is 2 to 1000 conversations.").OnField("spike_threshold")
)

/*
TicketSettings are the workspace's switches.
*/
type TicketSettings struct {
	IncidentMirror         bool   `json:"incident_mirror"`
	IncidentMirrorSeverity string `json:"incident_mirror_severity"`
	AutoLinkDuplicates     bool   `json:"auto_link_duplicates"`
	SpikeThreshold         int32  `json:"-"`
}

/*
TicketSettingsChange changes the switches; nil leaves one.
*/
type TicketSettingsChange struct {
	IncidentMirror         *bool
	IncidentMirrorSeverity *string
	AutoLinkDuplicates     *bool
	SpikeThreshold         *int32
}

/*
readTicketSettings reads the switches and the raw settings they sit in.
*/
func readTicketSettings(ctx context.Context, q *models.Queries, ws ids.UUID) (TicketSettings, map[string]any, error) {
	row, err := q.FindWorkspaceTicketSettings(ctx, ws)
	if err != nil {
		return TicketSettings{}, nil, fmt.Errorf("read ticket settings: %w", err)
	}

	out := TicketSettings{IncidentMirrorSeverity: "major"}
	_ = json.Unmarshal(row.TicketSettings, &out)
	out.SpikeThreshold = row.IncidentSpikeThreshold

	if !slices.Contains(mirrorLevels(), out.IncidentMirrorSeverity) {
		out.IncidentMirrorSeverity = "major"
	}

	return out, decodeObject(row.TicketSettings), nil
}

/*
WorkspaceSettings returns the workspace's switches.
*/
func (s *Issues) WorkspaceSettings(ctx context.Context, m authz.Membership) (TicketSettings, error) {
	if err := tickets.MayRead(m); err != nil {
		return TicketSettings{}, err
	}

	var out TicketSettings

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, _, err = readTicketSettings(ctx, q, m.Workspace.ID)

		return err
	})

	return out, err
}

/*
UpdateWorkspaceSettings changes the switches. Teammates who manage
tickets may.
*/
func (s *Issues) UpdateWorkspaceSettings(ctx context.Context, m authz.Membership, c TicketSettingsChange,
) (TicketSettings, error) {
	if !m.Can("tickets:manage") {
		return TicketSettings{}, exceptions.ErrForbidden
	}

	if c.IncidentMirrorSeverity != nil && !slices.Contains(mirrorLevels(), *c.IncidentMirrorSeverity) {
		return TicketSettings{}, ErrMirrorLevel
	}

	if c.SpikeThreshold != nil && (*c.SpikeThreshold < 2 || *c.SpikeThreshold > 1000) {
		return TicketSettings{}, ErrSpikeThreshold
	}

	var out TicketSettings

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		current, raw, err := readTicketSettings(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}

		out = applySettings(current, raw, c)

		encoded, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("encode ticket settings: %w", err)
		}

		return q.SaveWorkspaceTicketSettings(ctx, models.SaveWorkspaceTicketSettingsParams{
			TicketSettings: encoded, IncidentSpikeThreshold: out.SpikeThreshold, WorkspaceID: m.Workspace.ID,
		})
	})

	return out, err
}

/*
applySettings sets the changed switches on the settings and on the raw
object they are stored in.
*/
func applySettings(out TicketSettings, raw map[string]any, c TicketSettingsChange) TicketSettings {
	if c.IncidentMirror != nil {
		out.IncidentMirror, raw["incident_mirror"] = *c.IncidentMirror, *c.IncidentMirror
	}

	if c.IncidentMirrorSeverity != nil {
		out.IncidentMirrorSeverity = *c.IncidentMirrorSeverity
		raw["incident_mirror_severity"] = *c.IncidentMirrorSeverity
	}

	if c.AutoLinkDuplicates != nil {
		out.AutoLinkDuplicates, raw["auto_link_duplicates"] = *c.AutoLinkDuplicates, *c.AutoLinkDuplicates
	}

	if c.SpikeThreshold != nil {
		out.SpikeThreshold = *c.SpikeThreshold
	}

	return out
}
