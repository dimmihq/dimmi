/*
|--------------------------------------------------------------------------
| Try Redaction
|--------------------------------------------------------------------------
|
| A teammate checks what the workspace's rules would make of some text
| before relying on them. Nothing is stored and the text is never logged:
| it could be the very thing the rules exist to keep out.
|
*/

package redaction

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
TryRedactionInput is who tries which text.
*/
type TryRedactionInput struct {
	Membership authz.Membership
	Text       string
}

/*
Tried is the text as it would be stored and the rules that changed it.
*/
type Tried struct {
	Text string
	Hits []redaction.Hit
}

/*
TryRedaction applies the workspace's enabled rules to the text.

Rules:
  - The caller needs settings:write and redaction:manage.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type TryRedaction struct {
	DB *database.DB
}

/*
Handle returns what the rules make of the text.
*/
func (a *TryRedaction) Handle(ctx context.Context, in TryRedactionInput) (Tried, error) {
	if !canManage(in.Membership) {
		return Tried{}, exceptions.ErrForbidden
	}

	var out Tried

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		r, err := Load(ctx, models.New(tx), in.Membership.Workspace.ID)
		if err != nil {
			return err
		}

		out.Text, out.Hits = r.Apply(in.Text)

		return nil
	})

	return out, err
}
