/*
|--------------------------------------------------------------------------
| Audit Log: Erasures
|--------------------------------------------------------------------------
|
| A privacy request is something a workspace may have to prove it
| answered, so each step is kept in the audit log: asked for, held by a
| retention hold, withdrawn, and done with what it removed. The customer
| is named by id only, since their name is what is being erased.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
subjectErasure is the subject of erasure entries.
*/
const subjectErasure = "contact_erasure"

/*
listenErasures registers the entries of erasure requests.
*/
func (l *WriteAuditLog) listenErasures(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.erasureRequested)
	events.Listen(d, events.InTx, l.erasureCanceled)
	events.Listen(d, events.InTx, l.erasureHeld)
	events.Listen(d, events.InTx, l.contactErased)
}

/*
erasureSubject names the request and the customer it is about.
*/
func erasureSubject(erasure, contact ids.UUID) audit.Subject {
	return audit.Subject{
		Type: subjectErasure, ID: erasure, Label: models.PublicID(models.PrefixContact, contact),
	}
}

/*
erasureRequested records a request.
*/
func (l *WriteAuditLog) erasureRequested(ctx context.Context, e appevents.ContactErasureRequested) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventContactErasureRequested, ActorID: e.Actor.ID,
		Subject: erasureSubject(e.ErasureID, e.ContactID), Changes: map[string]any{"immediate": e.Immediate},
	})
}

/*
erasureCanceled records a request withdrawn.
*/
func (l *WriteAuditLog) erasureCanceled(ctx context.Context, e appevents.ContactErasureCanceled) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventContactErasureCanceled, ActorID: e.Actor.ID,
		Subject: erasureSubject(e.ErasureID, e.ContactID),
	})
}

/*
erasureHeld records a request a retention hold keeps.
*/
func (l *WriteAuditLog) erasureHeld(ctx context.Context, e appevents.ContactErasureHeld) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventContactErasureHeld, ActorID: ids.Nil(),
		Subject: erasureSubject(e.ErasureID, e.ContactID), Changes: map[string]any{"status": "pending_hold"},
	})
}

/*
contactErased records a request done, with what it removed.
*/
func (l *WriteAuditLog) contactErased(ctx context.Context, e appevents.ContactErased) error {
	changes := map[string]any{"status": "completed"}
	for key, count := range e.Counts {
		changes[key] = count
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventContactErasureCompleted, ActorID: ids.Nil(),
		Subject: erasureSubject(e.ErasureID, e.ContactID), Changes: changes,
	})
}
