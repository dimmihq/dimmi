/*
|--------------------------------------------------------------------------
| Resolving Tags
|--------------------------------------------------------------------------
|
| A teammate names a tag by its id or just types its name, and a name no
| tag has yet becomes a new tag, so tagging never stops to ask. Names
| match without regard to case. The vocabulary and each record have a
| ceiling, so one busy importer cannot make every picker useless.
|
*/

package tags

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits on tags.
*/
const (
	MaxOnRecord    = 50
	MaxInWorkspace = 10000
)

/*
Refs names tags by id and by name.
*/
type Refs struct {
	IDs   []ids.UUID
	Names []string
}

/*
Empty reports whether no tag is named.
*/
func (r Refs) Empty() bool {
	return len(r.IDs) == 0 && len(r.Names) == 0
}

/*
Scope is the workspace tags are resolved in, and the time of the change.
*/
type Scope struct {
	WorkspaceID ids.UUID
	Now         time.Time
}

/*
Resolve returns the ids of the tags named, creating a tag for each name
no tag has. Ids are returned as given; the queries that link tags ignore
an id the workspace does not have.
*/
func Resolve(ctx context.Context, q *models.Queries, scope Scope, refs Refs) ([]ids.UUID, error) {
	out := append([]ids.UUID{}, refs.IDs...)

	names := cleanNames(refs.Names)
	if len(names) == 0 {
		return out, nil
	}

	if err := checkRoom(ctx, q, scope.WorkspaceID, names); err != nil {
		return nil, err
	}

	ensured, err := ensure(ctx, q, scope, names)
	if err != nil {
		return nil, err
	}

	for _, tag := range ensured {
		out = append(out, tag.ID)
	}

	return out, nil
}

/*
cleanNames trims the names and drops empty ones and repeats, without
regard to case.
*/
func cleanNames(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)

		if key == "" || seen[key] {
			continue
		}

		seen[key] = true
		out = append(out, name)
	}

	return out
}

/*
checkRoom refuses names that would take the vocabulary past its limit.
*/
func checkRoom(ctx context.Context, q *models.Queries, workspaceID ids.UUID, names []string) error {
	found, err := q.TagsByNames(ctx, models.TagsByNamesParams{WorkspaceID: workspaceID, Names: names})
	if err != nil {
		return fmt.Errorf("find tags: %w", err)
	}

	count, err := q.CountWorkspaceTags(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count tags: %w", err)
	}

	if count+int64(len(names)-len(found)) > MaxInWorkspace {
		return exceptions.ErrTagLimitReached
	}

	return nil
}

/*
ensure returns the tags with the names. A tag another transaction
creates at the same moment is missed by the first attempt, so a short
answer asks once more.
*/
func ensure(ctx context.Context, q *models.Queries, scope Scope, names []string) ([]models.EnsureTagsRow, error) {
	params := models.EnsureTagsParams{WorkspaceID: scope.WorkspaceID, Names: names, Now: scope.Now}

	rows, err := q.EnsureTags(ctx, params)
	if err == nil && len(rows) < len(names) {
		rows, err = q.EnsureTags(ctx, params)
	}

	if err != nil {
		return nil, fmt.Errorf("ensure tags: %w", err)
	}

	return rows, nil
}
