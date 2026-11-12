/*
|--------------------------------------------------------------------------
| Status Board
|--------------------------------------------------------------------------
|
| The help center's public status page reads like the status pages
| customers know: one line for the whole product, then each affected
| area with its current status, then the incidents active now with
| their public logs, then those resolved in the last ninety days. An
| area's status is the worst severity among the active incidents that
| touch it; an area nothing touches is operational. The areas are the
| options of the incident types' affected areas field.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
StatusHistory is how long a resolved incident stays in the history.
*/
const StatusHistory = 90 * 24 * time.Hour

/*
The statuses of an area and of the whole product, best first.
*/
const (
	StatusOperational = "operational"
	StatusDegraded    = "degraded"
	StatusPartial     = "partial_outage"
	StatusMajor       = "major_outage"
)

/*
StatusComponent is one affected area and its current status.
*/
type StatusComponent struct {
	Name   string
	Status string
}

/*
StatusBoard is the status page: the overall status, the areas, the
incidents active now and those resolved in the history window.
*/
type StatusBoard struct {
	Overall    string
	Components []StatusComponent
	Active     []StatusIncident
	History    []StatusIncident
}

/*
Board returns the workspace's status page.
*/
func (t *Tickets) Board(ctx context.Context, ws ids.UUID) (StatusBoard, error) {
	var (
		out   StatusBoard
		areas []string
	)

	ctx = tenancy.With(ctx, ws.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		now := t.Clock.Now()

		setup, err := loadSetup(ctx, q, ws, now)
		if err != nil {
			return err
		}

		areas = areaOptions(setup)

		rows, err := q.ListRecentIncidents(ctx, models.ListRecentIncidentsParams{
			WorkspaceID: ws, Since: new(now.Add(-StatusHistory)),
		})
		if err != nil {
			return fmt.Errorf("list status incidents: %w", err)
		}

		out.Active, out.History, err = statusIncidents(ctx, q, setup, rows)

		return err
	})
	if err != nil {
		return out, err
	}

	out.Components, out.Overall = components(areas, out.Active)

	return out, nil
}

/*
statusIncidents splits the incidents into active and resolved, each
with its state and public log.
*/
func statusIncidents(
	ctx context.Context, q *models.Queries, setup Setup, rows []models.Ticket,
) ([]StatusIncident, []StatusIncident, error) {
	var active, history []StatusIncident

	for _, r := range rows {
		st, _ := setup.State(r.StateID)

		updates, err := q.ListTicketPublicUpdates(ctx, models.ListTicketPublicUpdatesParams{
			WorkspaceID: r.WorkspaceID, TicketID: r.ID,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list public updates: %w", err)
		}

		inc := StatusIncident{Ticket: r, State: st, Updates: updates}
		if r.StateGroup == GroupResolved {
			history = append(history, inc)
		} else {
			active = append(active, inc)
		}
	}

	return active, history, nil
}

/*
areaOptions lists the affected areas the incident types offer, in their
order, once each.
*/
func areaOptions(setup Setup) []string {
	var out []string

	for _, ty := range setup.Types {
		if ty.Kind != KindIncident || ty.ArchivedAt != nil {
			continue
		}

		for _, f := range setup.FieldsOf(ty.ID) {
			if f.Key == "areas" && f.ArchivedAt == nil {
				out = addNew(out, f.Options)
			}
		}
	}

	return out
}

/*
addNew appends the values the list does not hold yet.
*/
func addNew(list, values []string) []string {
	for _, v := range values {
		if !slices.Contains(list, v) {
			list = append(list, v)
		}
	}

	return list
}

/*
components gives each area the worst status of the active incidents
touching it, adding areas an incident names that no type offers, and
returns the worst status of all as the overall one.
*/
func components(areas []string, active []StatusIncident) ([]StatusComponent, string) {
	overall := StatusOperational
	worst := map[string]string{}

	for _, inc := range active {
		status := SeverityStatus(Severity(inc.Ticket))
		overall = worse(overall, status)

		for _, a := range areasOf(inc.Ticket) {
			if !slices.ContainsFunc(areas, func(o string) bool { return strings.EqualFold(o, a) }) {
				areas = append(areas, a)
			}

			key := strings.ToLower(a)
			worst[key] = worse(worst[key], status)
		}
	}

	out := make([]StatusComponent, len(areas))
	for i, a := range areas {
		out[i] = StatusComponent{Name: a, Status: worse(StatusOperational, worst[strings.ToLower(a)])}
	}

	return out, overall
}

/*
Severity reads an incident's severity field, empty when unset.
*/
func Severity(r models.Ticket) string {
	s, _ := objectOf(r.Fields)["severity"].(string)

	return strings.ToLower(s)
}

/*
SeverityStatus is the status an active incident of the severity gives:
critical is a major outage, major a partial outage, anything else
degraded performance.
*/
func SeverityStatus(severity string) string {
	switch severity {
	case "critical":
		return StatusMajor
	case "major":
		return StatusPartial
	}

	return StatusDegraded
}

/*
worse returns the worse of two statuses; empty counts as operational.
*/
func worse(a, b string) string {
	rank := func(s string) int {
		return slices.Index([]string{StatusOperational, StatusDegraded, StatusPartial, StatusMajor}, s)
	}

	if rank(b) > rank(a) {
		return b
	}

	if a == "" {
		return StatusOperational
	}

	return a
}

/*
Areas reads an incident's affected areas.
*/
func Areas(r models.Ticket) []string {
	return areasOf(r)
}
