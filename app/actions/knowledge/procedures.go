/*
|--------------------------------------------------------------------------
| Procedure Vectors
|--------------------------------------------------------------------------
|
| A workspace with many procedures offers dimmi only the few closest to
| what the customer asks. Each procedure's description is embedded when
| it is published, with the workspace's active model, and the customer's
| words with the same model when the procedures are ranked, so the two
| always compare. A workspace without embeddings has neither, and the
| procedures are ranked by the words they share instead.
|
*/

package knowledge

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
procedureEmbedTimeout bounds embedding one procedure's description.
*/
const procedureEmbedTimeout = 10 * time.Second

/*
EmbedDocument embeds a procedure's description with the workspace's
active model, returning the vector and the model's key.
*/
func (v *Vectors) EmbedDocument(ctx context.Context, ws ids.UUID, text string) ([]float32, string, error) {
	var drv contracts.Embeddings
	var model ModelKey
	var key string
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, found, err := settings(ctx, q, ws)
		if err != nil {
			return err
		}
		if !found || s.ActiveModelKey == nil {
			return ErrNoEmbeddings
		}
		if model, err = ParseModelKey(*s.ActiveModelKey); err != nil {
			return ErrNoEmbeddings
		}
		key = *s.ActiveModelKey
		drv, err = v.driverFor(ctx, q, ws, binding{model: model, keyID: s.EmbeddingKeyID})
		return err
	})
	if err != nil {
		return nil, "", err
	}
	embedCtx, cancel := context.WithTimeout(ctx, procedureEmbedTimeout)
	defer cancel()
	resp, err := drv.Embed(embedCtx, contracts.EmbedRequest{
		Model: model.Model, Inputs: []string{text}, Dimensions: int(model.Dims),
		InputType: contracts.EmbedInputDocument,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Vectors[0], key, nil
}

/*
EmbedQuery embeds a customer's words with the workspace's active model,
from the cache when they were asked before.
*/
func (v *Vectors) EmbedQuery(ctx context.Context, ws ids.UUID, text string) ([]float32, string, error) {
	out, err := v.queryVector(ctx, ws, text, v.queryTimeout(0))
	if err != nil {
		return nil, "", err
	}
	return out.vector, out.model, nil
}
