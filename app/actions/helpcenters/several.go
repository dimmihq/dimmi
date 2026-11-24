/*
|--------------------------------------------------------------------------
| Searching With Several Queries
|--------------------------------------------------------------------------
|
| A search may carry the question and up to two other ways of asking
| it. Each query is a search of its own: it finds its own passages, by
| meaning and by keywords, and the reranker orders them against that
| query's own words. The results then take turns, the question's best
| passage first, then each other query's best, then their seconds, and
| a passage several of them found is kept once, where it first came up.
|
| Ranking every passage against the question alone was tried and
| measured against this on a real help center. It lost the answer
| whenever the question was worded poorly and another query was worded
| well: the passages the good query found were then judged against the
| poor one and fell out. A passage is only fairly judged against the
| query that went looking for it.
|
| The queries are embedded together in one request first, so no search
| waits on a request of its own. The searches then run side by side,
| and the whole takes about as long as one.
|
*/

package helpcenters

import (
	"cmp"
	"context"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
searchEach runs one search per query and has their results take turns.
How the search went, its mode and why it fell back if it did, is the
question's; each wait is the longest any query had.
*/
func (k *Knowledge) searchEach(ctx context.Context, ws ids.UUID, asked []string, opts contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	one := opts
	one.Queries = nil
	started := k.Vectors.Clock.Now()
	warmed := k.Vectors.Warm(ctx, ws, asked, opts.QueryEmbedTimeout)
	embedded := k.Vectors.Clock.Now().Sub(started)

	results := make([]contracts.KnowledgeSearchResult, len(asked))
	errs := make([]error, len(asked))
	var wg sync.WaitGroup
	for i, query := range asked {
		wg.Go(func() {
			results[i], errs[i] = k.searchOne(ctx, ws, query, one)
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return contracts.KnowledgeSearchResult{}, err
		}
	}

	out := results[0]
	if out.Timings == nil {
		out.Timings = map[string]time.Duration{}
	}
	lists := make([][]contracts.KnowledgeChunk, len(results))
	for i, res := range results {
		lists[i] = res.Chunks
		for part, took := range res.Timings {
			out.Timings[part] = max(out.Timings[part], took)
		}
	}
	out.Chunks = takeTurns(cmp.Or(max(opts.Limit, 0), defaultLimit), lists)
	if warmed.Usage != nil {
		out.EmbeddingUsage, out.EmbeddingModel, out.EmbeddingKeyID = warmed.Usage, warmed.Model, warmed.KeyID
		out.Timings["embed_query"] = embedded
	}
	return out, nil
}

/*
takeTurns merges ranked lists by taking the first of each, then the
second of each, and so on, up to the limit. A passage, or another with
exactly its text, is kept once, where it first came up.
*/
func takeTurns(limit int, lists [][]contracts.KnowledgeChunk) []contracts.KnowledgeChunk {
	out := make([]contracts.KnowledgeChunk, 0, limit)
	seen := map[ids.UUID]bool{}
	said := map[string]bool{}
	longest := 0
	for _, list := range lists {
		longest = max(longest, len(list))
	}
	for i := range longest {
		for _, list := range lists {
			if i >= len(list) || len(out) >= limit {
				continue
			}
			ch := list[i]
			text := knowledge.SameText(ch.Text)
			if seen[ch.ID] || said[text] {
				continue
			}
			seen[ch.ID], said[text] = true, true
			ch.Rank = len(out) + 1
			out = append(out, ch)
		}
	}
	return out
}
