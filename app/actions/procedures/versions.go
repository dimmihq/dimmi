/*
|--------------------------------------------------------------------------
| Procedure Versions
|--------------------------------------------------------------------------
|
| Every publish keeps the procedure as it was, numbered, with who
| published it and why, never to change. The team lists them with how
| each did, compares any two, and puts an old one back: as the working
| copy to look over first, or published at once. Restoring names the
| lock version the editor started from, like any save.
|
*/

package procedures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
ErrVersionNotFound means the procedure has no version of that number.
*/
var ErrVersionNotFound = fault.New(fault.NotFound, "procedure_version_not_found",
	"This procedure has no version with that number.")

/*
versionStatsDays is how far back each version's numbers reach.
*/
const versionStatsDays = 90

/*
Version is one published version with how it did: the conversations
that followed it in the last 90 days and how many were handed over.
*/
type Version struct {
	models.ProcedureVersion
	Conversations int32
	HandedOff     int32
}

/*
VersionsPage is a page of versions, newest first; Next is the version
number to list below for the next page, zero at the end.
*/
type VersionsPage struct {
	Items []Version
	Next  int32
}

/*
Versions lists a procedure's versions, newest first, below a version
number when one is given.
*/
func (p *Procedures) Versions(ctx context.Context, m authz.Membership, id ids.UUID, page Paging,
) (VersionsPage, error) {
	if !m.Can("ai_actions:read") {
		return VersionsPage{}, exceptions.ErrForbidden
	}

	limit := min(max(page.Limit, 1), 100)
	var out VersionsPage
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		rows, err := q.ListProcedureVersions(ctx, models.ListProcedureVersionsParams{
			WorkspaceID: m.Workspace.ID, ProcedureID: id, Before: page.Before, RowLimit: limit + 1,
		})
		if err != nil {
			return fmt.Errorf("list procedure versions: %w", err)
		}

		if len(rows) > int(limit) {
			rows = rows[:limit]
			out.Next = rows[len(rows)-1].Version
		}

		out.Items, err = p.withStats(ctx, q, procedureRef{ws: m.Workspace.ID, id: id}, rows)

		return err
	})

	return out, err
}

/*
Paging is a page of versions: below a version number when one is
given, at most Limit of them.
*/
type Paging struct {
	Before int32
	Limit  int32
}

/*
withStats adds to each version how it did in the last 90 days.
*/
func (p *Procedures) withStats(ctx context.Context, q *models.Queries, ref procedureRef,
	rows []models.ProcedureVersion,
) ([]Version, error) {
	stats, err := q.ProcedureVersionStats(ctx, models.ProcedureVersionStatsParams{
		WorkspaceID: ref.ws, ProcedureID: ref.id, Since: p.Clock.Now().Add(-versionStatsDays * 24 * time.Hour),
	})
	if err != nil {
		return nil, fmt.Errorf("count procedure versions: %w", err)
	}

	out := make([]Version, 0, len(rows))
	for _, r := range rows {
		v := Version{ProcedureVersion: r}
		for _, s := range stats {
			if s.Version == r.Version {
				v.Conversations, v.HandedOff = s.Conversations, s.HandedOff
			}
		}
		out = append(out, v)
	}

	return out, nil
}

/*
ShowVersion returns one version of a procedure.
*/
func (p *Procedures) ShowVersion(ctx context.Context, m authz.Membership, id ids.UUID, version int32,
) (models.ProcedureVersion, error) {
	if !m.Can("ai_actions:read") {
		return models.ProcedureVersion{}, exceptions.ErrForbidden
	}

	var out models.ProcedureVersion
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findVersion(ctx, models.New(tx), m.Workspace.ID, versionRef{id: id, version: version})

		return err
	})

	return out, err
}

/*
RestoreInput restores a version: the procedure, the version number,
whether to publish it at once, and the lock version the editor started
from.
*/
type RestoreInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	Version     int32
	Publish     bool
	LockVersion int32
}

/*
Restore puts an old version back as the working copy, and publishes it
when asked.
*/
func (p *Procedures) Restore(ctx context.Context, in RestoreInput) (models.Procedure, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		v, err := p.restorable(ctx, q, in)
		if err != nil {
			return err
		}

		out, err = q.RestoreProcedureDraft(ctx, models.RestoreProcedureDraftParams{
			Draft: v.Definition, MemberID: &m.Member.ID, Now: p.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: in.ID,
		})
		if err != nil {
			return wrap("restore procedure", err)
		}

		if err := p.audit(ctx, m, auditing{event: audit.EventProcedureRestored, proc: out, changes: map[string]any{
			"version": v.Version, "published": in.Publish,
		}}); err != nil {
			return err
		}

		if !in.Publish {
			return nil
		}

		out, err = p.republish(ctx, q, publishing{
			m: m, current: out, note: "Restored version " + strconv.Itoa(int(v.Version)),
		})

		return err
	})
	if err == nil && in.Publish {
		p.embed(ctx, out)
	}

	return out, err
}

/*
restorable is the version to restore, once the editor's copy is known
to be current and the version reads.
*/
func (p *Procedures) restorable(ctx context.Context, q *models.Queries, in RestoreInput,
) (models.ProcedureVersion, error) {
	ws := in.Membership.Workspace.ID
	current, err := find(ctx, q, ws, in.ID)
	if err != nil {
		return models.ProcedureVersion{}, err
	}

	if in.LockVersion != 0 && in.LockVersion != current.LockVersion {
		return models.ProcedureVersion{}, ErrEditConflict
	}

	v, err := findVersion(ctx, q, ws, versionRef{id: in.ID, version: in.Version})
	if err != nil {
		return v, err
	}

	if _, err := procedure.Parse(v.Definition); err != nil {
		return v, ErrInvalid.With("reason", err.Error())
	}

	return v, nil
}

/*
republish publishes a restored version at once, as a new version.
*/
func (p *Procedures) republish(ctx context.Context, q *models.Queries, pub publishing) (models.Procedure, error) {
	if err := p.publishable(ctx, q, pub.current); err != nil {
		return pub.current, err
	}

	return p.makeLive(ctx, q, pub)
}

/*
procedureRef names a procedure of a workspace.
*/
type procedureRef struct {
	ws, id ids.UUID
}

/*
versionRef names one version of a procedure by number.
*/
type versionRef struct {
	id      ids.UUID
	version int32
}

/*
findVersion returns one version of a procedure by number.
*/
func findVersion(ctx context.Context, q *models.Queries, ws ids.UUID, ref versionRef) (models.ProcedureVersion, error) {
	if _, err := find(ctx, q, ws, ref.id); err != nil {
		return models.ProcedureVersion{}, err
	}

	v, err := q.FindProcedureVersion(ctx, models.FindProcedureVersionParams{
		WorkspaceID: ws, ProcedureID: ref.id, Version: ref.version,
	})
	if errors.Is(err, database.ErrNotFound) {
		return v, ErrVersionNotFound
	}

	return v, wrap("find procedure version", err)
}

/*
vectorText writes a vector as pgvector reads it.
*/
func vectorText(v []float32) string {
	out, _ := json.Marshal(v)

	return string(out)
}
