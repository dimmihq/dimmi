/*
|--------------------------------------------------------------------------
| Rank Fusion
|--------------------------------------------------------------------------
|
| Keyword and vector scores live on different scales, so a hybrid
| search fuses ranks, not scores: each retriever adds 1/(60 + rank)
| for every chunk it found, the reciprocal rank fusion that rewards a
| chunk both found. A fused chunk keeps both judgments: its cosine
| similarity when the vector arm found it, and its keyword coverage
| when keywords did. Ties go by chunk id, so a result never reorders
| between runs, and no document fills the result alone.
|
*/

package knowledge

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
rrfK damps the weight of the top ranks.
*/
const rrfK = 60

/*
fused is one chunk with its fused score.
*/
type fused struct {
	chunk contracts.KnowledgeChunk
	score float64
}

/*
Fuse merges ranked lists into one, best first, at most perDocument
chunks of one document and limit in all.
*/
func Fuse(limit, perDocument int, lists ...[]contracts.KnowledgeChunk) []contracts.KnowledgeChunk {
	byID := map[ids.UUID]*fused{}
	var order []*fused
	for _, list := range lists {
		for i, ch := range list {
			f, ok := byID[ch.ID]
			if !ok {
				f = &fused{chunk: ch}
				f.chunk.MatchedBy = nil
				byID[ch.ID] = f
				order = append(order, f)
			}
			f.score += 1 / float64(rrfK+i+1)
			merge(&f.chunk, ch)
		}
	}
	slices.SortStableFunc(order, func(a, b *fused) int {
		if a.score != b.score {
			return compare(b.score, a.score)
		}
		return strings.Compare(a.chunk.ID.String(), b.chunk.ID.String())
	})
	perDoc := map[ids.UUID]int{}
	out := make([]contracts.KnowledgeChunk, 0, min(limit, len(order)))
	for _, f := range order {
		if len(out) >= limit || (perDocument > 0 && perDoc[f.chunk.DocumentID] >= perDocument) {
			continue
		}
		perDoc[f.chunk.DocumentID]++
		f.chunk.Score, f.chunk.Rank = f.score, len(out)+1
		out = append(out, f.chunk)
	}
	return out
}

/*
merge folds one retriever's view of a chunk into the fused chunk: the
vector arm's similarity wins, keyword coverage is kept, and the
retrievers that found it add up.
*/
func merge(into *contracts.KnowledgeChunk, ch contracts.KnowledgeChunk) {
	if slices.Contains(ch.MatchedBy, "vector") {
		into.Similarity = ch.Similarity
	}
	if ch.Coverage != nil {
		into.Coverage = ch.Coverage
		if !slices.Contains(into.MatchedBy, "vector") {
			into.Similarity = ch.Coverage
		}
	}
	for _, m := range ch.MatchedBy {
		if !slices.Contains(into.MatchedBy, m) {
			into.MatchedBy = append(into.MatchedBy, m)
		}
	}
}
