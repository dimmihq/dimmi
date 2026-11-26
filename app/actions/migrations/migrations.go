/*
|--------------------------------------------------------------------------
| Migrations
|--------------------------------------------------------------------------
|
| A migration is started for one account of another tool, named by the
| client: the tool's short name and the account's id there. Starting
| again for the same account finds the one already under way, so a
| client that lost its place picks it up. Its progress is read from its
| ledger: how many records of each kind were applied, wait for a
| reference, failed or are not supported yet, and the latest problems.
| Only callers who may run imports reach any of it.
|
*/

package migrations

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The refusals of the migrations API.
*/
var (
	ErrNotFound = fault.New(fault.NotFound, "migration_not_found", "No migration of this workspace has this id.")
	ErrSource   = fault.New(fault.Invalid, "migration_source_invalid",
		"A source is 1 to 40 lowercase letters, digits, dashes or underscores, starting with a letter or digit.").
		OnField("source")
	ErrSourceAccount = fault.New(fault.Invalid, "migration_source_account_invalid",
		"A source account is 1 to 255 characters.").OnField("source_account")
	ErrClosed = fault.New(fault.Conflict, "migration_closed",
		"This migration was undone or is being undone, so it takes no more records.")
	ErrRollingBack = fault.New(fault.Conflict, "migration_rollback_running",
		"This migration is already being undone, or was undone.")
	ErrUnknownKind = fault.New(fault.Invalid, "migration_kind_unknown",
		"This is not a kind of record the import format knows.").OnField("kind")
	ErrTooMany = fault.New(fault.Invalid, "migration_batch_too_large",
		"A batch holds at most 100 records.").OnField("records")
	ErrEmpty = fault.New(fault.Invalid, "migration_batch_empty", "A batch holds at least one record.").
			OnField("records")
)

/*
sourcePattern is what a source's name looks like.
*/
var sourcePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

/*
MaxBatch is how many records one batch holds, and MaxBatchBytes how
large its body may be.
*/
const (
	MaxBatch      = 100
	MaxBatchBytes = 5 << 20
)

/*
maxProblems is how many of the latest problems a migration shows.
*/
const maxProblems = 50

/*
Migrations runs migrations.
*/
type Migrations struct {
	DB    *database.DB
	Clock clock.Clock
	Queue *queue.Queue
	IDs   *ids.Registry
	Kit   Kit
}

/*
Migration is a migration with its progress: counts per kind and the
latest problems.
*/
type Migration struct {
	Row      models.Migration
	Counts   []KindCount
	Problems []models.MigrationRecord
}

/*
KindCount is how many records of one kind reached each status, how
many of them the migration created, and how many carry conflicts.
*/
type KindCount struct {
	Kind        string
	Applied     int
	Created     int
	Deferred    int
	Failed      int
	Unsupported int
	Conflicted  int
}

/*
StartInput is who starts a migration from which account of which tool.
*/
type StartInput struct {
	Membership    authz.Membership
	Source        string
	SourceAccount string
	Name          *string
}

/*
CanMigrate refuses a caller who may not run imports.
*/
func CanMigrate(m authz.Membership) error {
	if !m.Can("imports:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
migrator returns the membership the migration writes as: the caller's
member with every permission of their role, so a token granted only the
import permission still writes conversations, contacts and articles the
member's own role allows.
*/
func migrator(m authz.Membership) authz.Membership {
	out := m
	out.Principal.Permissions = appauth.RolePermissions(enums.Role(m.Member.Role))

	return out
}

/*
Start starts the migration, or finds the one already under way for the
same account, and reports whether it made a new one. A new name renames
the one found.

Rules:
  - The caller needs imports:manage.
  - The source is a short lowercase name; the account is the id of the
    account in that tool.

Events: none.
*/
func (s *Migrations) Start(ctx context.Context, in StartInput) (Migration, bool, error) {
	if err := CanMigrate(in.Membership); err != nil {
		return Migration{}, false, err
	}

	source := strings.TrimSpace(in.Source)
	if !sourcePattern.MatchString(source) {
		return Migration{}, false, ErrSource
	}

	account := strings.TrimSpace(in.SourceAccount)
	if account == "" || len([]rune(account)) > 255 {
		return Migration{}, false, ErrSourceAccount
	}

	name := cleanName(in.Name)
	ws := in.Membership.Workspace.ID
	ctx = tenancy.With(ctx, ws.String())

	var (
		row     models.Migration
		created bool
	)

	params := models.CreateMigrationParams{
		WorkspaceID: ws, Source: source, SourceAccount: account, Name: name,
		CreatedByMemberID: &in.Membership.Member.ID, Now: s.Clock.Now(),
	}

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, created, err = s.findOrCreate(ctx, models.New(tx), params)

		return err
	})
	if err != nil {
		return Migration{}, false, err
	}

	out, err := s.progress(ctx, row)

	return out, created, err
}

/*
findOrCreate returns the live migration of the account, renamed when a
new name came, or a new one, and reports whether it made one.
*/
func (s *Migrations) findOrCreate(ctx context.Context, q *models.Queries, p models.CreateMigrationParams,
) (models.Migration, bool, error) {
	found, err := q.FindLiveMigration(ctx, models.FindLiveMigrationParams{
		WorkspaceID: p.WorkspaceID, Source: p.Source, SourceAccount: p.SourceAccount,
	})

	switch {
	case err == nil:
		if p.Name == nil || (found.Name != nil && *found.Name == *p.Name) {
			return found, false, nil
		}

		renamed, err := q.RenameMigration(ctx, models.RenameMigrationParams{
			Name: p.Name, Now: p.Now, WorkspaceID: p.WorkspaceID, ID: found.ID,
		})

		return renamed, false, wrap("rename migration", err)
	case !errors.Is(err, database.ErrNotFound):
		return found, false, fmt.Errorf("find migration: %w", err)
	}

	row, err := q.CreateMigration(ctx, p)

	return row, true, wrap("create migration", err)
}

/*
cleanName trims the name, nil when it is empty, cut to 120 characters.
*/
func cleanName(name *string) *string {
	if name == nil {
		return nil
	}

	n := strings.TrimSpace(*name)
	if n == "" {
		return nil
	}

	return new(cut(n, 120))
}

/*
List returns the workspace's latest migrations, newest first, without
their progress.
*/
func (s *Migrations) List(ctx context.Context, m authz.Membership) ([]models.Migration, error) {
	if err := CanMigrate(m); err != nil {
		return nil, err
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var rows []models.Migration

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListMigrations(ctx, models.ListMigrationsParams{
			WorkspaceID: m.Workspace.ID, RowLimit: 100,
		})

		return wrap("list migrations", err)
	})

	return rows, err
}

/*
Show returns the migration with its progress.
*/
func (s *Migrations) Show(ctx context.Context, m authz.Membership, id ids.UUID) (Migration, error) {
	if err := CanMigrate(m); err != nil {
		return Migration{}, err
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	row, err := s.find(ctx, m.Workspace.ID, id)
	if err != nil {
		return Migration{}, err
	}

	return s.progress(ctx, row)
}

/*
find returns the migration of the workspace.
*/
func (s *Migrations) find(ctx context.Context, ws, id ids.UUID) (models.Migration, error) {
	var row models.Migration

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindMigration(ctx, models.FindMigrationParams{WorkspaceID: ws, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return ErrNotFound
		}

		return wrap("find migration", err)
	})

	return row, err
}

/*
progress reads the migration's counts and latest problems.
*/
func (s *Migrations) progress(ctx context.Context, row models.Migration) (Migration, error) {
	out := Migration{Row: row}

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		counts, err := q.CountMigrationRecords(ctx, row.ID)
		if err != nil {
			return fmt.Errorf("count migration records: %w", err)
		}

		out.Counts = tally(counts)

		out.Problems, err = q.ListMigrationProblems(ctx, models.ListMigrationProblemsParams{
			MigrationID: row.ID, RowLimit: maxProblems,
		})

		return wrap("list migration problems", err)
	})

	return out, err
}

/*
tally folds the counts by status into one count per kind, in sending
order; files come last.
*/
func tally(rows []models.CountMigrationRecordsRow) []KindCount {
	byKind := map[string]*KindCount{}

	for _, r := range rows {
		c, ok := byKind[r.Kind]
		if !ok {
			c = &KindCount{Kind: r.Kind}
			byKind[r.Kind] = c
		}

		n := int(r.Records)
		c.Created += int(r.Created)
		c.Conflicted += int(r.Conflicted)

		switch r.Status {
		case statusApplied:
			c.Applied += n
		case statusDeferred:
			c.Deferred += n
		case statusFailed:
			c.Failed += n
		case statusUnsupported:
			c.Unsupported += n
		}
	}

	var out []KindCount

	for _, kind := range append(Kinds(), kindFile) {
		if c, ok := byKind[kind]; ok {
			out = append(out, *c)
		}
	}

	return out
}

/*
wrap wraps a database error with what was being done, nil for none.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*fault.Error](err); ok {
		return err
	}

	return fmt.Errorf("%s: %w", what, err)
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

/*
orNil returns a pointer to the text, nil when it is empty.
*/
func orNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	return &s
}

/*
timeOr returns the time, or the fallback when it is nil or zero.
*/
func timeOr(at *time.Time, fallback time.Time) time.Time {
	if at == nil || at.IsZero() {
		return fallback
	}

	return at.UTC()
}
