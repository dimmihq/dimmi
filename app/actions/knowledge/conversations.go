/*
|--------------------------------------------------------------------------
| Conversation Memory
|--------------------------------------------------------------------------
|
| A closed conversation is remembered as one text: its subject, the
| customer's first message and the team's last reply, which is what
| tells the next teammate how it was solved. The text is searchable by
| keywords at once and by meaning once its vector, embedded with the
| workspace's models like any chunk, is stored. Closing it again after
| a reopen remembers the new ending. Similar conversations fuse both
| rankings; they are for members only and never reach a customer.
|
*/

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
similarCandidates is how many conversations each arm ranks.
*/
const similarCandidates = 20

/*
SimilarConversation is one closed conversation like another, with
its fused score.
*/
type SimilarConversation struct {
	ID       ids.UUID
	Number   int64
	Subject  string
	Excerpt  string
	ClosedAt time.Time
	Score    float64
}

/*
EmbedConversation is the worker of jobs.EmbedConversationArgs.
*/
type EmbedConversation struct {
	Vectors *Vectors
}

/*
Handle remembers the closed conversation and embeds it.
*/
func (w *EmbedConversation) Handle(ctx context.Context, job *queue.Job[jobs.EmbedConversationArgs]) error {
	return w.Vectors.RememberConversation(ctx, job.Args.WorkspaceID, job.Args.ConversationID)
}

/*
RememberConversation stores the closed conversation's text and queues
its vector for every model in use. A conversation open again, or
gone, is left alone.
*/
func (v *Vectors) RememberConversation(ctx context.Context, ws, conversationID ids.UUID) error {
	return v.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		c, err := q.FindConversationForEmbedding(ctx, models.FindConversationForEmbeddingParams{
			WorkspaceID: ws, ID: conversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find conversation to remember: %w", err)
		}
		return v.storeConversation(ctx, tx, c)
	})
}

/*
storeConversation stores the conversation's text and queues its vector
for every model in use; a conversation with no text is skipped.
*/
func (v *Vectors) storeConversation(ctx context.Context, tx *database.Tx, c models.FindConversationForEmbeddingRow,
) error {
	q, ws := models.New(tx), c.WorkspaceID
	body := conversationText(c)
	if body == "" {
		return nil
	}
	hash, now := EmbedHash(body), v.Clock.Now()
	closed := now
	if c.ClosedAt != nil {
		closed = *c.ClosedAt
	}
	if err := q.UpsertKnowledgeConversation(ctx, models.UpsertKnowledgeConversationParams{
		ConversationID: c.ID, WorkspaceID: ws, Body: body, EmbedHash: hash, ClosedAt: closed, Now: now,
	}); err != nil {
		return fmt.Errorf("remember conversation: %w", err)
	}
	s, found, err := settings(ctx, q, ws)
	if err != nil || !found {
		return err
	}
	for _, model := range embeddingModels(s) {
		args := jobs.EmbedKnowledgeChunksArgs{WorkspaceID: ws, ModelKey: model}
		if err := v.pushBatches(ctx, tx, args, []string{hash}); err != nil {
			return err
		}
	}
	return nil
}

/*
conversationText joins the parts a conversation is remembered by.
*/
func conversationText(c models.FindConversationForEmbeddingRow) string {
	var parts []string
	if c.Subject != nil {
		parts = append(parts, strings.TrimSpace(*c.Subject))
	}
	parts = append(parts, strings.TrimSpace(c.FirstMessage), strings.TrimSpace(c.LastReply))
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n")
}

/*
SimilarConversations returns the closed conversations most like the
remembered one, by keywords and by meaning, best first.

Rules:
  - The caller needs conversations:read.
  - Only conversations the caller may see are compared or returned.
  - A conversation not remembered yet has no similar ones.
*/
func (v *Vectors) SimilarConversations(ctx context.Context, m authz.Membership, conversationID ids.UUID, limit int,
) ([]SimilarConversation, error) {
	if !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}
	ws, scope := m.Workspace.ID, m.ConversationScope()
	var lists [][]SimilarConversation
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		conv, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conversationID})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !scope.Contains(conv)) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find conversation: %w", err)
		}
		self, err := q.FindKnowledgeConversation(ctx, models.FindKnowledgeConversationParams{
			WorkspaceID: ws, ConversationID: conversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find remembered conversation: %w", err)
		}
		lists, err = v.similarLists(ctx, q, self, scope.Params())
		return err
	})
	if err != nil {
		return nil, err
	}
	return fuseConversations(limit, lists...), nil
}

/*
similarLists ranks the other conversations by the words and by the
vector of the remembered one.
*/
func (v *Vectors) similarLists(ctx context.Context, q *models.Queries, self models.KnowledgeConversation,
	access authz.VisibleParams,
) ([][]SimilarConversation, error) {
	ws := self.WorkspaceID
	var lists [][]SimilarConversation
	if terms := orTerms(self.Body); terms != "" {
		rows, err := q.SearchKnowledgeConversationsFTS(ctx, models.SearchKnowledgeConversationsFTSParams{
			WorkspaceID: ws, ExcludeID: self.ConversationID, Query: terms, RowLimit: similarCandidates,
			AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
			AccessUnassigned: access.Unassigned, AccessHiddenInboxes: access.HiddenInboxes,
		})
		if err != nil {
			return nil, fmt.Errorf("search conversations by words: %w", err)
		}
		var list []SimilarConversation
		for _, r := range rows {
			list = append(list, similar(remembered{
				id: r.ConversationID, number: r.Number, subject: r.Subject, body: r.Body, closed: r.ClosedAt,
			}))
		}
		lists = append(lists, list)
	}
	vector, err := v.conversationVectors(ctx, q, self, access)
	return append(lists, vector), err
}

/*
conversationVectors ranks the other conversations by the cosine
distance of their vectors of the active model to the remembered one's.
*/
func (v *Vectors) conversationVectors(ctx context.Context, q *models.Queries, self models.KnowledgeConversation,
	access authz.VisibleParams,
) ([]SimilarConversation, error) {
	ws := self.WorkspaceID
	s, found, err := settings(ctx, q, ws)
	if err != nil || !found || s.ActiveModelKey == nil {
		return nil, err
	}
	rows, err := q.SearchKnowledgeConversationsVectorBySelf(ctx, models.SearchKnowledgeConversationsVectorBySelfParams{
		WorkspaceID: ws, ModelKey: *s.ActiveModelKey, ConversationID: self.ConversationID, RowLimit: similarCandidates,
		AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
		AccessUnassigned: access.Unassigned, AccessHiddenInboxes: access.HiddenInboxes,
	})
	if err != nil {
		return nil, fmt.Errorf("search conversations by meaning: %w", err)
	}
	var list []SimilarConversation
	for _, r := range rows {
		list = append(list, similar(remembered{
			id: r.ConversationID, number: r.Number, subject: r.Subject, body: r.Body, closed: r.ClosedAt,
		}))
	}
	return list, nil
}

/*
remembered is one remembered conversation a search found.
*/
type remembered struct {
	id      ids.UUID
	number  int64
	subject *string
	body    string
	closed  time.Time
}

/*
similar builds one result, its excerpt the start of its text.
*/
func similar(c remembered) SimilarConversation {
	out := SimilarConversation{ID: c.id, Number: c.number, Excerpt: c.body, ClosedAt: c.closed}
	if c.subject != nil {
		out.Subject = *c.subject
	}
	if r := []rune(c.body); len(r) > 200 {
		out.Excerpt = string(r[:200]) + "…"
	}
	return out
}

/*
fuseConversations fuses the rankings by reciprocal rank.
*/
func fuseConversations(limit int, lists ...[]SimilarConversation) []SimilarConversation {
	scores := map[ids.UUID]*SimilarConversation{}
	var order []*SimilarConversation
	for _, list := range lists {
		for i, c := range list {
			f, ok := scores[c.ID]
			if !ok {
				f = &c
				scores[c.ID], order = f, append(order, f)
			}
			f.Score += 1 / float64(rrfK+i+1)
		}
	}
	out := make([]SimilarConversation, 0, len(order))
	for _, f := range order {
		out = append(out, *f)
	}
	sortSimilar(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

/*
sortSimilar orders by fused score, ties by id.
*/
func sortSimilar(list []SimilarConversation) {
	slices.SortStableFunc(list, func(a, b SimilarConversation) int {
		if a.Score != b.Score {
			return compare(b.Score, a.Score)
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
}

/*
orTerms is a query matching any of the text's first distinct words.
*/
func orTerms(text string) string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens(text) {
		if !seen[t] && len(out) < 16 {
			seen[t] = true
			out = append(out, t)
		}
	}
	return strings.Join(out, " | ")
}
