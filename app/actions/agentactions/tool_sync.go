/*
|--------------------------------------------------------------------------
| MCP Tool Sync
|--------------------------------------------------------------------------
|
| Connecting a remote MCP server turns its tools into draft actions the
| admin reviews and publishes. Every new tool starts as a write that
| asks the customer to confirm: a server's own claim that a tool only
| reads is not trusted until the admin says so. Tools already turned
| into actions are left alone, and a tool whose schema falls outside the
| portable subset is skipped with the reason.
|
*/

package agentactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mcp"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionschema"
)

/*
ToolSync is what a sync did: the draft actions it created, how many
tools already had one, and the tools it skipped with why.
*/
type ToolSync struct {
	Created  []models.AiAction
	Existing int
	Skipped  []SkippedTool
}

/*
SkippedTool is a tool a sync could not turn into an action.
*/
type SkippedTool struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

/*
SyncTools turns the MCP server's new tools into draft actions.
*/
func (c *Connections) SyncTools(ctx context.Context, m authz.Membership, id ids.UUID) (ToolSync, error) {
	conn, err := c.manageable(ctx, m, id)
	if err != nil {
		return ToolSync{}, err
	}
	if conn.Kind != "mcp" {
		return ToolSync{}, exceptions.ErrConnectionNotMCP
	}
	tools, err := c.Kit.ListTools(ctx, conn)
	if err != nil {
		return ToolSync{}, exceptions.ErrMCPServerIncompatible
	}

	var out ToolSync
	err = c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s := &syncing{q: models.New(tx), m: m, conn: conn, out: &out}
		for _, tool := range tools {
			if err := c.syncTool(ctx, s, tool); err != nil {
				return err
			}
		}
		return models.New(tx).RecordActionConnectionSync(ctx, models.RecordActionConnectionSyncParams{
			WorkspaceID: conn.WorkspaceID, ID: conn.ID, McpServerInfo: json.RawMessage("{}"), Now: c.Clock.Now(),
		})
	})
	return out, err
}

/*
syncing is a sync in progress: its queries, who runs it, the connection
and what it did so far.
*/
type syncing struct {
	q    *models.Queries
	m    authz.Membership
	conn models.AiActionConnection
	out  *ToolSync
}

/*
syncTool creates the draft for one tool, unless it has one.
*/
func (c *Connections) syncTool(ctx context.Context, s *syncing, tool mcp.ToolInfo) error {
	q, conn, out := s.q, s.conn, s.out
	_, err := q.FindActionByToolName(ctx, models.FindActionByToolNameParams{
		WorkspaceID: conn.WorkspaceID, ConnectionID: conn.ID, McpToolName: &tool.Name,
	})
	if err == nil {
		out.Existing++
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find synced action: %w", err)
	}
	name, err := freeName(ctx, q, conn.WorkspaceID, tool.Name)
	if err != nil {
		return err
	}
	spec, reason := draftOf(conn, tool, name)
	if reason != "" {
		out.Skipped = append(out.Skipped, SkippedTool{Tool: tool.Name, Reason: reason})
		return nil
	}
	row, err := q.CreateAction(ctx, createParams(conn.WorkspaceID, spec, DefinitionInput{
		Membership: s.m, Source: "mcp_sync",
	}, c.Clock))
	if err != nil {
		return actionWriteError(err)
	}
	out.Created = append(out.Created, row)
	return nil
}

/*
draftOf is the checked draft of a tool, or why there can be none.
*/
func draftOf(conn models.AiActionConnection, tool mcp.ToolInfo, name string) (actionkit.Spec, string) {
	schema, err := actionschema.Normalize(tool.InputSchema)
	if err != nil {
		return actionkit.Spec{}, "schema_unsupported"
	}
	spec, err := actionkit.Normalize(toolSpec(conn, tool, name, schema), conn)
	if err != nil {
		return actionkit.Spec{}, "definition_invalid"
	}
	return spec, ""
}

/*
toolSpec is the draft definition of a tool.
*/
func toolSpec(conn models.AiActionConnection, tool mcp.ToolInfo, name string, schema json.RawMessage,
) actionkit.Spec {
	title := tool.Title
	if title == "" {
		title = strings.ReplaceAll(tool.Name, "_", " ")
	}
	description := strings.TrimSpace(tool.Description)
	if len([]rune(description)) < 10 {
		description = "Calls the " + tool.Name + " tool. " + description
	}
	return actionkit.Spec{
		ConnectionID: conn.ID, Name: name, Title: clip(title, 80), Description: clip(description, 1024),
		Kind: actionkit.KindMCPTool, SideEffect: actionkit.SideEffectWrite,
		Confirmation: actionkit.ConfirmationCustomer, Approval: actionkit.ApprovalNone,
		MCPToolName: tool.Name, InputSchema: schema,
	}
}

/*
freeName turns a tool name into a tool name of ours no action holds yet.
*/
func freeName(ctx context.Context, q *models.Queries, workspaceID ids.UUID, tool string) (string, error) {
	base := sanitize(tool)
	for i := 1; i < 100; i++ {
		name := base
		if i > 1 {
			suffix := "_" + strconv.Itoa(i)
			name = clip(base, 48-len(suffix)) + suffix
		}
		_, err := q.FindActionByName(ctx, models.FindActionByNameParams{WorkspaceID: workspaceID, Name: name})
		if errors.Is(err, database.ErrNotFound) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("find action name: %w", err)
		}
	}
	return "", exceptions.ErrActionNameTaken
}

/*
sanitize lowercases a tool name into the tool-name alphabet.
*/
func sanitize(tool string) string {
	name := regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(strings.ToLower(tool), "_")
	name = strings.Trim(name, "_")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "tool_" + name
	}
	if actionkit.ReservedName(name) {
		name = "mcp_" + name
	}
	for len(name) < 3 {
		name += "_"
	}
	return clip(name, 48)
}

/*
clip keeps at most n runes.
*/
func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
