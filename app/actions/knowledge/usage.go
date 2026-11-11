/*
|--------------------------------------------------------------------------
| Embedding Usage
|--------------------------------------------------------------------------
|
| Every embedding batch is added to the month's usage, charged to the
| knowledge sources whose chunks it embedded and, for the workspace's
| own articles and conversations, to no source. The provider's token
| count is shared between them by the length of what each sent; when
| the provider does not report tokens they are estimated from the text
| and marked estimated. The cost comes from the price table, and a
| model it does not know is marked unpriced rather than counted free,
| so the figures can be billed on without guessing.
|
*/

package knowledge

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
batchUsage is what one stored batch used: the workspace, the model, the
inputs of the hashes embedded, and the tokens the provider counted.
*/
type batchUsage struct {
	ws     ids.UUID
	model  ModelKey
	inputs map[string]string
	usage  contracts.EmbedUsage
}

/*
usageOf gathers the usage of a stored batch.
*/
func usageOf(ws ids.UUID, plan *batchPlan, out embedded) batchUsage {
	inputs := map[string]string{}
	for i, h := range plan.hashes {
		if _, ok := out.vectors[h]; ok {
			inputs[h] = plan.inputs[i]
		}
	}

	return batchUsage{ws: ws, model: plan.binding.model, inputs: inputs, usage: out.usage}
}

/*
recordEmbedUsage adds the batch to the month's usage of each source it
served, inside the transaction that stored its vectors.
*/
func (v *Vectors) recordEmbedUsage(ctx context.Context, q *models.Queries, b batchUsage, now time.Time) error {
	if len(b.inputs) == 0 {
		return nil
	}

	shares, total, err := sharesOf(ctx, q, b)
	if err != nil {
		return err
	}

	tokens, estimated := b.usage.InputTokens, int64(0)
	if !b.usage.Reported || tokens <= 0 {
		tokens = (total + 3) / 4
		estimated = tokens
	}

	keys := make([]ids.UUID, 0, len(shares))
	for k := range shares {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareIDs)

	left, leftEstimated := tokens, estimated
	for i, key := range keys {
		part, partEstimated := left, leftEstimated
		if i < len(keys)-1 {
			part, partEstimated = tokens*shares[key]/total, estimated*shares[key]/total
		}
		left, leftEstimated = left-part, leftEstimated-partEstimated

		slice := share{source: key, tokens: part, estimated: partEstimated, at: now}
		if err := v.addUsage(ctx, q, b, slice); err != nil {
			return err
		}
	}

	return nil
}

/*
sharesOf sums the length of what the batch sent per source, the
workspace's own content under the nil id, and the length of it all.
*/
func sharesOf(ctx context.Context, q *models.Queries, b batchUsage) (map[ids.UUID]int64, int64, error) {
	hashes := make([]string, 0, len(b.inputs))
	for h := range b.inputs {
		hashes = append(hashes, h)
	}

	rows, err := q.ListEmbedHashSources(ctx, models.ListEmbedHashSourcesParams{WorkspaceID: b.ws, Hashes: hashes})
	if err != nil {
		return nil, 0, fmt.Errorf("list embedded sources: %w", err)
	}

	sourceOf := map[string]ids.UUID{}
	for _, r := range rows {
		sourceOf[r.EmbedHash] = r.SourceID
	}

	shares, total := map[ids.UUID]int64{}, int64(0)
	for h, input := range b.inputs {
		n := max(int64(len(input)), 1)
		shares[sourceOf[h]] += n
		total += n
	}

	return shares, total, nil
}

/*
share is one source's part of a batch: its tokens, of them estimated,
and when the batch was stored.
*/
type share struct {
	source            ids.UUID
	tokens, estimated int64
	at                time.Time
}

/*
addUsage adds one source's part of the batch to its month.
*/
func (v *Vectors) addUsage(ctx context.Context, q *models.Queries, b batchUsage, s share) error {
	var sourceID *ids.UUID
	if s.source != ids.Nil() {
		sourceID = &s.source
	}
	tokens, estimated, now := s.tokens, s.estimated, s.at

	cost, unpriced := int64(0), true
	if v.Prices != nil {
		if c := v.Prices.Cost(b.model.Provider, b.model.Model, aiprices.TokenCounts{Input: tokens}); c != nil {
			cost, unpriced = *c, false
		}
	}

	err := q.AddKnowledgeUsage(ctx, models.AddKnowledgeUsageParams{
		WorkspaceID: b.ws, PeriodStart: database.Date(MonthStart(now)), SourceID: sourceID, Kind: "embedding",
		ModelKey: b.model.String(), InputTokens: tokens, EstimatedTokens: estimated, Requests: 1,
		CostMicros: cost, Unpriced: unpriced, Now: now,
	})
	if err != nil {
		return fmt.Errorf("add embedding usage: %w", err)
	}

	return nil
}

/*
MonthStart is the first day of the UTC month holding t, the key of a
month's usage.
*/
func MonthStart(t time.Time) time.Time {
	t = t.UTC()

	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func compareIDs(a, b ids.UUID) int {
	return slices.Compare(a[:], b[:])
}
