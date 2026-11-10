/*
|--------------------------------------------------------------------------
| Erasing A Customer's Chat App Deliveries
|--------------------------------------------------------------------------
|
| Deliveries from chat apps are kept as the provider sent them, so they
| name the customer and carry what they wrote until the prune takes
| them. When the customer is erased, every kept delivery of the
| workspace that carries one of their chat identities, received before
| the request was made, goes too. The identities come from the request's
| sealed stash, since the contact lost them when the request was made;
| a delivery after that belongs to whoever wrote since. A Slack identity
| is kept as team and user, while Slack's payloads name the user on its
| own, so the user id is what is looked for. Erasing again finds
| nothing left.
|
*/

package erasures

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
eraseDeliveries deletes the kept deliveries that name the request's
customer on a chat app, returning how many went.
*/
func (a *EraseContact) eraseDeliveries(ctx context.Context, row models.ContactErasure) (int64, error) {
	stash, err := openStash(ctx, a.Keys, row)
	if err != nil {
		return 0, err
	}

	users := slackUsers(stash)
	if len(users) == 0 {
		return 0, nil
	}

	var erased int64

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		erased, err = models.New(tx).EraseChannelInboundEventsMentioning(ctx,
			models.EraseChannelInboundEventsMentioningParams{
				WorkspaceID: row.WorkspaceID, Provider: string(enums.ChannelTypeSlack),
				Before: row.CreatedAt, Needles: users,
			})

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("erase channel deliveries: %w", err)
	}

	return erased, nil
}

/*
slackUsers returns the Slack user ids of the Slack identities, which
are kept as the team and the user joined by a colon.
*/
func slackUsers(identities []stashedIdentity) []string {
	var users []string

	for _, identity := range identities {
		if identity.Type != string(enums.IdentityTypeSlackUser) {
			continue
		}

		_, user, found := strings.Cut(identity.Value, ":")
		if !found {
			user = identity.Value
		}

		if user != "" {
			users = append(users, user)
		}
	}

	return users
}

/*
openStash returns the identities sealed in the request, none when it
holds no stash.
*/
func openStash(ctx context.Context, keys encryption.KeyManager, row models.ContactErasure) ([]stashedIdentity, error) {
	if len(row.IdentitiesStash) == 0 {
		return nil, nil
	}

	plain, err := keys.Decrypt(ctx, row.IdentitiesStash, stashData(row.ContactID))
	if err != nil {
		return nil, fmt.Errorf("open identities: %w", err)
	}

	var stash []stashedIdentity
	if err := json.Unmarshal(plain, &stash); err != nil {
		return nil, fmt.Errorf("decode identities: %w", err)
	}

	return stash, nil
}
