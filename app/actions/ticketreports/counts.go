/*
|--------------------------------------------------------------------------
| Ticket Counts
|--------------------------------------------------------------------------
|
| How many tickets there are of each type, created and resolved in the
| range and open now, and how the tickets of one type created in the
| range split by the value of one of its fields. A multiple choice field
| counts a ticket once under each choice it holds; tickets without a
| value are counted under none.
|
*/

package ticketreports

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of reading a field's report.
*/
var (
	ErrTypeNotFound = fault.New(fault.NotFound, "ticket_type_not_found",
		"No ticket type of this workspace has this id.").OnField("type_id")
	ErrFieldNotFound = fault.New(fault.NotFound, "ticket_field_not_found",
		"The ticket type has no field with this key.").OnField("field")
)

/*
TypeCount is one type's tickets: created and resolved in the range, and
open now.
*/
type TypeCount struct {
	Type     models.TicketType
	Created  int64
	Resolved int64
	Open     int64
}

/*
ByType counts each live type's tickets, types without tickets included.
*/
func (r *Reports) ByType(ctx context.Context, m authz.Membership, rg Range) ([]TypeCount, error) {
	var out []TypeCount

	err := r.read(ctx, m, rg, func(ctx context.Context, q *models.Queries, rq reports.Query) error {
		var err error
		out, err = byType(ctx, q, rq)

		return err
	})

	return out, err
}

/*
byType counts tickets per type.
*/
func byType(ctx context.Context, q *models.Queries, rq reports.Query) ([]TypeCount, error) {
	a := access(rq)

	rows, err := q.CountReportTicketsByType(ctx, models.CountReportTicketsByTypeParams{
		RangeFrom: rq.Start, RangeTo: rq.End, WorkspaceID: rq.Membership.Workspace.ID, AccessMode: a.Mode,
		AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return nil, fmt.Errorf("count tickets by type: %w", err)
	}

	types, err := q.ListTicketTypes(ctx, rq.Membership.Workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("list ticket types: %w", err)
	}

	out := make([]TypeCount, 0, len(types))
	for _, t := range types {
		i := slices.IndexFunc(rows, func(r models.CountReportTicketsByTypeRow) bool { return r.TypeID == t.ID })
		if i < 0 && t.ArchivedAt != nil {
			continue
		}

		c := TypeCount{Type: t}
		if i >= 0 {
			c.Created, c.Resolved, c.Open = rows[i].Created, rows[i].Resolved, rows[i].Open
		}

		out = append(out, c)
	}

	return out, nil
}

/*
FieldValue is how many tickets hold one value of the field, and how
many of those are resolved. Value is nil for tickets without one.
*/
type FieldValue struct {
	Value    *string
	Tickets  int64
	Resolved int64
}

/*
FieldReport is a field of a type and its values' counts.
*/
type FieldReport struct {
	Type   models.TicketType
	Field  models.TicketField
	Values []FieldValue
}

/*
FieldInput names the type and the key of its field to count by.
*/
type FieldInput struct {
	Range
	TypeID ids.UUID
	Field  string
}

/*
ByField counts the type's tickets created in the range by the field's
values.
*/
func (r *Reports) ByField(ctx context.Context, m authz.Membership, in FieldInput) (FieldReport, error) {
	var out FieldReport

	err := r.read(ctx, m, in.Range, func(ctx context.Context, q *models.Queries, rq reports.Query) error {
		var err error
		out, err = byField(ctx, q, rq, in)

		return err
	})

	return out, err
}

/*
byField finds the field and counts its values.
*/
func byField(ctx context.Context, q *models.Queries, rq reports.Query, in FieldInput) (FieldReport, error) {
	n, err := loadNames(ctx, q, rq)
	if err != nil {
		return FieldReport{}, err
	}

	ty, ok := n.types[in.TypeID]
	if !ok {
		return FieldReport{}, ErrTypeNotFound
	}

	fields, err := q.ListTicketFields(ctx, rq.Membership.Workspace.ID)
	if err != nil {
		return FieldReport{}, fmt.Errorf("list ticket fields: %w", err)
	}

	i := slices.IndexFunc(fields, func(f models.TicketField) bool { return f.TypeID == ty.ID && f.Key == in.Field })
	if i < 0 {
		return FieldReport{}, ErrFieldNotFound
	}

	a := access(rq)

	rows, err := q.CountReportTicketsByField(ctx, models.CountReportTicketsByFieldParams{
		FieldKey: in.Field, WorkspaceID: rq.Membership.Workspace.ID, TypeID: ty.ID, RangeFrom: rq.Start,
		RangeTo: rq.End, AccessMode: a.Mode, AccessViewer: a.Viewer, AccessTeams: a.Teams,
		AccessUnassigned: a.Unassigned, AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return FieldReport{}, fmt.Errorf("count tickets by field: %w", err)
	}

	out := FieldReport{Type: ty, Field: fields[i], Values: make([]FieldValue, 0, len(rows))}
	for _, row := range rows {
		v := FieldValue{Tickets: row.Tickets, Resolved: row.Resolved}
		if row.Value != "" {
			v.Value = &row.Value
		}

		out.Values = append(out.Values, v)
	}

	return out, nil
}
