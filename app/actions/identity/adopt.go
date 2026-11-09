/*
|--------------------------------------------------------------------------
| Adopt Visitor
|--------------------------------------------------------------------------
|
| A help center pass names a contact the workspace already has, so there
| is nothing to find by id and nothing to write over them: the messenger
| signs in as that contact. An anonymous lead the visitor was chatting as
| first is folded into them, its conversations moving over, so a reader
| who signs in mid-conversation carries on where they were.
|
*/

package identity

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Adoption is a sign-in as one of the workspace's contacts: the contact,
and the anonymous lead the visitor was before, nil when there was none.
*/
type Adoption struct {
	WorkspaceID ids.UUID
	Contact     ids.UUID
	Lead        *models.Contact
	Now         time.Time
}

/*
Adopt returns the contact the adoption names, the one they were merged
into if they were, after folding the visitor's lead into them.
*/
func Adopt(ctx context.Context, tx *database.Tx, dispatcher *events.Dispatcher, a Adoption) (Identified, error) {
	q := models.New(tx)
	known, err := contacts.Survivor(ctx, q, a.WorkspaceID, a.Contact)
	if err != nil {
		return Identified{}, err
	}
	lead := a.Lead
	if lead == nil || lead.ID == known.ID || lead.Kind != string(enums.ContactKindLead) {
		return Identified{Contact: known}, nil
	}
	run := &identifyRun{q: q, tx: tx, v: Visitor{WorkspaceID: a.WorkspaceID, Now: a.Now}, dispatcher: dispatcher}
	ended, err := run.absorb(ctx, known, *lead)
	return Identified{Contact: known, Ended: ended}, err
}
