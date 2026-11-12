package imports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
writes runs fn in a transaction scoped to the context's workspace. Row
security lets the application role write only inside one, so every
change an import makes outside an applier goes through here.
*/
func writes(ctx context.Context, db *database.DB, fn func(q *models.Queries) error) error {
	return db.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
reads returns queries that each read in a transaction scoped to the
context's workspace, the only way row security shows the application
role its rows.
*/
func reads(ctx context.Context, db *database.DB) *models.Queries {
	return models.New(db.Reader(ctx))
}
