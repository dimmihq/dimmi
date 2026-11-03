/*
|--------------------------------------------------------------------------
| List Audit Logs
|--------------------------------------------------------------------------
|
| Owners and admins read who changed what in their workspace, newest
| first, narrowed by event, actor, channel, subject and time. Programs
| that copy the log elsewhere read it oldest first and resume from where
| they stopped. Pages are cut on the time and id of the last entry, never
| counted, so a page stays fast however long the log grows.
|
*/

package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
readPermission is what reading a workspace's audit log takes.
*/
const readPermission = "audit_logs:read"

/*
Position is where a page of the log starts: just past the entry
written at At with the id ID.
*/
type Position struct {
	At time.Time `json:"t"`
	ID ids.UUID  `json:"i"`
}

/*
Filter narrows the log. Events lists the event names to keep, none
meaning all; a nil id or time leaves that narrowing out. CreatedAfter
is inclusive and CreatedBefore exclusive.
*/
type Filter struct {
	Events        []string
	ActorID       *ids.UUID
	ViaID         *ids.UUID
	SubjectID     *ids.UUID
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

/*
ListAuditLogsInput is who asks, through their membership, and which
page of the log they want: narrowed by Filter, oldest first when
Ascending, after the position From when it is set.
*/
type ListAuditLogsInput struct {
	Membership authz.Membership
	Filter     Filter
	Ascending  bool
	From       *Position
	Limit      int
}

/*
Page is one page of the log, and whether more follow.
*/
type Page struct {
	Entries []models.AuditLog
	More    bool
}

/*
ListAuditLogs lists the audit log of the caller's workspace.

Rules:
  - The caller needs audit_logs:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListAuditLogs struct {
	DB *database.DB
}

/*
Handle returns the page of entries.
*/
func (a *ListAuditLogs) Handle(ctx context.Context, in ListAuditLogsInput) (Page, error) {
	if !in.Membership.Can(readPermission) {
		return Page{}, exceptions.ErrForbidden
	}

	f := in.Filter
	params := models.ListAuditLogsParams{
		WorkspaceID: in.Membership.Workspace.ID, Events: f.Events, ActorID: f.ActorID, ViaID: f.ViaID,
		SubjectID: f.SubjectID, CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
		RowLimit: int64(in.Limit + 1),
	}

	if in.From != nil {
		params.CursorAt, params.CursorID = &in.From.At, &in.From.ID
	}

	var rows []models.AuditLog

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		q := models.New(tx)
		if in.Ascending {
			rows, err = q.ListAuditLogsAscending(ctx, models.ListAuditLogsAscendingParams(params))
		} else {
			rows, err = q.ListAuditLogs(ctx, params)
		}

		return err
	})
	if err != nil {
		return Page{}, fmt.Errorf("list audit logs: %w", err)
	}

	if len(rows) > in.Limit {
		return Page{Entries: rows[:in.Limit], More: true}, nil
	}

	return Page{Entries: rows}, nil
}
