/*
|--------------------------------------------------------------------------
| Provider Key Test
|--------------------------------------------------------------------------
|
| The on-demand revalidation: it calls the provider's models
| endpoint through the key's driver and stores the fresh status.
|
*/

package ai

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TestKeyInput names the key to revalidate.
*/
type TestKeyInput struct {
	Membership authz.Membership
	KeyID      ids.UUID
}

/*
TestProviderKey revalidates a connection on demand and returns its
fresh status.

Rules:
  - The caller needs ai:keys.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type TestProviderKey struct {
	DB    *database.DB
	Clock clock.Clock
	LLM   *managers.LLMManager
}

/*
Handle tests and returns the key.
*/
func (a *TestProviderKey) Handle(ctx context.Context, in TestKeyInput) (*models.AiProviderKey, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}

	var key models.AiProviderKey

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		key, err = findKey(ctx, tx, in.Membership.Workspace.ID, in.KeyID)
		if err != nil {
			return err
		}

		key, err = keyValidator{llm: a.LLM, clock: a.Clock}.recordValidation(ctx, tx, key)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &key, nil
}
