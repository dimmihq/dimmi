/*
|--------------------------------------------------------------------------
| Statuses
|--------------------------------------------------------------------------
|
| Where a feature request stands. Each workspace keeps its own statuses:
| it starts with the built-in five and names, colors, adds and removes
| them from there. Every status falls in a category that says what it
| means: new requests start in the first open status, a done status is
| what shipping means, and closed requests leave the public roadmap but
| keep their page. A request names its status by key.
|
*/

package feedback

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The categories a status falls in, in the order work moves through them.
*/
const (
	CategoryOpen       = "open"
	CategoryPlanned    = "planned"
	CategoryInProgress = "in_progress"
	CategoryDone       = "done"
	CategoryClosed     = "closed"
)

/*
Categories returns every category, in the order work moves through
them.
*/
func Categories() []string {
	return []string{CategoryOpen, CategoryPlanned, CategoryInProgress, CategoryDone, CategoryClosed}
}

/*
Status is one stage a request moves through: its stored key, the name
and slug readers see, its color, its category and whether the public
roadmap shows it as a column.
*/
type Status struct {
	Key      string
	Name     string
	Slug     string
	Color    string
	Category string
	Roadmap  bool
}

/*
Statuses returns the built-in statuses, in the order a workspace starts
with.
*/
func Statuses() []Status {
	return []Status{
		{
			Key: "in_progress", Name: "In progress", Slug: "in-progress", Color: "#f59e0b",
			Category: CategoryInProgress, Roadmap: true,
		},
		{Key: "planned", Name: "Planned", Slug: "planned", Color: "#3b82f6", Category: CategoryPlanned, Roadmap: true},
		{
			Key: "under_review", Name: "Under consideration", Slug: "under-consideration", Color: "#a1a1aa",
			Category: CategoryOpen, Roadmap: true,
		},
		{Key: "shipped", Name: "Shipped", Slug: "shipped", Color: "#16a34a", Category: CategoryDone, Roadmap: true},
		{Key: "closed", Name: "Closed", Slug: "closed", Color: "#71717a", Category: CategoryClosed},
	}
}

/*
StatusOf returns the built-in status with the key, and whether there is
one.
*/
func StatusOf(key string) (Status, bool) {
	return statusIn(Statuses(), key)
}

/*
statusIn returns the status of the list with the key.
*/
func statusIn(list []Status, key string) (Status, bool) {
	for _, s := range list {
		if s.Key == key {
			return s, true
		}
	}
	return Status{}, false
}

/*
OnRoadmap says whether the public roadmap shows the status as a column:
it asks to be shown and is not closed.
*/
func (s Status) OnRoadmap() bool {
	return s.Roadmap && s.Category != CategoryClosed
}

/*
statusOfRow is a stored status as the feature reads it.
*/
func statusOfRow(r models.FeedbackStatus) Status {
	slug := helpcenters.Slugify(r.Name, 60)
	if slug == "" {
		slug = strings.ReplaceAll(r.Key, "_", "-")
	}
	return Status{Key: r.Key, Name: r.Name, Slug: slug, Color: r.Color, Category: r.Category, Roadmap: r.Roadmap}
}

/*
ordered returns the workspace's statuses in its order, giving a
workspace without any the built-in ones first.
*/
func ordered(ctx context.Context, q *models.Queries, ws ids.UUID) ([]Status, error) {
	rows, err := q.ListFeedbackStatuses(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list feature request statuses: %w", err)
	}
	if len(rows) == 0 {
		for i, s := range Statuses() {
			if err := q.SeedFeedbackStatus(ctx, models.SeedFeedbackStatusParams{
				WorkspaceID: ws, Key: s.Key, Name: s.Name, Color: s.Color, Category: s.Category, Roadmap: s.Roadmap,
				Position: int32(i),
			}); err != nil {
				return nil, fmt.Errorf("seed feature request statuses: %w", err)
			}
		}
		if rows, err = q.ListFeedbackStatuses(ctx, ws); err != nil {
			return nil, fmt.Errorf("list feature request statuses: %w", err)
		}
	}
	out := make([]Status, len(rows))
	for i, r := range rows {
		out[i] = statusOfRow(r)
	}
	return out, nil
}

/*
startingStatus is the status a new request starts in: the workspace's
first open status, or its first status when none is open.
*/
func startingStatus(list []Status) string {
	if len(list) == 0 {
		return "under_review"
	}
	i := slices.IndexFunc(list, func(s Status) bool { return s.Category == CategoryOpen })
	return list[max(i, 0)].Key
}

/*
statusOfKey returns the workspace's status with the key, or
ErrStatusInvalid.
*/
func statusOfKey(ctx context.Context, q *models.Queries, ws ids.UUID, key string) (Status, error) {
	list, err := ordered(ctx, q, ws)
	if err != nil {
		return Status{}, err
	}
	s, ok := statusIn(list, key)
	if !ok {
		return Status{}, ErrStatusInvalid
	}
	return s, nil
}
