/*
|--------------------------------------------------------------------------
| Shared Helpers
|--------------------------------------------------------------------------
|
| Small pieces the email actions share. Tenant rows are only visible
| inside a transaction scoped to their workspace, so even a single read
| or write runs in one.
|
*/

package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
within runs the function with queries in a transaction of the
context's workspace.
*/
func within(ctx context.Context, db *database.DB, fn func(ctx context.Context, q *models.Queries) error) error {
	return db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
inWorkspace scopes the context to the workspace, so transactions see
its rows.
*/
func inWorkspace(ctx context.Context, workspaceID ids.UUID) context.Context {
	return tenancy.With(ctx, workspaceID.String())
}

/*
optional returns a pointer to the text, nil when it is empty.
*/
func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

/*
lower returns the address lowercased and trimmed.
*/
func lower(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

/*
bytesReader reads the bytes.
*/
func bytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}

/*
jsonOf encodes a value stored as JSON.
*/
func jsonOf(v any) (json.RawMessage, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode %T: %w", v, err)
	}

	return encoded, nil
}

/*
addressesOf returns the lowercased addresses of a list.
*/
func addressesOf(list []mail.Address) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		if a.Email != "" {
			out = append(out, lower(a.Email))
		}
	}

	return out
}

/*
readVerdicts decodes the provider verdicts kept on a row; verdicts that
do not decode are none at all.
*/
func readVerdicts(raw json.RawMessage) mailbox.Verdicts {
	var verdicts mailbox.Verdicts
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &verdicts)
	}

	return verdicts
}
