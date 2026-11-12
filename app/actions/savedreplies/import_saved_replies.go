/*
|--------------------------------------------------------------------------
| Import Saved Replies
|--------------------------------------------------------------------------
|
| An import adds many replies at once from rows a spreadsheet gave, all
| or none.
|
*/

package savedreplies

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxImported is the most rows one import adds.
*/
const MaxImported = 500

/*
ImportRow is one reply of an import.
*/
type ImportRow struct {
	Title    string
	Body     string
	Shortcut *string
	Shared   bool
}

/*
ImportSavedRepliesInput is who imports which rows.
*/
type ImportSavedRepliesInput struct {
	Membership authz.Membership
	Rows       []ImportRow
}

/*
ImportSavedReplies adds many replies at once, all or none.

Rules:
  - Each row follows the rules of [CreateSavedReply]; the first refusal
    stops the import and adds nothing.

Errors: those of [CreateSavedReply].
*/
type ImportSavedReplies struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle adds the rows' replies and returns them in row order.
*/
func (a *ImportSavedReplies) Handle(ctx context.Context, in ImportSavedRepliesInput) ([]models.SavedReply, error) {
	out := make([]models.SavedReply, 0, len(in.Rows))

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		for _, row := range in.Rows {
			shared := row.Shared
			created, err := create(ctx, q, CreateSavedReplyInput{
				Membership: in.Membership,
				Fields:     Fields{Title: &row.Title, Body: &row.Body, Shortcut: row.Shortcut, Shared: &shared},
			}, a.Clock.Now())
			if err != nil {
				return err
			}

			out = append(out, created)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
