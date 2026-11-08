/*
|--------------------------------------------------------------------------
| Procedures
|--------------------------------------------------------------------------
|
| The team writes procedures for requests it wants handled a set way.
| The agent sees each live procedure's name and when it applies in the
| description of one tool; when the customer's request matches, it
| calls the tool and gets the procedure's steps, naming the tool each
| action step calls, and when to hand over. Procedures are the team's
| own guidance, so they read as instructions, unlike anything the
| customer wrote. Every run that followed a procedure is flagged with
| it, which counts how often each one is used.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
toolProcedure is the procedure tool's name.
*/
const toolProcedure = "follow_procedure"

/*
liveProcedure is a procedure the agent may follow.
*/
type liveProcedure struct {
	row models.ListLiveProceduresRow
	def procedure.Definition
}

/*
publicID is the id the agent names the procedure by.
*/
func (p liveProcedure) publicID() string {
	return models.PublicID(models.PrefixProcedure, p.row.ID)
}

/*
loadProcedures reads the workspace's live procedures, leaving out any
that cannot be read.
*/
func loadProcedures(ctx context.Context, q *models.Queries, e *env) error {
	rows, err := q.ListLiveProcedures(ctx, e.Conversation.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list procedures: %w", err)
	}

	for _, r := range rows {
		def, err := procedure.Parse(r.Live)
		if err != nil {
			continue
		}

		e.Procedures = append(e.Procedures, liveProcedure{row: r, def: def})
	}

	return nil
}

/*
procedureSpec describes the procedure tool, listing every procedure by
its id, name and when it applies.
*/
func procedureSpec(e *env) contracts.ToolSpec {
	ids := make([]string, len(e.Procedures))
	var list strings.Builder
	for i, p := range e.Procedures {
		ids[i] = p.publicID()
		list.WriteString("\n- " + p.publicID() + ": " + p.row.Name + ". Use when: " + p.def.Description)
	}

	enum, _ := json.Marshal(ids)

	return contracts.ToolSpec{
		Name: toolProcedure,
		Description: "Get the steps your team wrote for a kind of request. When the customer's request matches " +
			"one of these, call this before answering, then do what the steps say, in order; call it again on " +
			"later turns while you are still following it. Procedures:" + list.String(),
		Schema: json.RawMessage(`{"type":"object","properties":{"procedure_id":{"type":"string","enum":` +
			string(enum) + `}},"required":["procedure_id"],"additionalProperties":false}`),
	}
}

/*
followProcedure returns the procedure's steps, naming the tool each of
its actions calls in this turn, and flags the run with it.
*/
func (r *Runner) followProcedure(_ context.Context, st *run, args json.RawMessage) (string, error) {
	var in struct {
		ID string `json:"procedure_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", errToolArguments
	}

	for _, p := range st.env.Procedures {
		if p.publicID() != in.ID {
			continue
		}

		st.flags, st.followed = appendOnce(st.flags, "procedure:"+p.row.ID.String()), true
		out, err := json.Marshal(map[string]string{"procedure": procedure.Render(p.row.Name, p.def, actionTools(st))})

		return string(out), err
	}

	return "", errToolArguments
}

/*
actionTools maps the public id of each action offered in the turn to
the tool that calls it.
*/
func actionTools(st *run) map[string]string {
	out := make(map[string]string, len(st.actions.offered))
	for _, live := range st.actions.offered {
		out[models.PublicID(models.PrefixAiAction, live.ActionID)] = live.Spec.Name
	}

	return out
}
