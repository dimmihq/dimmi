/*
|--------------------------------------------------------------------------
| Matching Known People
|--------------------------------------------------------------------------
|
| Journeys ask about people they already hold: does this person, or
| anyone in this account, match the rules right now. The answer uses the
| same rules and the same facts as every other audience, read inside the
| caller's transaction.
|
*/

package audiences

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Matching returns the ids of the contacts that match the saved audience,
or the rules when none is named. No audience and no rules match
everyone.
*/
func (a *Audiences) Matching(
	ctx context.Context, q *models.Queries, in MatchInput, contacts []models.Contact,
) ([]ids.UUID, error) {
	if len(contacts) == 0 {
		return nil, nil
	}

	program, err := a.programOf(ctx, q, in)
	if err != nil {
		return nil, err
	}

	if program == nil {
		out := make([]ids.UUID, len(contacts))
		for i, c := range contacts {
			out[i] = c.ID
		}

		return out, nil
	}

	env, err := a.env(ctx, q, in.WorkspaceID)
	if err != nil {
		return nil, err
	}

	return judge{program: program, env: env}.matching(ctx, q, contacts)
}
