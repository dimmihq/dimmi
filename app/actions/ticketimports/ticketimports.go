/*
|--------------------------------------------------------------------------
| Importing Tickets
|--------------------------------------------------------------------------
|
| An import reads the other tool's setup first and maps it, reusing a
| type, state or field the workspace already has under the same name,
| then brings the tickets in one transaction each, so one bad ticket is
| reported and the rest still land. Trackers are linked last, once
| every ticket is in. The import keeps its counts, the tickets it could
| not bring in with why, and every record it touched.
|
*/

package ticketimports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxErrors is how many failed tickets an import names.
*/
const maxErrors = 200

/*
Importer brings tickets in from other tools.
*/
type Importer struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	Queue   *queue.Queue
	Tickets *tickets.Tickets
}

/*
Move is the ticket entering a state at a time.
*/
type Move struct {
	At      time.Time
	StateID string
}

/*
RowError is a ticket the import could not bring in, and why.
*/
type RowError struct {
	ExternalID string `json:"external_id"`
	Reason     string `json:"reason"`
}

/*
canManage refuses a member who may not set tickets up.
*/
func canManage(m authz.Membership) error {
	if !m.Can("tickets:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
Import brings the source's tickets into the member's workspace and
returns the finished import. A failure reading the source fails the
import with why; it is returned as well.
*/
func (i *Importer) Import(ctx context.Context, m authz.Membership, src Source) (models.TicketImport, error) {
	if err := canManage(m); err != nil {
		return models.TicketImport{}, err
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	row, err := models.New(i.DB).CreateTicketImport(ctx, models.CreateTicketImportParams{
		WorkspaceID: m.Workspace.ID, Source: src.Name(), CreatedByMemberID: &m.Member.ID, Now: i.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("create ticket import: %w", err)
	}

	r := &run{importer: i, row: row, src: src, made: map[string]madeTicket{}}

	runErr := r.all(ctx)

	return r.finish(ctx, runErr)
}

/*
run is one import under way.
*/
type run struct {
	importer *Importer
	row      models.TicketImport
	src      Source
	mapped   mapping
	made     map[string]madeTicket
	pending  []RowError
}

/*
madeTicket is a ticket the run brought in or found, and the tracker it
reports.
*/
type madeTicket struct {
	ticket  models.Ticket
	tracker string
}

/*
all maps the setup, brings each ticket in, then links trackers.
*/
func (r *run) all(ctx context.Context) error {
	catalog, err := r.src.Catalog(ctx)
	if err != nil {
		return err
	}

	if err := r.mapSetup(ctx, catalog); err != nil {
		return err
	}

	err = r.src.Tickets(ctx, func(t Ticket) error {
		return r.one(ctx, t)
	})
	if err != nil {
		return err
	}

	return r.linkTrackers(ctx)
}

/*
one brings a ticket in, keeping a refusal as the ticket's error and
stopping on anything else.
*/
func (r *run) one(ctx context.Context, t Ticket) error {
	made, created, err := r.ticket(ctx, t)

	if f, ok := errors.AsType[*fault.Error](err); ok {
		r.fail(t.ExternalID, f.Message())

		return r.progress(ctx, counts{failed: 1})
	}

	if err != nil {
		return err
	}

	r.made[t.ExternalID] = madeTicket{ticket: made, tracker: t.TrackerID}

	if created {
		return r.progress(ctx, counts{tickets: 1})
	}

	return r.progress(ctx, counts{found: 1})
}

/*
fail keeps why a ticket could not be brought in.
*/
func (r *run) fail(external, reason string) {
	r.pending = append(r.pending, RowError{ExternalID: external, Reason: reason})
}

/*
counts is what a step of the import came to.
*/
type counts struct {
	types, fields, states, tickets, found, failed, links int
}

/*
progress adds the counts and any kept errors to the import.
*/
func (r *run) progress(ctx context.Context, c counts) error {
	errs := r.pending
	if over := int(r.row.TicketsFailed) + len(errs) - maxErrors; over > 0 {
		errs = errs[:max(len(errs)-over, 0)]
	}

	raw, err := json.Marshal(append([]RowError{}, errs...))
	if err != nil {
		return fmt.Errorf("encode import errors: %w", err)
	}

	r.pending = nil
	r.row.TicketsFailed += int32(c.failed) //nolint:gosec // one step counts a handful

	err = models.New(r.importer.DB).RecordTicketImportProgress(ctx, models.RecordTicketImportProgressParams{
		TypesCreated: int32(c.types), FieldsCreated: int32(c.fields), //nolint:gosec // small counts
		StatesCreated: int32(c.states), TicketsCreated: int32(c.tickets), //nolint:gosec // small counts
		TicketsFound: int32(c.found), TicketsFailed: int32(c.failed), //nolint:gosec // small counts
		LinksCreated: int32(c.links), RowErrors: raw, Now: r.importer.Clock.Now(), //nolint:gosec // small counts
		WorkspaceID: r.row.WorkspaceID, ID: r.row.ID,
	})
	if err != nil {
		return fmt.Errorf("record ticket import progress: %w", err)
	}

	return nil
}

/*
finish ends the import, failed with why when the run stopped.
*/
func (r *run) finish(ctx context.Context, runErr error) (models.TicketImport, error) {
	status := "completed"

	var reason *string

	if runErr != nil {
		status = "failed"
		message := "The import stopped unexpectedly."

		if f, ok := errors.AsType[*fault.Error](runErr); ok {
			message = f.Message()
		}

		reason = &message
	}

	row, err := models.New(r.importer.DB).FinishTicketImport(ctx, models.FinishTicketImportParams{
		Status: status, Error: reason, Now: r.importer.Clock.Now(), WorkspaceID: r.row.WorkspaceID, ID: r.row.ID,
	})
	if err != nil {
		return row, errors.Join(runErr, fmt.Errorf("finish ticket import: %w", err))
	}

	return row, runErr
}

/*
touched is one record the import made or found, under its id in the
other tool.
*/
type touched struct {
	kind, external string
	id             ids.UUID
	created        bool
}

/*
record keeps one record the import made or found.
*/
func (r *run) record(ctx context.Context, q *models.Queries, t touched) error {
	err := q.RecordTicketImportRecord(ctx, models.RecordTicketImportRecordParams{
		WorkspaceID: r.row.WorkspaceID, ImportID: r.row.ID, RecordKind: t.kind, ExternalID: cut(t.external, 255),
		RecordID: t.id, Created: t.created, Now: r.importer.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record imported %s: %w", t.kind, err)
	}

	return nil
}

/*
cut shortens text to at most n characters.
*/
func cut(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}

	return string(runes[:n])
}
