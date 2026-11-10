/*
|--------------------------------------------------------------------------
| Embedding Batches
|--------------------------------------------------------------------------
|
| A batch embeds up to 64 hashes with one model, skipping the ones
| already stored, so a repeated or overlapping batch costs nothing. A
| batch for a model the workspace no longer embeds with is dropped. The
| provider is called outside any transaction. An input the provider
| refuses is remembered, after the batch is retried one input at a time
| so one bad input never blocks the rest; any other failure is retried
| by the queue, and a failing key marks the workspace degraded. After
| each batch the pending model's progress is counted, and the model is
| switched in once its coverage is complete; batches finishing at once
| may each try, and only the first switch changes anything.
|
*/

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
switchShare is the coverage a model may switch at when every input
left out was refused by the provider.
*/
const switchShare = 0.995

/*
batchPlan is one batch ready to embed: its binding, driver, and the
missing hashes with their inputs in the same order.
*/
type batchPlan struct {
	binding binding
	driver  contracts.Embeddings
	hashes  []string
	inputs  []string
}

/*
embedded is a batch's outcome: a vector per stored hash and the hashes
the provider refused.
*/
type embedded struct {
	vectors map[string][]float32
	refused map[string]string
}

/*
EmbedKnowledgeChunks is the worker of jobs.EmbedKnowledgeChunksArgs.
*/
type EmbedKnowledgeChunks struct {
	Vectors *Vectors
}

/*
Handle embeds the batch.
*/
func (w *EmbedKnowledgeChunks) Handle(ctx context.Context, job *queue.Job[jobs.EmbedKnowledgeChunksArgs]) error {
	return w.Vectors.EmbedBatch(ctx, job.Args)
}

/*
EmbedBatch embeds the batch's missing hashes with its model and
records the pending model's progress.
*/
func (v *Vectors) EmbedBatch(ctx context.Context, args jobs.EmbedKnowledgeChunksArgs) error {
	plan, err := v.planBatch(ctx, args)
	if errors.Is(err, ErrKeyRemoved) {
		v.markDegraded(ctx, args.WorkspaceID, "key_removed")
		return queue.Cancel(err)
	}
	if err != nil {
		return err
	}
	if plan != nil {
		out, err := v.embedPlan(ctx, plan)
		if err != nil {
			v.noteProviderFailure(ctx, args.WorkspaceID, err)
			return fmt.Errorf("embed knowledge batch: %w", err)
		}
		if err := v.storeBatch(ctx, args, plan.binding.model.Dims, out); err != nil {
			return err
		}
	}
	return v.progress(ctx, args.WorkspaceID, args.ModelKey)
}

/*
planBatch reads what the batch still needs, nil when the model is out
of use or nothing is missing.
*/
func (v *Vectors) planBatch(ctx context.Context, args jobs.EmbedKnowledgeChunksArgs) (*batchPlan, error) {
	var plan *batchPlan
	ws := args.WorkspaceID
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		b, ok, err := bindingFor(ctx, q, ws, args.ModelKey)
		if err != nil || !ok {
			return err
		}
		done, err := q.ListEmbeddedHashes(ctx, models.ListEmbeddedHashesParams{
			WorkspaceID: ws, ModelKey: args.ModelKey, Hashes: args.Hashes,
		})
		if err != nil {
			return fmt.Errorf("list embedded hashes: %w", err)
		}
		hashes, inputs, err := embedInputs(ctx, q, ws, without(args.Hashes, done))
		if err != nil || len(hashes) == 0 {
			return err
		}
		drv, err := v.driverFor(ctx, q, ws, b)
		if err != nil {
			return err
		}
		plan = &batchPlan{binding: b, driver: drv, hashes: hashes, inputs: inputs}
		return nil
	})
	return plan, err
}

/*
bindingFor is the binding the workspace embeds the model with; ok is
false when it no longer embeds with that model.
*/
func bindingFor(ctx context.Context, q *models.Queries, ws ids.UUID, model string) (binding, bool, error) {
	s, found, err := settings(ctx, q, ws)
	if err != nil || !found || !slices.Contains(embeddingModels(s), model) {
		return binding{}, false, err
	}
	key, ok := modelKeyOf(model)
	if !ok {
		return binding{}, false, nil
	}
	b := binding{model: key, keyID: s.PendingKeyID}
	if s.ActiveModelKey != nil && *s.ActiveModelKey == model {
		b.keyID = s.EmbeddingKeyID
	}
	return b, true, nil
}

/*
embedInputs returns the inputs of the hashes that still exist, chunks
and conversations alike, in matching order.
*/
func embedInputs(ctx context.Context, q *models.Queries, ws ids.UUID, hashes []string,
) ([]string, []string, error) {
	if len(hashes) == 0 {
		return nil, nil, nil
	}
	chunks, err := q.ListChunkEmbedInputs(ctx, models.ListChunkEmbedInputsParams{WorkspaceID: ws, Hashes: hashes})
	if err != nil {
		return nil, nil, fmt.Errorf("list chunk inputs: %w", err)
	}
	convs, err := q.ListConversationEmbedInputs(ctx, models.ListConversationEmbedInputsParams{
		WorkspaceID: ws, Hashes: hashes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list conversation inputs: %w", err)
	}
	found := map[string]string{}
	for _, c := range chunks {
		found[c.EmbedHash] = EmbedInput(c.Title, c.HeadingPath, c.Body)
	}
	for _, c := range convs {
		found[c.EmbedHash] = c.Body
	}
	var outHashes, outInputs []string
	for _, h := range hashes {
		if in, ok := found[h]; ok {
			outHashes, outInputs = append(outHashes, h), append(outInputs, in)
		}
	}
	return outHashes, outInputs, nil
}

/*
embedPlan calls the provider for the whole batch, and one input at a
time when it refuses the batch.
*/
func (v *Vectors) embedPlan(ctx context.Context, plan *batchPlan) (embedded, error) {
	out := embedded{vectors: map[string][]float32{}, refused: map[string]string{}}
	vectors, err := v.embedDocuments(ctx, plan, plan.inputs)
	if err == nil {
		for i, h := range plan.hashes {
			out.vectors[h] = vectors[i]
		}
		return out, nil
	}
	if !refused(err) {
		return out, err
	}
	for i, h := range plan.hashes {
		vectors, err := v.embedDocuments(ctx, plan, plan.inputs[i:i+1])
		switch {
		case err == nil:
			out.vectors[h] = vectors[0]
		case refused(err):
			out.refused[h] = err.Error()
		default:
			return out, err
		}
	}
	return out, nil
}

/*
embedDocuments embeds document inputs with the plan's model.
*/
func (v *Vectors) embedDocuments(ctx context.Context, plan *batchPlan, inputs []string) ([][]float32, error) {
	resp, err := plan.driver.Embed(ctx, contracts.EmbedRequest{
		Model: plan.binding.model.Model, Inputs: inputs, Dimensions: int(plan.binding.model.Dims),
		InputType: contracts.EmbedInputDocument,
	})
	if err != nil {
		return nil, err
	}
	return resp.Vectors, nil
}

/*
storeBatch stores the vectors and remembers the refused inputs.
*/
func (v *Vectors) storeBatch(ctx context.Context, args jobs.EmbedKnowledgeChunksArgs, dims int32,
	out embedded,
) error {
	ws, now := args.WorkspaceID, v.Clock.Now()
	return v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var stored []string
		for h, vec := range out.vectors {
			if err := q.InsertKnowledgeEmbedding(ctx, models.InsertKnowledgeEmbeddingParams{
				WorkspaceID: ws, ModelKey: args.ModelKey, EmbedHash: h, Dims: dims,
				Embedding: database.NewHalfVector(vec), Now: now,
			}); err != nil {
				return fmt.Errorf("store embedding: %w", err)
			}
			stored = append(stored, h)
		}
		if err := q.ClearEmbeddingFailures(ctx, models.ClearEmbeddingFailuresParams{
			WorkspaceID: ws, ModelKey: args.ModelKey, Hashes: stored,
		}); err != nil {
			return fmt.Errorf("clear embedding failures: %w", err)
		}
		for h, reason := range out.refused {
			if err := q.RecordEmbeddingFailure(ctx, models.RecordEmbeddingFailureParams{
				WorkspaceID: ws, ModelKey: args.ModelKey, EmbedHash: h, LastError: reason, Now: now,
			}); err != nil {
				return fmt.Errorf("record embedding failure: %w", err)
			}
		}
		return nil
	})
}

/*
progress counts the pending model's coverage and switches it in once
complete; nothing happens for any other model.
*/
func (v *Vectors) progress(ctx context.Context, ws ids.UUID, model string) error {
	return v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, found, err := settings(ctx, q, ws)
		if err != nil || !found || s.PendingModelKey == nil || *s.PendingModelKey != model {
			return err
		}
		cov, err := q.CountEmbeddingCoverage(ctx, models.CountEmbeddingCoverageParams{WorkspaceID: ws, ModelKey: model})
		if err != nil {
			return fmt.Errorf("count embedding coverage: %w", err)
		}
		now := v.Clock.Now()
		if err := q.UpdateReembedProgress(ctx, models.UpdateReembedProgressParams{
			Total: cov.Total, Done: cov.Embedded, Failed: cov.Failed, Now: now, WorkspaceID: ws, ModelKey: model,
		}); err != nil {
			return fmt.Errorf("record reembed progress: %w", err)
		}
		if !complete(cov) {
			return nil
		}
		if err := q.SwitchEmbeddingModel(ctx, models.SwitchEmbeddingModelParams{
			Now: now, WorkspaceID: ws, ModelKey: model,
		}); err != nil {
			return fmt.Errorf("switch embedding model: %w", err)
		}
		return nil
	})
}

/*
complete reports whether a model covers every chunk, or all but a
sliver the provider refused.
*/
func complete(c models.CountEmbeddingCoverageRow) bool {
	if c.Embedded >= c.Total {
		return true
	}
	return c.Embedded+c.Failed >= c.Total && float64(c.Embedded) >= switchShare*float64(c.Total)
}

/*
refused reports whether the provider refused the input itself, which
no retry changes.
*/
func refused(err error) bool {
	var e *contracts.LLMError
	return errors.As(err, &e) && (e.Kind == contracts.LLMErrorInvalidRequest ||
		e.Kind == contracts.LLMErrorContentFiltered)
}

/*
noteProviderFailure marks the workspace degraded when its key no
longer works.
*/
func (v *Vectors) noteProviderFailure(ctx context.Context, ws ids.UUID, err error) {
	var e *contracts.LLMError
	if !errors.As(err, &e) {
		return
	}
	broken := []contracts.LLMErrorKind{
		contracts.LLMErrorAuth, contracts.LLMErrorPermission, contracts.LLMErrorQuota, contracts.LLMErrorModelNotFound,
	}
	if slices.Contains(broken, e.Kind) {
		v.markDegraded(ctx, ws, string(e.Kind))
	}
}

/*
markDegraded records why the workspace's embedder stopped working.
Search falls back to keywords on its own, so a failure to record is
dropped.
*/
func (v *Vectors) markDegraded(ctx context.Context, ws ids.UUID, reason string) {
	_ = v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		return q.SetEmbeddingsStatus(ctx, models.SetEmbeddingsStatusParams{
			EmbeddingsStatus: "degraded", EmbeddingsError: &reason, Now: v.Clock.Now(), WorkspaceID: ws,
		})
	})
}

/*
without returns the hashes not in done.
*/
func without(hashes, done []string) []string {
	out := make([]string, 0, len(hashes))
	for _, h := range hashes {
		if !slices.Contains(done, h) && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}
