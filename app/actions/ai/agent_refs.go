/*
|--------------------------------------------------------------------------
| Agent References
|--------------------------------------------------------------------------
|
| An agent names other rows: the procedures, actions and knowledge it may
| use, and the channels and inboxes it answers on. A save that names a
| row the workspace does not have is refused, naming the field.
|
*/

package ai

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
refCheck is one list of ids, the field it came in and the kind of row
it names.
*/
type refCheck struct {
	field string
	kind  string
	list  []ids.UUID
}

/*
checkScopes refuses a patch whose scopes or placement name a row the
workspace does not have.
*/
func checkScopes(ctx context.Context, q *models.Queries, workspaceID ids.UUID, p AgentPatch) error {
	var checks []refCheck
	if p.Procedures != nil {
		checks = append(checks, refCheck{"procedure_scope", "procedures", p.Procedures.IDs})
	}
	if p.Actions != nil {
		checks = append(checks, refCheck{"action_scope", "actions", p.Actions.IDs})
	}
	if k := p.Knowledge; k != nil {
		checks = append(checks,
			refCheck{"knowledge_scope", "sources", k.SourceIDs},
			refCheck{"knowledge_scope", "help_centers", k.HelpCenterIDs},
		)
	}
	if pl := p.Placement; pl != nil {
		checks = append(checks,
			refCheck{"placement", "connections", pl.ChannelConnectionIDs},
			refCheck{"placement", "rooms", pl.ChannelRoomIDs},
			refCheck{"placement", "inboxes", pl.InboxIDs},
		)
	}

	for _, c := range checks {
		c.list = distinct(c.list)
		if len(c.list) == 0 {
			continue
		}
		n, err := countRefs(ctx, q, workspaceID, c)
		if err != nil {
			return fmt.Errorf("check %s: %w", c.field, err)
		}
		if int(n) != len(c.list) {
			return fault.New(fault.Invalid, "reference_not_found", "it names something the workspace does not have").
				OnField(c.field)
		}
	}
	return nil
}

/*
countRefs counts how many of the ids are rows of one kind the workspace
has.
*/
func countRefs(ctx context.Context, q *models.Queries, ws ids.UUID, c refCheck) (int32, error) {
	list := c.list
	switch c.kind {
	case "procedures":
		return q.CountWorkspaceProcedures(ctx, models.CountWorkspaceProceduresParams{WorkspaceID: ws, Ids: list})
	case "actions":
		return q.CountWorkspaceActions(ctx, models.CountWorkspaceActionsParams{WorkspaceID: ws, Ids: list})
	case "sources":
		return q.CountWorkspaceKnowledgeSources(ctx,
			models.CountWorkspaceKnowledgeSourcesParams{WorkspaceID: ws, Ids: list})
	case "help_centers":
		return q.CountWorkspaceHelpCenters(ctx, models.CountWorkspaceHelpCentersParams{WorkspaceID: ws, Ids: list})
	case "connections":
		return q.CountWorkspaceConnections(ctx, models.CountWorkspaceConnectionsParams{WorkspaceID: ws, Ids: list})
	case "rooms":
		return q.CountWorkspaceRooms(ctx, models.CountWorkspaceRoomsParams{WorkspaceID: ws, Ids: list})
	default:
		return q.CountWorkspaceInboxes(ctx, models.CountWorkspaceInboxesParams{WorkspaceID: ws, Ids: list})
	}
}
