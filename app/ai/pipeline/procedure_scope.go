/*
|--------------------------------------------------------------------------
| Where A Procedure Applies
|--------------------------------------------------------------------------
|
| A procedure may be narrowed to some channels, inboxes, saved
| audiences, or signed-in customers, and a reusable one is only ever
| followed from inside another. The agent is offered only the
| procedures that apply to this conversation; an audience is judged on
| the customer's trusted attributes alone.
|
| A workspace with many procedures would bury the agent in them, so past
| procedureShortlist eligible ones the agent sees the procedureTop
| closest to what the customer asks: by the meaning of their latest
| messages against each description when the workspace embeds, else by
| the words they share. The procedure being followed is always offered.
|
*/

package pipeline

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Routing limits: past procedureShortlist eligible procedures the agent
sees only the procedureTop closest.
*/
const (
	procedureShortlist = 15
	procedureTop       = 12
)

/*
ProcedureVectors embeds text for routing procedures: a description as
a stored document, a customer's words as a query, each returning the
vector and the key of the model that made it.
*/
type ProcedureVectors interface {
	/*
		EmbedDocument embeds a description as a stored document.
	*/
	EmbedDocument(ctx context.Context, workspaceID ids.UUID, text string) ([]float32, string, error)

	/*
		EmbedQuery embeds a customer's words as a query.
	*/
	EmbedQuery(ctx context.Context, workspaceID ids.UUID, text string) ([]float32, string, error)
}

/*
inScope keeps the procedures that apply to the conversation.
*/
func (r *Runner) inScope(ctx context.Context, q *models.Queries, e *env, all []liveProcedure) ([]liveProcedure, error) {
	identity := identityOf(e.ChannelType, e.Trigger, e.Contact)
	scope := agents.ProceduresOf(e.Agent)
	var out []liveProcedure
	var needed []ids.UUID
	for _, p := range all {
		if !scope.Allows(p.row.ID) || !applies(e, identity, p) {
			continue
		}
		needed = appendAudiences(needed, p)
		out = append(out, p)
	}
	if len(needed) == 0 {
		return out, nil
	}
	allowed, err := r.allowedAudiences(ctx, q, e, needed)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(out, func(p liveProcedure) bool { return !inAudience(p, allowed) }), nil
}

/*
applies reports whether a procedure's channels, inboxes and identity
rule allow the conversation; a reusable-only one never applies.
*/
func applies(e *env, identity actionkit.Identity, p liveProcedure) bool {
	s := p.def.Scope
	if s == nil {
		return true
	}
	inbox := models.PublicID(models.PrefixInbox, e.Conversation.InboxID)
	emailOnly := identity.EmailProven && len(procedureActionIDs(e, p)) == 0
	switch {
	case s.ReusableOnly:
		return false
	case len(s.Channels) > 0 && !slices.Contains(s.Channels, e.ChannelType):
		return false
	case len(s.InboxIDs) > 0 && !slices.Contains(s.InboxIDs, inbox):
		return false
	case s.RequireVerifiedIdentity && !identity.Verified && !emailOnly:
		return false
	}
	return true
}

/*
appendAudiences adds the saved audiences a procedure is for.
*/
func appendAudiences(needed []ids.UUID, p liveProcedure) []ids.UUID {
	if p.def.Scope == nil {
		return needed
	}
	for _, a := range p.def.Scope.AudienceIDs {
		if id, ok := decodePublic(models.PrefixAudience, a); ok && !slices.Contains(needed, id) {
			needed = append(needed, id)
		}
	}
	return needed
}

/*
inAudience reports whether the customer is in one of a procedure's
audiences, or it names none.
*/
func inAudience(p liveProcedure, allowed []ids.UUID) bool {
	if p.def.Scope == nil || len(p.def.Scope.AudienceIDs) == 0 {
		return true
	}
	return slices.ContainsFunc(p.def.Scope.AudienceIDs, func(a string) bool {
		id, ok := decodePublic(models.PrefixAudience, a)
		return ok && slices.Contains(allowed, id)
	})
}

/*
allowedAudiences lists the audiences of those needed the customer is in,
judged on their trusted attributes; a conversation without a customer
is in none.
*/
func (r *Runner) allowedAudiences(ctx context.Context, q *models.Queries, e *env, needed []ids.UUID,
) ([]ids.UUID, error) {
	if e.Contact == nil {
		return nil, nil
	}
	people, err := audiences.LoadPeople(ctx, q, e.Conversation.WorkspaceID, []models.Contact{*e.Contact})
	if err != nil || len(people) == 0 {
		return nil, err
	}
	person := people[0]
	person.TrustedOnly = true

	return audiences.Allowed(ctx, q, audiences.Reader{
		WorkspaceID: e.Conversation.WorkspaceID, Person: &person, Needed: needed, Now: e.now,
	})
}

/*
shortlistProcedures narrows a long list of procedures to the closest to
what the customer asks, keeping the one being followed.
*/
func (r *Runner) shortlistProcedures(ctx context.Context, st *run) {
	e := st.env
	if len(e.Procedures) <= procedureShortlist {
		return
	}
	query := r.redactFor(e, e.pendingText())
	ranked := r.rankByVector(ctx, e, query)
	if len(ranked) == 0 {
		ranked = rankByWords(e.Procedures, query)
	}
	keep := ranked[:min(procedureTop, len(ranked))]
	if a := e.ActiveProcedure; a != nil && !slices.Contains(keep, a.proc.row.ID) {
		keep = append(keep, a.proc.row.ID)
	}
	e.Procedures = slices.DeleteFunc(e.Procedures, func(p liveProcedure) bool {
		return !slices.Contains(keep, p.row.ID)
	})
	st.flags = appendOnce(st.flags, "procedures_shortlisted")
}

/*
rankByVector orders the live procedures by how close their description
is to the query, nothing when the workspace does not embed or a
procedure has no vector yet.
*/
func (r *Runner) rankByVector(ctx context.Context, e *env, query string) []ids.UUID {
	if r.ProcedureVectors == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	vector, model, err := r.ProcedureVectors.EmbedQuery(ctx, e.Workspace.ID, query)
	if err != nil {
		return nil
	}
	var rows []models.RankLiveProceduresRow
	err = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err = models.New(tx).RankLiveProcedures(ctx, models.RankLiveProceduresParams{
			Query: VectorText(vector), WorkspaceID: e.Workspace.ID, ModelKey: &model, RowLimit: 200,
		})
		return err
	})
	if err != nil {
		return nil
	}
	var out []ids.UUID
	for _, row := range rows {
		if slices.ContainsFunc(e.Procedures, func(p liveProcedure) bool { return p.row.ID == row.ID }) {
			out = append(out, row.ID)
		}
	}
	if len(out) < len(e.Procedures) {
		for _, id := range rankByWords(e.Procedures, query) {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out
}

/*
rankByWords orders procedures by how many of the query's words their
name and description share, ties by name.
*/
func rankByWords(procs []liveProcedure, query string) []ids.UUID {
	words := wordSet(query)
	type scored struct {
		id    ids.UUID
		name  string
		score int
	}
	list := make([]scored, 0, len(procs))
	for _, p := range procs {
		n := 0
		for w := range wordSet(p.row.Name + " " + p.def.Description) {
			if words[w] {
				n++
			}
		}
		list = append(list, scored{id: p.row.ID, name: p.row.Name, score: n})
	}
	slices.SortStableFunc(list, func(a, b scored) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return strings.Compare(a.name, b.name)
	})
	out := make([]ids.UUID, len(list))
	for i, s := range list {
		out[i] = s.id
	}
	return out
}

/*
wordSet is the lowercased words of a text longer than two letters.
*/
func wordSet(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

/*
VectorText writes a vector as pgvector reads it.
*/
func VectorText(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.FormatFloat(float64(x), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
