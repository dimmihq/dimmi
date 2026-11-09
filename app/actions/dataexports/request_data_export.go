/*
|--------------------------------------------------------------------------
| Data Exports
|--------------------------------------------------------------------------
|
| A member takes conversations or customers out of the workspace as a
| file: transcripts, a spreadsheet or the data itself. Each format has a
| ceiling on how many rows it covers, checked when the export is asked
| for so the member hears at once rather than after a long wait. Notes
| stay out unless the member may moderate the team's conversations, and
| the file is built in the background.
|
*/

package dataexports

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of export.
*/
const (
	KindConversations = "conversations"
	KindContacts      = "contacts"
)

/*
countCap is how far the rows of an export are counted to tell the
member how many matched.
*/
const countCap = 1_000_000

/*
limits holds, for each kind, the formats it comes in and how many rows
each may cover.
*/
func limits() map[string]map[string]int {
	return map[string]map[string]int{
		KindConversations: {"txt": 1_000, "csv": 50_000, "json": 10_000},
		KindContacts:      {"csv": 50_000, "json": 10_000},
	}
}

/*
permissionOf returns the permission an export of the kind needs.
*/
func permissionOf(kind string) string {
	if kind == KindContacts {
		return "contacts:export"
	}

	return "conversations:export"
}

/*
BuildArgs names the export a job builds.
*/
type BuildArgs = jobs.BuildDataExportArgs

/*
RequestInput is who asks for which export. Conversations is read for a
conversations export and Contacts for a contacts export.
*/
type RequestInput struct {
	Membership    authz.Membership
	Kind          string
	Format        string
	Conversations conversations.Selection
	Contacts      ContactSelection
	IncludeNotes  bool
}

/*
RequestDataExport starts building an export.

Rules:
  - The caller needs conversations:export or contacts:export for the
    kind, and conversations:moderate to include notes.
  - The kind comes only in its formats.
  - The rows the caller may see are counted now; more than the format
    allows is refused with how many matched.

Errors: [exceptions.ErrForbidden], [exceptions.ErrExportFormat],
[exceptions.ErrExportTooLarge], or a wrapped error.
*/
type RequestDataExport struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Queue    *queue.Queue
	Selector conversations.Selector
}

/*
Handle records the export and queues its build.
*/
func (a *RequestDataExport) Handle(ctx context.Context, in RequestInput) (models.DataExport, error) {
	limit, err := allowed(in)
	if err != nil {
		return models.DataExport{}, err
	}

	var out models.DataExport

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		matched, err := a.count(ctx, tx, in)
		if err != nil {
			return err
		}

		if matched > limit {
			return exceptions.ErrExportTooLarge.With("matched", matched).With("limit", limit)
		}

		out, err = a.record(ctx, tx, in)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.DataExportRequested{
			Base:     base(in.Membership, a.Clock),
			ExportID: out.ID, Kind: in.Kind, Format: in.Format, Matched: matched,
		})
	})

	return out, err
}

/*
allowed checks the caller may take the export and returns how many rows
its format covers.
*/
func allowed(in RequestInput) (int, error) {
	if !in.Membership.Can(permissionOf(in.Kind)) {
		return 0, exceptions.ErrForbidden
	}

	if in.IncludeNotes && (in.Kind != KindConversations || !in.Membership.Can("conversations:moderate")) {
		return 0, exceptions.ErrForbidden
	}

	limit, ok := limits()[in.Kind][in.Format]
	if !ok {
		return 0, exceptions.ErrExportFormat
	}

	return limit, nil
}

/*
count returns how many rows the export covers as the caller sees them.
*/
func (a *RequestDataExport) count(ctx context.Context, tx *database.Tx, in RequestInput) (int, error) {
	if in.Kind == KindContacts {
		return countContacts(ctx, models.New(tx), contactRead{
			ws: in.Membership.Workspace.ID, sel: in.Contacts, now: a.Clock.Now(),
		})
	}

	return a.Selector.Count(ctx, tx, conversations.Reading{
		Membership: in.Membership, Selection: in.Conversations, Limit: countCap,
	})
}

/*
record stores the export with its selection and queues its build.
*/
func (a *RequestDataExport) record(ctx context.Context, tx *database.Tx, in RequestInput) (models.DataExport, error) {
	var selection any = in.Conversations
	if in.Kind == KindContacts {
		selection = in.Contacts
	}

	raw, err := json.Marshal(selection)
	if err != nil {
		return models.DataExport{}, fmt.Errorf("encode selection: %w", err)
	}

	ws := in.Membership.Workspace.ID

	row, err := models.New(tx).CreateDataExport(ctx, models.CreateDataExportParams{
		WorkspaceID: ws, RequestedByMemberID: &in.Membership.Member.ID, Kind: in.Kind, Format: in.Format,
		Selection: raw, IncludeNotes: in.IncludeNotes, Now: a.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("record export: %w", err)
	}

	if err := a.Queue.PushTx(ctx, tx, BuildArgs{WorkspaceID: ws, ExportID: row.ID}); err != nil {
		return row, fmt.Errorf("queue export: %w", err)
	}

	return row, nil
}

/*
base stamps an event the member caused.
*/
func base(m authz.Membership, c clock.Clock) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: c.Now(),
		Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
	}
}
