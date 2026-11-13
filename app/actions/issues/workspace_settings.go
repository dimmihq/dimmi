/*
|--------------------------------------------------------------------------
| Workspace Ticket Settings
|--------------------------------------------------------------------------
|
| The switches a workspace sets once: whether duplicate conversations
| are linked on their own. They live in the workspace's ticket settings
| beside switches other parts of tickets keep there, which a change
| leaves as they are.
|
*/

package issues

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketSettings are the workspace's switches.
*/
type TicketSettings struct {
	AutoLinkDuplicates bool `json:"auto_link_duplicates"`
}

/*
TicketSettingsChange changes the switches; nil leaves one.
*/
type TicketSettingsChange struct {
	AutoLinkDuplicates *bool
}

/*
readTicketSettings reads the switches and the raw settings they sit in.
*/
func readTicketSettings(ctx context.Context, q *models.Queries, ws ids.UUID) (TicketSettings, map[string]any, error) {
	raw, err := q.FindWorkspaceTicketSettings(ctx, ws)
	if err != nil {
		return TicketSettings{}, nil, fmt.Errorf("read ticket settings: %w", err)
	}

	var out TicketSettings
	_ = json.Unmarshal(raw, &out)

	return out, decodeObject(raw), nil
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
			TicketSettings: encoded, WorkspaceID: m.Workspace.ID,
		})
	})

	return out, err
}

/*
applySettings sets the changed switches on the settings and on the raw
object they are stored in.
*/
func applySettings(out TicketSettings, raw map[string]any, c TicketSettingsChange) TicketSettings {
	if c.AutoLinkDuplicates != nil {
		out.AutoLinkDuplicates, raw["auto_link_duplicates"] = *c.AutoLinkDuplicates, *c.AutoLinkDuplicates
	}

	return out
}
