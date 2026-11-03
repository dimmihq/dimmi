/*
|--------------------------------------------------------------------------
| Keyset Store
|--------------------------------------------------------------------------
|
| The key manager keeps its keysets here, one row per owner and purpose,
| each wrapped by the key encryption key. Two servers may race to create
| or rotate the same keyset: creating never overwrites a row another
| server wrote first, and replacing only swaps the keyset the caller
| read, so the loser retries with the winner's keys instead of
| discarding them along with every value they sealed.
|
*/

package keysets

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Store is the keyset store on the encryption_keys table.
*/
type Store struct {
	DB *database.DB
}

var _ encryption.KeysetStore = (*Store)(nil)

/*
Find returns the wrapped keyset, or encryption.ErrKeysetNotFound.
*/
func (s *Store) Find(ctx context.Context, ref encryption.KeysetRef) ([]byte, error) {
	wrapped, err := models.New(s.DB).FindEncryptionKeyset(ctx, models.FindEncryptionKeysetParams{
		ScopeKind: ref.Scope.Kind, ScopeID: ref.Scope.ID, Purpose: string(ref.Purpose),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, encryption.ErrKeysetNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find keyset: %w", err)
	}

	return wrapped, nil
}

/*
Create stores a new keyset, or returns encryption.ErrKeysetExists when
another server stored one first.
*/
func (s *Store) Create(ctx context.Context, ref encryption.KeysetRef, wrapped []byte) error {
	created, err := models.New(s.DB).CreateEncryptionKeyset(ctx, models.CreateEncryptionKeysetParams{
		ScopeKind: ref.Scope.Kind, ScopeID: ref.Scope.ID, Purpose: string(ref.Purpose), Wrapped: wrapped,
	})
	if err != nil {
		return fmt.Errorf("create keyset: %w", err)
	}

	if created == 0 {
		return encryption.ErrKeysetExists
	}

	return nil
}

/*
Replace swaps the keyset while it still equals previous, and returns
encryption.ErrKeysetChanged otherwise.
*/
func (s *Store) Replace(ctx context.Context, ref encryption.KeysetRef, previous, next []byte) error {
	replaced, err := models.New(s.DB).ReplaceEncryptionKeyset(ctx, models.ReplaceEncryptionKeysetParams{
		ScopeKind: ref.Scope.Kind, ScopeID: ref.Scope.ID, Purpose: string(ref.Purpose),
		Previous: previous, Next: next,
	})
	if err != nil {
		return fmt.Errorf("replace keyset: %w", err)
	}

	if replaced == 0 {
		return encryption.ErrKeysetChanged
	}

	return nil
}

/*
All yields every stored keyset's reference.
*/
func (s *Store) All(ctx context.Context) iter.Seq2[encryption.KeysetRef, error] {
	return func(yield func(encryption.KeysetRef, error) bool) {
		rows, err := models.New(s.DB).ListEncryptionKeysets(ctx)
		if err != nil {
			yield(encryption.KeysetRef{}, fmt.Errorf("list keysets: %w", err))

			return
		}

		for _, row := range rows {
			ref := encryption.KeysetRef{
				Scope: encryption.Scope{Kind: row.ScopeKind, ID: row.ScopeID}, Purpose: encryption.Purpose(row.Purpose),
			}
			if !yield(ref, nil) {
				return
			}
		}
	}
}
