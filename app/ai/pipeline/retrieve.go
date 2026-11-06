/*
|--------------------------------------------------------------------------
| Retrieve And Answerability
|--------------------------------------------------------------------------
|
| The agent answers only from the workspace's knowledge. Retrieval asks
| the index with the customer's own words — their messages since the
| last reply, plus their previous message when they follow up on it —
| over public and agent-only content; then only chunks whose
| similarity clears the agent's minimum are kept. A chunk with no
| similarity at all never passes. With nothing left the agent does not
| guess: the run goes straight to the honest no-answer.
|
*/

package pipeline

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Retrieval limits: chunks asked for, chunks kept, and the default
minimum similarity.
*/
const (
	searchLimit       = 8
	keepChunks        = 6
	defaultSimilarity = 0.35
)

/*
source is one kept chunk and the number the answer cites it by.
*/
type source struct {
	n     int
	chunk contracts.KnowledgeChunk
}

/*
retrieve searches the index and keeps the chunks that clear the
minimum similarity. A failed search keeps nothing.
*/
func (r *Runner) retrieve(ctx context.Context, st *run) {
	e := st.env
	query := retrievalQuery(e, st.c)
	st.query = query

	started := r.Clock.Now()
	res, err := r.Knowledge.Search(ctx, e.Workspace.ID, query, contracts.KnowledgeSearchOptions{
		Limit: searchLimit, Surface: "agent",
		Visibility: []contracts.KnowledgeVisibility{
			contracts.KnowledgeVisibilityPublic, contracts.KnowledgeVisibilityAgentOnly,
		},
	})

	threshold := minSimilarity(e)
	for _, ch := range res.Chunks {
		if err != nil || len(st.sources) >= keepChunks {
			break
		}
		if ch.Similarity == nil || *ch.Similarity < threshold {
			continue
		}
		st.sources = append(st.sources, source{n: len(st.sources) + 1, chunk: ch})
	}
	st.retrieval = retrievalJSON(st.sources)

	request, _ := json.Marshal(map[string]any{"query": query})
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindRetrieve, started: started, err: err,
		request: r.redactFor(e, string(request)), response: string(st.retrieval),
		usage: embeddingUsage(res),
	})
}

/*
retrievalQuery is the customer's pending messages, led by their
previous message when they follow up on it.
*/
func retrievalQuery(e *env, c classification) string {
	query := e.pendingText()
	if c.Intent != "followup" {
		return query
	}
	for i := len(e.History) - 1; i >= 0; i-- {
		if e.History[i].AuthorType == string(enums.AuthorTypeContact) {
			return joinParagraphs([]string{body(e.History[i]), query})
		}
	}
	return query
}

/*
answerability sends a run with no usable source to the no-answer, or
to a person when the customer also asked for one.
*/
func (r *Runner) answerability(st *run) *decision {
	if len(st.sources) > 0 {
		return nil
	}
	if st.c.WantsHuman {
		return handoff("customer_requested", "", st.env.handoffKey())
	}
	return unknown("")
}

/*
minSimilarity is the agent's minimum similarity, or the default when
unset.
*/
func minSimilarity(e *env) float64 {
	v, err := e.Agent.MinSimilarity.Float64Value()
	if err != nil || !v.Valid {
		return defaultSimilarity
	}
	return v.Float64
}

/*
embeddingUsage is the query embedding's usage, counted on the step.
*/
func embeddingUsage(res contracts.KnowledgeSearchResult) contracts.LLMUsage {
	if res.EmbeddingUsage == nil {
		return contracts.LLMUsage{Reported: true}
	}
	u := *res.EmbeddingUsage
	u.Reported = true
	return u
}
