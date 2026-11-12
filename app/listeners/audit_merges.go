/*
|--------------------------------------------------------------------------
| Audit Log: Contact Merges
|--------------------------------------------------------------------------
|
| Merging two contacts cannot be taken back, so each merge gets an entry
| naming who did it, the contact that survived and the one merged away,
| with how many conversations and identities moved.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
subjectContact is the subject of contact entries.
*/
const subjectContact = "contact"

/*
listenMerges registers the entries of contact merges.
*/
func (l *WriteAuditLog) listenMerges(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.contactMerged)
}

/*
contactMerged records a merge. The contacts are named by id only, since
their names are personal data the log keeps no copy of.
*/
func (l *WriteAuditLog) contactMerged(ctx context.Context, e appevents.ContactMerged) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventContactMerged, ActorID: e.Actor.ID,
		Subject: audit.Subject{
			Type: subjectContact, ID: e.ContactID, Label: models.PublicID(models.PrefixContact, e.ContactID),
		},
		Changes: map[string]any{
			"merged_contact_id": models.PublicID(models.PrefixContact, e.SourceID),
			"conversations":     e.Conversations, "identities": e.Identities,
		},
	})
}
