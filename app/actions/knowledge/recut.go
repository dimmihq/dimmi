/*
|--------------------------------------------------------------------------
| Cutting Stored Articles Again
|--------------------------------------------------------------------------
|
| A published version's chunks are stored once and kept, so a change to
| how articles are cut reaches only what is published after it. Recut
| brings the rest along: it drops a workspace's stored article chunks
| and indexes its live articles afresh, which queues the vectors the
| new chunks lack. Search keeps answering meanwhile from keywords, and
| by meaning again as the vectors arrive. The vectors of the old chunks
| are left for the sweep.
|
| A source's documents are cut again each time the source syncs, so
| they need nothing here.
|
*/

package knowledge

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Recut cuts the workspace's live articles again and queues the vectors
the new chunks lack. It returns how many chunks the workspace has
afterwards.
*/
func (v *Vectors) Recut(ctx context.Context, ws ids.UUID) (int, error) {
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, _ *models.Queries) error {
		_, err := tx.Exec(ctx, "DELETE FROM knowledge_chunks WHERE workspace_id = $1", ws)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("drop stored chunks: %w", err)
	}
	if err := v.indexUnindexed(ctx, ws); err != nil {
		return 0, err
	}
	var n int32
	err = v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) (err error) {
		n, err = q.CountKnowledgeChunks(ctx, ws)
		return err
	})
	return int(n), err
}
