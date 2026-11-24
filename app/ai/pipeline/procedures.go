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
| A model sometimes asks for a procedure the request has nothing to do
| with. When a decision model is configured, the first time a
| conversation asks for a procedure it is asked whether each procedure
| fits what the customer wants now. A procedure that plainly does not
| fit is held back with a note, or swapped for the one that does; the
| agent gets it anyway by asking again. With no decision model, or one
| that fails, the steps are returned as asked.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
toolProcedure is the procedure tool's name.
*/
const toolProcedure = "follow_procedure"

/*
liveProcedure is a procedure the agent may follow, with the number of
its live version.
*/
type liveProcedure struct {
	row     models.ListLiveProceduresRow
	def     procedure.Definition
	version int32
}

/*
publicID is the id the agent names the procedure by.
*/
func (p liveProcedure) publicID() string {
	return models.PublicID(models.PrefixProcedure, p.row.ID)
}

/*
loadProcedures reads the workspace's live procedures, leaving out any
that cannot be read. Every one is kept for following as a sub-procedure;
those the agent is offered exclude the reusable-only ones and those
scoped away from this conversation.
*/
func (r *Runner) loadProcedures(ctx context.Context, q *models.Queries, e *env) error {
	rows, err := q.ListLiveProcedures(ctx, e.Conversation.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list procedures: %w", err)
	}

	for _, row := range rows {
		def, err := procedure.Parse(row.Live)
		if err != nil {
			continue
		}

		e.AllProcedures = append(e.AllProcedures, liveProcedure{row: row, def: def, version: row.Version})
	}

	e.Procedures, err = r.inScope(ctx, q, e, e.AllProcedures)

	return err
}

/*
procedureSpec describes the procedure tool, listing every procedure by
its id, name and when it applies, the one being followed included.
*/
func procedureSpec(st *run) contracts.ToolSpec {
	listed := slices.Clone(st.env.Procedures)
	if st.proc != nil && !slices.ContainsFunc(listed, func(p liveProcedure) bool {
		return p.row.ID == st.proc.live.row.ID
	}) {
		listed = append(listed, st.proc.live)
	}
	known := make([]string, len(listed))
	var list strings.Builder
	for i, p := range listed {
		known[i] = p.publicID()
		list.WriteString("\n- " + p.publicID() + ": " + p.row.Name + ". Use when: " + p.def.Description)
	}

	return contracts.ToolSpec{
		Name: toolProcedure,
		Description: "Get the steps your team wrote for a kind of request. Check this list on every customer " +
			"message. When the request matches one, call this first, before searching or answering, then do what " +
			"the steps say, in order, in place of your usual approach; call it again on later turns while you are " +
			"still following it. Procedures:" + list.String(),
		Schema: objectSchema(map[string]any{"procedure_id": enumProperty(known)}, "procedure_id"),
	}
}

/*
A procedure the decision model scores below procedureMisfit does not
fit the request; one it scores at procedureFit or above does. Between
the two the agent's own choice stands. The model reads at most
procedureTurns of the conversation's latest messages, each cut to
procedureChars.
*/
const (
	procedureMisfit = 0.05
	procedureFit    = 0.7
	procedureTurns  = 12
	procedureChars  = 1200
)

/*
followProcedure returns the procedure's steps, naming the tool each of
its actions calls in this turn, and flags the run with it. A procedure
that does not fit the request comes back as a note instead, with the
steps of the one that fits when there is one.
*/
func (r *Runner) followProcedure(ctx context.Context, st *run, args json.RawMessage) (string, error) {
	var in struct {
		ID string `json:"procedure_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", errToolArguments
	}

	for _, p := range r.followable(st) {
		if p.publicID() != in.ID {
			continue
		}

		use, note := r.fittingProcedure(ctx, st, p)
		body := map[string]string{}
		if note != "" {
			body["note"] = note
		}
		if use != nil {
			st.flags, st.followed = appendOnce(st.flags, "procedure:"+use.row.ID.String()), true
			var fit *float64
			if score, ok := st.procedureFits[use.publicID()]; ok {
				fit = &score
			}
			r.startProcedure(ctx, st, *use, fit)
			r.ensureProcedureActions(ctx, st)
			body["procedure"] = r.renderProcedure(st)
			if len(st.proc.articles) > 0 {
				body["articles"] = strings.Join(st.proc.articles, "\n")
			}
			body["note"] = strings.TrimSpace(body["note"] + " Follow these steps now. " + stepByStep +
				" Tools marked required are checked before your reply is sent. Call " + toolProcedureStatus +
				" with completed when every step is done, or exited_topic_changed when the customer moves on.")
		}
		out, err := json.Marshal(body)

		return string(out), err
	}

	return "", errToolArguments
}

/*
followable is every procedure the agent may ask for: those offered,
and the one being followed.
*/
func (r *Runner) followable(st *run) []liveProcedure {
	out := slices.Clone(st.env.Procedures)
	if st.proc != nil && !slices.ContainsFunc(out, func(p liveProcedure) bool {
		return p.row.ID == st.proc.live.row.ID
	}) {
		out = append(out, st.proc.live)
	}

	return out
}

/*
fittingProcedure returns the procedure to hand over and a note for the
agent: the asked one with no note when it fits or was not checked,
another with a note when that one fits and the asked one does not, and
none with a note when nothing fits. A procedure is checked once: asked
for again in the run, or followed earlier in the conversation, it is
handed over. The decision model is asked once in a run, whatever the
number of procedures the agent asks for.
*/
func (r *Runner) fittingProcedure(ctx context.Context, st *run, asked liveProcedure) (*liveProcedure, string) {
	id := asked.publicID()
	if r.Decider == nil || st.procedureAsked[id] || r.followedBefore(ctx, st, asked) {
		return &asked, ""
	}
	if st.procedureAsked == nil {
		st.procedureAsked = map[string]bool{}
	}
	st.procedureAsked[id] = true

	if st.procedureFits == nil {
		st.procedureFits = r.procedureFits(ctx, st)
	}
	fits := st.procedureFits
	score, ok := fits[id]
	if !ok || score >= procedureMisfit {
		return &asked, ""
	}

	var better *liveProcedure
	for _, p := range st.env.Procedures {
		if s := fits[p.publicID()]; s >= procedureFit && (better == nil || s > fits[better.publicID()]) {
			better = &p
		}
	}
	if better != nil {
		st.procedureAsked[better.publicID()] = true

		return better, "You asked for \"" + asked.row.Name + "\", which is for when: " + asked.def.Description +
			" What the customer wants now fits \"" + better.row.Name + "\" instead, so these are its steps. " +
			"If you are sure \"" + asked.row.Name + "\" is the right one, call this again with its procedure_id."
	}

	return nil, "The steps were not loaded. \"" + asked.row.Name + "\" is for when: " + asked.def.Description +
		" What the customer wants now does not look like that, and no other procedure fits it either. " +
		"Help the customer as you normally would, and ask them if it is not clear what they need. " +
		"If you are sure this procedure is the right one, call this again with the same procedure_id."
}

/*
followedBefore reports whether an earlier run of the conversation
followed the procedure.
*/
func (r *Runner) followedBefore(ctx context.Context, st *run, p liveProcedure) bool {
	if st.rec == nil || st.env.Conversation.ID == (ids.UUID{}) {
		return false
	}

	var before bool
	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		before, err = models.New(tx).ConversationFollowedProcedure(ctx, models.ConversationFollowedProcedureParams{
			WorkspaceID: st.env.Workspace.ID, ConversationID: &st.env.Conversation.ID, Before: st.rec.Run.ID,
			Flag: "procedure:" + p.row.ID.String(),
		})

		return err
	})

	return before
}

/*
procedureFits asks the decision model, in one call, how likely each
procedure is to fit what the customer wants now, by public id. It
records the call as a step and returns nothing when the model fails.
*/
func (r *Runner) procedureFits(ctx context.Context, st *run) map[string]float64 {
	req := contracts.DecisionRequest{
		State: r.procedureState(st.env), Questions: map[string]contracts.DecisionQuestion{},
	}
	for _, p := range st.env.Procedures {
		req.Questions[p.publicID()] = question("procedure_fits", prompts.Vars{
			"name": p.row.Name, "description": p.def.Description,
		})
	}

	fits := map[string]float64{}
	for id, a := range r.decide(ctx, st, req) {
		fits[id] = a.Noul
	}

	return fits
}

/*
procedureState is what the decision model reads: the conversation's
latest messages, the customer's newest last.
*/
func (r *Runner) procedureState(e *env) string {
	return prompts.Judgment("procedure_fits", prompts.Vars{"conversation": r.conversationLines(e)}).State
}

/*
conversationLines is the conversation's latest messages as the decision
model reads them, one line each, the customer's newest last.
*/
func (r *Runner) conversationLines(e *env) string {
	all := append(append([]models.Message{}, e.History...), e.Pending...)
	if len(all) > procedureTurns {
		all = all[len(all)-procedureTurns:]
	}

	var b strings.Builder
	for _, m := range all {
		text := body(m)
		if text == "" {
			continue
		}
		who := "AGENT"
		if m.AuthorType == string(enums.AuthorTypeContact) {
			who = "CUSTOMER"
		}
		b.WriteString("\n" + who + ": " + truncate(r.redactFor(e, text), procedureChars))
	}

	return b.String()
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
