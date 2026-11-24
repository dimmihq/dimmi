/*
|--------------------------------------------------------------------------
| Writing Tools
|--------------------------------------------------------------------------
|
| The three ways Operator leaves something behind. A reply draft goes to
| the teammate's reply box and is never sent. A proposal becomes a
| changeset that waits for a teammate, or applies at once when every
| group it touches is set to auto and the run read nothing a customer
| wrote. A memory is kept only because the teammate asked.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of what the writing tools accept.
*/
const (
	maxDraft      = 20000
	maxMemory     = 1000
	maxOperations = 50
)

/*
writeTools are the tools that leave something behind.
*/
func (e *Engine) writeTools() []tool {
	return append(append(e.articleTools(), e.procedureEditTool()), []tool{
		{spec: contracts.ToolSpec{
			Name: "draft_reply",
			Description: "Draft a reply to the customer of this thread's conversation, following how this " +
				"workspace replies to customers: its tone, languages and guidance, given with the open " +
				"conversation. It lands in the teammate's reply box for them to edit and send; it is never " +
				"sent by you.",
			Schema: schema(`{"type":"object","required":["body"],"properties":{"body":{"type":"string",` +
				`"description":"The whole reply in Markdown, written to the customer in their language and the ` +
				`workspace's tone."}}}`),
		}, handle: e.draftReply},
		{spec: contracts.ToolSpec{
			Name: "propose_changes", Description: proposeDescription, Schema: schema(proposeSchema),
		}, writes: changeKinds(), handle: e.proposeChanges, was: wasTitle()},
		{spec: contracts.ToolSpec{
			Name: "remember",
			Description: "Remember something the teammate asked you to keep in mind for every future " +
				"conversation with them, such as how they like answers written.",
			Schema: schema(`{"type":"object","required":["fact"],"properties":{"fact":{"type":"string",` +
				`"maxLength":1000}}}`),
		}, handle: e.remember},
	}...)
}

/*
changeKinds are the kinds propose_changes writes; workspace actions and
help articles are proposed through their own tools.
*/
func changeKinds() []Kind {
	return slices.DeleteFunc(Kinds(), func(k Kind) bool {
		return k == KindAction || k == KindArticleEdit || k == KindArticleNew
	})
}

/*
draftReply hands a reply draft to the teammate.
*/
func (e *Engine) draftReply(_ context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Body string `json:"body"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	body := strings.TrimSpace(in.Body)
	if body == "" || len(body) > maxDraft {
		return result{}, fmt.Errorf("%w: a draft is 1 to %d characters", errBadArguments, maxDraft)
	}
	if s.conversation == nil {
		return result{}, fmt.Errorf("%w: drafts are for a thread about a conversation", errBadArguments)
	}
	return result{
		text: "The draft is in the teammate's reply box, waiting for them.",
		step: Step{Tool: "draft_reply", Summary: "Drafted a reply", Draft: body},
	}, nil
}

/*
remember keeps a fact the teammate asked Operator to remember.
*/
func (e *Engine) remember(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Fact string `json:"fact"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	fact := strings.TrimSpace(in.Fact)
	if fact == "" || len([]rune(fact)) > maxMemory {
		return result{}, fmt.Errorf("%w: a memory is 1 to %d characters", errBadArguments, maxMemory)
	}
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).CreateOperatorMemory(ctx, models.CreateOperatorMemoryParams{
			WorkspaceID: s.m.Workspace.ID, MemberID: s.m.Member.ID, Body: fact, Now: e.Clock.Now(),
		})
		return err
	})
	if err != nil {
		return result{}, fmt.Errorf("remember: %w", err)
	}
	return result{text: "Remembered.", step: Step{Tool: "remember", Summary: "Remembered: " + truncate(fact, 80)}}, nil
}

/*
proposeChanges turns the model's operations into a changeset, applying
it at once when the settings allow and the run is clean.
*/
func (e *Engine) proposeChanges(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in proposal
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	return e.propose(ctx, s, in)
}

/*
already settles a proposal that needs no new changeset: a routine's
that reaches past one target or repeats a waiting suggestion, and a
teammate's change to the suggestion they have open, which is added to
it in place.
*/
func (e *Engine) already(ctx context.Context, s *session, in proposal, ops []Operation) (result, bool, error) {
	if s.routine == nil {
		return e.amend(ctx, s, in, ops)
	}
	if err := oneTarget(ops); err != nil {
		return result{}, false, err
	}
	text, ok, err := e.repeated(ctx, s, ops)
	return result{text: text, step: Step{Tool: "propose_changes", Summary: "Skipped a suggestion made before"}}, ok, err
}

/*
propose checks a proposal and stores it as a changeset, whichever tool
it came through.
*/
func (e *Engine) propose(ctx context.Context, s *session, in proposal) (result, error) {
	if err := in.check(); err != nil {
		return result{}, err
	}
	ops, err := e.operations(ctx, s, in.Operations)
	if err != nil {
		return result{}, err
	}
	if out, ok, err := e.already(ctx, s, in, ops); err != nil || ok {
		return out, err
	}
	if text, ok, err := e.duplicate(ctx, s, in.DedupKey); err != nil || ok {
		return result{text: text, step: Step{Tool: "propose_changes", Summary: "Skipped a suggestion made before"}}, err
	}
	auto := autoApplies(s.settings, ops) && !s.tainted && !s.background && s.routine == nil
	row, err := e.createChangeset(ctx, s, changeset{proposal: in, ops: ops, auto: auto})
	if err != nil {
		return result{}, err
	}
	s.proposed = append(s.proposed, row)
	public := models.PublicID(models.PrefixOperatorChangeset, row.ID)
	st := Step{Tool: "propose_changes", Summary: "Proposed: " + row.Title, Changeset: public}
	if !auto {
		return result{text: fmt.Sprintf("Changeset %s is waiting for the teammate to review %d change(s).%s",
			public, len(ops), notesOf(ops)), step: st}, nil
	}
	if _, err := e.Apply(ctx, s.m, row.ID); err != nil {
		return result{text: "The changes could not be applied: " + refusal(err), step: st}, nil
	}
	st.Summary = "Applied: " + row.Title
	return result{text: fmt.Sprintf("Applied changeset %s; the teammate can undo it.", public), step: st}, nil
}

/*
notesOf is what the operations were matched to when that differs from
what the model sent, for the model to read.
*/
func notesOf(ops []Operation) string {
	var b strings.Builder
	for _, op := range ops {
		if op.Note != "" {
			b.WriteString(" " + op.Note)
		}
	}
	return b.String()
}

/*
changeset is a checked proposal about to be stored: what the model
sent, the operations it resolved to, and whether it applies at once.
*/
type changeset struct {
	proposal
	ops  []Operation
	auto bool
}

/*
createChangeset stores a proposal, about the conversation every
operation shares when they share one.
*/
func (e *Engine) createChangeset(ctx context.Context, s *session, in changeset) (models.OperatorChangeset, error) {
	var row models.OperatorChangeset
	now := e.Clock.Now()
	params := models.CreateOperatorChangesetParams{
		WorkspaceID: s.m.Workspace.ID, ThreadID: s.threadRef(), MemberID: s.memberRef(),
		ConversationID: sharedConversation(in.ops), Title: truncate(strings.TrimSpace(in.Title), 200),
		Reason: truncate(strings.TrimSpace(in.Reason), 2000), Operations: encodeOperations(in.ops), Auto: in.auto,
		RoutineID: s.thread.RoutineID, RoutineRunID: s.thread.RoutineRunID, DedupKey: nilable(dedupKey(in.DedupKey)),
		Now: now,
	}
	if s.routine != nil {
		params.Preview = s.routine.dryRun
		params.ExpiresAt = new(now.Add(suggestionLife))
	}
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).CreateOperatorChangeset(ctx, params)
		return err
	})
	if err != nil {
		return row, fmt.Errorf("store changeset: %w", err)
	}
	return row, nil
}

/*
threadRef is the thread a run's changesets belong to, none for a run
Operator started on its own or a call from an outside agent.
*/
func (s *session) threadRef() *ids.UUID {
	if s.background || s.outside {
		return nil
	}
	return &s.thread.ID
}

/*
memberRef is who a run's changesets were proposed to, nobody for a run
Operator started on its own, which any teammate who sees the
conversation may review, or for a run on a routine's thread, whose
changesets belong to the routine.
*/
func (s *session) memberRef() *ids.UUID {
	if s.background || s.thread.RoutineID != nil {
		return nil
	}
	return &s.m.Member.ID
}
