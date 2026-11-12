/*
|--------------------------------------------------------------------------
| Managing Imports
|--------------------------------------------------------------------------
|
| A teammate uploads a file, which is kept as a draft with its header,
| its first rows and the mapping its headers suggest. They map the
| columns, try the mapping in a dry run that counts what would be added,
| changed, skipped or refused, and then start it; from then on the
| import only runs, once. Importing takes contacts:write and
| imports:manage.
|
*/

package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listedImport is how many imports a list returns.
*/
const listedImport = 20

/*
The refusals of managing imports.
*/
var (
	ErrKind    = fault.New(fault.Invalid, "import_kind_invalid", "Choose contacts or companies.").OnField("kind")
	ErrStarted = fault.New(fault.Conflict, "import_started", "This import has already started.")
)

/*
CreateInput is who uploads which file for which kind of import.
*/
type CreateInput struct {
	Membership authz.Membership
	Kind       string
	FileName   string
	File       io.Reader
}

/*
Input names an import, with the plan for a dry run or a start.
*/
type Input struct {
	Membership authz.Membership
	ID         ids.UUID
	Plan       Plan
}

/*
Imports keeps and runs imports.
*/
type Imports struct {
	DB     *database.DB
	Clock  clock.Clock
	Queue  *queue.Queue
	Upsert *contacts.UpsertIdentifiedContact
}

/*
allowed refuses a caller who may not import.
*/
func allowed(m authz.Membership) error {
	if !m.Can("contacts:write") || !m.Can("imports:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
Create keeps the uploaded file as a draft.

Errors: [exceptions.ErrForbidden], [ErrKind], [ErrUnreadable],
[ErrEmpty], or a wrapped database error.
*/
func (a *Imports) Create(ctx context.Context, in CreateInput) (View, error) {
	if err := allowed(in.Membership); err != nil {
		return View{}, err
	}

	if in.Kind != KindContacts && in.Kind != KindCompanies {
		return View{}, ErrKind
	}

	data, err := io.ReadAll(io.LimitReader(in.File, MaxBytes+1))
	if err != nil {
		return View{}, ErrUnreadable
	}

	s, err := read(data)
	if err != nil {
		return View{}, err
	}

	columns, err := json.Marshal(s.columns)
	if err != nil {
		return View{}, fmt.Errorf("encode columns: %w", err)
	}

	q := models.New(a.DB)

	row, err := q.CreateImport(ctx, models.CreateImportParams{
		WorkspaceID: in.Membership.Workspace.ID, Kind: in.Kind, FileName: fileName(in.FileName), File: data,
		Columns: columns, RowCount: int32Of(len(s.rows)), CreatedByMemberID: &in.Membership.Member.ID,
		Now: a.Clock.Now(),
	})
	if err != nil {
		return View{}, fmt.Errorf("keep import: %w", err)
	}

	return describeImport(ctx, q, row)
}

/*
Show returns the import as it now stands.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
func (a *Imports) Show(ctx context.Context, in Input) (View, error) {
	if err := allowed(in.Membership); err != nil {
		return View{}, err
	}

	q := models.New(a.DB)

	row, err := find(ctx, q, in)
	if err != nil {
		return View{}, err
	}

	return describeImport(ctx, q, row)
}

/*
List returns the workspace's latest imports of the kind.

Errors: [exceptions.ErrForbidden], [ErrKind], or a wrapped database
error.
*/
func (a *Imports) List(ctx context.Context, m authz.Membership, kind string) ([]models.ListImportsRow, error) {
	if err := allowed(m); err != nil {
		return nil, err
	}

	if kind != KindContacts && kind != KindCompanies {
		return nil, ErrKind
	}

	rows, err := models.New(a.DB).ListImports(ctx, models.ListImportsParams{
		WorkspaceID: m.Workspace.ID, Kind: kind, RowLimit: listedImport,
	})
	if err != nil {
		return nil, fmt.Errorf("list imports: %w", err)
	}

	return rows, nil
}

/*
Predict runs the plan over every row without writing anything and
counts what each would come to.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], [ErrMapping],
[ErrMode], or a wrapped database error.
*/
func (a *Imports) Predict(ctx context.Context, in Input) (Prediction, error) {
	if err := allowed(in.Membership); err != nil {
		return Prediction{}, err
	}

	q := models.New(a.DB)

	row, err := find(ctx, q, in)
	if err != nil {
		return Prediction{}, err
	}

	s, b, err := prepare(ctx, q, row, in.Plan)
	if err != nil {
		return Prediction{}, err
	}

	ap := applier{m: in.Membership, kind: row.Kind, mode: in.Plan.Mode}
	out := Prediction{Rows: len(s.rows), Errors: []RowError{}}

	for _, l := range s.rows {
		r, problem := b.build(l)

		outcome := OutcomeFailed
		if problem == "" {
			if outcome, err = ap.predict(ctx, q, r); err != nil {
				return out, err
			}
		}

		out.count(outcome, l.number, problem)
	}

	return out, nil
}

/*
Start fixes the plan and queues the run.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], [ErrMapping],
[ErrMode], [ErrStarted], or a wrapped error.
*/
func (a *Imports) Start(ctx context.Context, in Input) (View, error) {
	if err := allowed(in.Membership); err != nil {
		return View{}, err
	}

	var out View

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := find(ctx, q, in)
		if err != nil {
			return err
		}

		if _, _, err := prepare(ctx, q, row, in.Plan); err != nil {
			return err
		}

		mapping, err := json.Marshal(in.Plan.Mapping)
		if err != nil {
			return fmt.Errorf("encode mapping: %w", err)
		}

		row, err = q.QueueImport(ctx, models.QueueImportParams{
			Mapping: mapping, Mode: in.Plan.Mode, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrStarted
		}

		if err != nil {
			return fmt.Errorf("queue import: %w", err)
		}

		if err := a.Queue.PushTx(ctx, tx, RunArgs{WorkspaceID: row.WorkspaceID, ImportID: row.ID}); err != nil {
			return fmt.Errorf("queue import run: %w", err)
		}

		out, err = describeImport(ctx, q, row)

		return err
	})

	return out, err
}

/*
find returns the import the input names in the caller's workspace.
*/
func find(ctx context.Context, q *models.Queries, in Input) (models.Import, error) {
	row, err := q.FindImport(ctx, models.FindImportParams{WorkspaceID: in.Membership.Workspace.ID, ID: in.ID})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find import: %w", err)
	}

	return row, nil
}

/*
prepare reads the import's file and checks the plan against it,
returning a builder for its rows.
*/
func prepare(ctx context.Context, q *models.Queries, row models.Import, p Plan) (sheet, builder, error) {
	s, err := read(row.File)
	if err != nil {
		return s, builder{}, err
	}

	if err := p.check(row.Kind, len(s.columns)); err != nil {
		return s, builder{}, err
	}

	defs, err := definitions(ctx, q, row)
	if err != nil {
		return s, builder{}, err
	}

	return s, newBuilder(row.Kind, p, defs, s.columns), nil
}

/*
definitions returns the custom attributes of the import's kind.
*/
func definitions(ctx context.Context, q *models.Queries, row models.Import) ([]models.AttributeDefinition, error) {
	object := attributedefs.ObjectContact
	if row.Kind == KindCompanies {
		object = attributedefs.ObjectCompany
	}

	defs, err := q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{
		WorkspaceID: row.WorkspaceID, Object: &object,
	})
	if err != nil {
		return nil, fmt.Errorf("list attribute definitions: %w", err)
	}

	return defs, nil
}

/*
fileName returns the name to keep for an uploaded file.
*/
func fileName(name string) string {
	if name == "" {
		return "import.csv"
	}

	return string([]rune(name)[:min(len([]rune(name)), 200)])
}
