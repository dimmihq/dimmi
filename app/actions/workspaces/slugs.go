/*
|--------------------------------------------------------------------------
| Workspace Slugs
|--------------------------------------------------------------------------
|
| A slug is a workspace's address: lowercase ASCII letters, digits and
| single dashes, 3 to 40 characters, a shape requests check before an
| action sees it. Some slugs would read as part of the product and are
| never handed out. A slug a workspace gives up keeps pointing at it for
| a hold period, during which nobody else may take it, so old links
| never land in someone else's workspace.
|
*/

package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of a slug, the suffix numbers tried before a random one, and
how long a slug given up stays held for its old workspace.
*/
const (
	slugMinLength = 3
	slugMaxLength = 40
	slugMaxSuffix = 99
	slugHold      = 30 * 24 * time.Hour
)

/*
reservedSlug reports whether the slug would read as part of the
product, such as a page of the dashboard, whose addresses start with
the workspace's slug.
*/
func reservedSlug(slug string) bool {
	return slices.Contains([]string{
		"api", "app", "www", "admin", "help", "docs", "status", "mail", "support", "dimmi", "auth", "login",
		"setup", "static", "assets", "cdn", "widget", "mcp", "oauth", "billing", "inbox", "settings", "customers",
		"notifications", "agent", "feedback", "announcements", "changelog", "shortcuts", "procedures", "workflows",
		"broadcasts", "series", "operator",
	}, slug)
}

/*
slugTaken reports whether a workspace other than the one the check
names holds its slug, as its own or as one it gave up within the hold
period at the check's time.
*/
func slugTaken(ctx context.Context, q *models.Queries, check models.CountWorkspacesWithSlugParams) (bool, error) {
	taken, err := q.CountWorkspacesWithSlug(ctx, check)
	if err != nil {
		return false, fmt.Errorf("check workspace slug: %w", err)
	}

	return taken > 0, nil
}

/*
heldSlug is a slug a workspace gave up, held for it until a time.
*/
type heldSlug struct {
	Slug  string    `json:"slug"`
	Until time.Time `json:"until"`
}

/*
holdSlug returns the settings with the old slug held for the hold
period. Holds that ran out are dropped, and so is a hold on the slug the
workspace takes back.
*/
func holdSlug(settings json.RawMessage, old, next string, now time.Time) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	var held []heldSlug
	if raw, ok := values["previous_slugs"]; ok {
		if err := json.Unmarshal(raw, &held); err != nil {
			return nil, fmt.Errorf("read previous slugs: %w", err)
		}
	}

	held = slices.DeleteFunc(held, func(h heldSlug) bool {
		return !h.Until.After(now) || strings.EqualFold(h.Slug, next) || strings.EqualFold(h.Slug, old)
	})
	held = append(held, heldSlug{Slug: old, Until: now.Add(slugHold).UTC()})

	raw, err := json.Marshal(held)
	if err != nil {
		return nil, fmt.Errorf("write previous slugs: %w", err)
	}

	values["previous_slugs"] = raw

	return json.Marshal(values)
}
