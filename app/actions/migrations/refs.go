/*
|--------------------------------------------------------------------------
| References
|--------------------------------------------------------------------------
|
| Records name each other by source id. Before a record is applied, the
| records it names are looked up in the ledger. One that never arrived
| holds the record back until it does. One that arrived but could not be
| applied is unavailable: an author that names it becomes a system
| author labelled with its name, and an optional reference is left out.
|
*/

package migrations

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
resolved is the ledger rows of the records a record named, by
reference; a reference with no row never arrived.
*/
type resolved struct {
	asked []Ref
	rows  map[Ref]models.MigrationRecord
}

/*
resolve looks up the references in the ledger.
*/
func (r *runner) resolve(ctx context.Context, q *models.Queries, refs []Ref) (resolved, error) {
	out := resolved{rows: map[Ref]models.MigrationRecord{}}

	byKind := map[string][]string{}

	for _, ref := range refs {
		if ref.SourceID == "" || slices.Contains(out.asked, ref) {
			continue
		}

		out.asked = append(out.asked, ref)
		byKind[ref.Kind] = append(byKind[ref.Kind], ref.SourceID)
	}

	for kind, sourceIDs := range byKind {
		rows, err := q.FindMigrationRecords(ctx, models.FindMigrationRecordsParams{
			MigrationID: r.row.ID, Kind: kind, SourceIds: sourceIDs,
		})
		if err != nil {
			return out, fmt.Errorf("resolve references: %w", err)
		}

		for _, row := range rows {
			out.rows[Ref{Kind: kind, SourceID: row.SourceID}] = row
		}
	}

	return out, nil
}

/*
missing returns the references that never arrived, or that arrived but
are still waiting themselves.
*/
func (r resolved) missing() []Ref {
	var out []Ref

	for _, ref := range r.asked {
		row, ok := r.rows[ref]
		if !ok || row.Status == statusDeferred {
			out = append(out, ref)
		}
	}

	return out
}

/*
id returns the row the reference became, and reports whether it is
there.
*/
func (r resolved) id(kind, sourceID string) (ids.UUID, bool) {
	if sourceID == "" {
		return ids.Nil(), false
	}

	row, ok := r.rows[Ref{Kind: kind, SourceID: sourceID}]
	if !ok || row.Status != statusApplied || row.RecordID == nil {
		return ids.Nil(), false
	}

	return *row.RecordID, true
}

/*
label returns the name the reference was known by, empty when none.
*/
func (r resolved) label(kind, sourceID string) string {
	row, ok := r.rows[Ref{Kind: kind, SourceID: sourceID}]
	if !ok || row.Label == nil {
		return ""
	}

	return *row.Label
}

/*
must returns the row a required reference became, refusing one that
arrived but could not be applied.
*/
func (r resolved) must(field, kind, sourceID string) (ids.UUID, error) {
	id, ok := r.id(kind, sourceID)
	if !ok {
		return id, ErrReferenceUnavailable.OnField(field)
	}

	return id, nil
}

/*
ids returns the rows the references of the kind became, leaving out
those that are not there.
*/
func (r resolved) ids(kind string, sourceIDs []string) []ids.UUID {
	var out []ids.UUID

	for _, sid := range sourceIDs {
		if id, ok := r.id(kind, sid); ok && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}

/*
refsOf returns references of the kind to the source ids.
*/
func refsOf(kind string, sourceIDs ...string) []Ref {
	out := make([]Ref, 0, len(sourceIDs))

	for _, sid := range sourceIDs {
		if sid != "" {
			out = append(out, Ref{Kind: kind, SourceID: sid})
		}
	}

	return out
}

/*
authorRefs returns the reference an author makes, none for an AI agent
or the system.
*/
func authorRefs(a *Author) []Ref {
	if a == nil {
		return nil
	}

	switch a.Type {
	case AuthorTeammate:
		return refsOf(KindTeammate, a.Ref)
	case AuthorContact:
		return refsOf(KindContact, a.Ref)
	}

	return nil
}

/*
checkAuthor refuses an author of no known type, or a teammate or contact
without a reference.
*/
func checkAuthor(field string, a Author) error {
	if err := oneOf(field+".type", a.Type, AuthorTeammate, AuthorContact, AuthorAIAgent, AuthorSystem); err != nil {
		return err
	}

	if a.Type == "" {
		return invalid(field+".type", field+".type is required.")
	}

	if (a.Type == AuthorTeammate || a.Type == AuthorContact) && a.Ref == "" {
		return invalid(field+".ref", "A teammate or contact author names its record in ref.")
	}

	return nil
}

/*
member returns the teammate the author became, and reports whether the
author is one that was applied.
*/
func (s *step) member(a *Author) (ids.UUID, bool) {
	if a == nil || a.Type != AuthorTeammate {
		return ids.Nil(), false
	}

	return s.refs.id(KindTeammate, a.Ref)
}

/*
authorName returns the name the author is shown with when no teammate
holds it: the AI agent's label, the teammate's imported name, or empty.
*/
func (s *step) authorName(a *Author) string {
	if a == nil {
		return ""
	}

	switch a.Type {
	case AuthorAIAgent:
		if a.Label != "" {
			return cut(a.Label, 80)
		}

		return "AI agent (imported)"
	case AuthorTeammate:
		if name := s.refs.label(KindTeammate, a.Ref); name != "" {
			return name + " (imported)"
		}

		return "Teammate (imported)"
	}

	return ""
}
