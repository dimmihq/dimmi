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
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "draft_reply",
			Description: "Draft a reply to the customer of this thread's conversation. It lands in the " +
				"teammate's reply box for them to edit and send; it is never sent by you.",
			Schema: schema(`{"type":"object","required":["body"],"properties":{"body":{"type":"string",` +
				`"description":"The reply in Markdown, written to the customer."}}}`),
		}, handle: e.draftReply},
		{spec: contracts.ToolSpec{
			Name: "propose_changes", Description: proposeDescription, Schema: schema(proposeSchema),
		}, writes: changeKinds(), handle: e.proposeChanges},
		{spec: contracts.ToolSpec{
			Name: "remember",
			Description: "Remember something the teammate asked you to keep in mind for every future " +
				"conversation with them, such as how they like answers written.",
			Schema: schema(`{"type":"object","required":["fact"],"properties":{"fact":{"type":"string",` +
				`"maxLength":1000}}}`),
		}, handle: e.remember},
	}
}

/*
changeKinds are the kinds propose_changes writes; workspace actions are
proposed through their own tools.
*/
func changeKinds() []Kind {
	return slices.DeleteFunc(Kinds(), func(k Kind) bool { return k == KindAction })
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
	if err := in.check(); err != nil {
		return result{}, err
	}
	ops, err := e.operations(ctx, s, in.Operations)
	if err != nil {
		return result{}, err
	}
	auto := autoApplies(s.settings, ops) && !s.tainted && !s.background
	row, err := e.createChangeset(ctx, s, changeset{proposal: in, ops: ops, auto: auto})
	if err != nil {
		return result{}, err
	}
	s.proposed = append(s.proposed, row)
	public := models.PublicID(models.PrefixOperatorChangeset, row.ID)
	st := Step{Tool: "propose_changes", Summary: "Proposed: " + row.Title, Changeset: public}
	if !auto {
		return result{text: fmt.Sprintf("Changeset %s is waiting for the teammate to review %d change(s).",
			public, len(ops)), step: st}, nil
	}
	if _, err := e.Apply(ctx, s.m, row.ID); err != nil {
		return result{text: "The changes could not be applied: " + refusal(err), step: st}, nil
	}
	st.Summary = "Applied: " + row.Title
	return result{text: fmt.Sprintf("Applied changeset %s; the teammate can undo it.", public), step: st}, nil
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
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).CreateOperatorChangeset(ctx, models.CreateOperatorChangesetParams{
			WorkspaceID: s.m.Workspace.ID, ThreadID: s.threadRef(), MemberID: s.memberRef(),
			ConversationID: sharedConversation(in.ops), Title: truncate(strings.TrimSpace(in.Title), 200),
			Reason: truncate(strings.TrimSpace(in.Reason), 2000), Operations: encodeOperations(in.ops), Auto: in.auto,
			Now: e.Clock.Now(),
		})
		return err
	})
	if err != nil {
		return row, fmt.Errorf("store changeset: %w", err)
	}
	return row, nil
}

/*
threadRef is the thread a run's changesets belong to, none for a run
Operator started on its own.
*/
func (s *session) threadRef() *ids.UUID {
	if s.background {
		return nil
	}
	return &s.thread.ID
}

/*
memberRef is who a run's changesets were proposed to, nobody for a run
Operator started on its own, which any teammate who sees the
conversation may review.
*/
func (s *session) memberRef() *ids.UUID {
	if s.background {
		return nil
	}
	return &s.m.Member.ID
}
