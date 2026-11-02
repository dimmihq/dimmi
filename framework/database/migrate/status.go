/*
|--------------------------------------------------------------------------
| Migration Status And Plan
|--------------------------------------------------------------------------
|
| Operators look before they leap. Status lists every migration and
| whether it ran; Plan shows what would run next, with its SQL and lint
| findings, without touching the schema. Neither takes the migration
| lock, so both work while another server is migrating.
|
*/

package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"time"

	"github.com/pressly/goose/v3"
)

/*
Migration is one migration's state: its set, version, file name,
whether it ran and when.
*/
type Migration struct {
	Set       string
	Version   int64
	Name      string
	Applied   bool
	AppliedAt time.Time
}

/*
Pending is a migration that Up would apply. Transactional is false for
files marked "-- +goose NO TRANSACTION". Phase is the phase its header
declares, SQL its up section, and Findings what Lint reports about it.
*/
type Pending struct {
	Set           string
	Version       int64
	Name          string
	Phase         string
	Transactional bool
	SQL           string
	Findings      []Finding
}

/*
Status lists every migration of every set, in the order Up applies them.
*/
func (r Runner) Status(ctx context.Context) ([]Migration, error) {
	r = r.withDefaults()

	var migrations []Migration

	err := r.withSession(ctx, func(ctx context.Context, s *session) error {
		for _, set := range s.sets {
			statuses, err := set.provider.Status(ctx)
			if err != nil {
				return fmt.Errorf("status of %s: %w", set.name, err)
			}

			for _, status := range statuses {
				migrations = append(migrations, Migration{
					Set:       set.name,
					Version:   status.Source.Version,
					Name:      path.Base(status.Source.Path),
					Applied:   status.State == goose.StateApplied,
					AppliedAt: status.AppliedAt,
				})
			}
		}

		return nil
	})

	return migrations, err
}

/*
Plan lists the migrations Up would apply, in order, without applying
anything.
*/
func (r Runner) Plan(ctx context.Context) ([]Pending, error) {
	r = r.withDefaults()

	var plan []Pending

	err := r.withSession(ctx, func(ctx context.Context, s *session) error {
		for _, set := range s.sets {
			pending, err := planSet(ctx, set)
			if err != nil {
				return err
			}

			plan = append(plan, pending...)
		}

		return nil
	})

	return plan, err
}

func planSet(ctx context.Context, set migrationSet) ([]Pending, error) {
	statuses, err := set.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("status of %s: %w", set.name, err)
	}

	findings := Lint(set.fsys)

	var plan []Pending

	for _, status := range statuses {
		if status.State != goose.StatePending {
			continue
		}

		text, err := fs.ReadFile(set.fsys, status.Source.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s migration %s: %w", set.name, status.Source.Path, err)
		}

		file := parseFile(status.Source.Path, string(text))
		plan = append(plan, Pending{
			Set:           set.name,
			Version:       status.Source.Version,
			Name:          file.path,
			Phase:         file.phase,
			Transactional: file.transactional,
			SQL:           file.up,
			Findings:      findingsFor(findings, file.path),
		})
	}

	return plan, nil
}

func findingsFor(findings []Finding, name string) []Finding {
	var matched []Finding

	for _, finding := range findings {
		if finding.Path == name {
			matched = append(matched, finding)
		}
	}

	return matched
}
