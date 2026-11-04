/*
|--------------------------------------------------------------------------
| Inbox Preferences
|--------------------------------------------------------------------------
|
| How each teammate arranges their own inbox: the sidebar, and per view
| the sort, the status shown, the layout and its columns, plus the
| customer attributes they pin. Preferences are personal, so reading and
| saving them takes only conversations:read, and the event about them
| reaches that member alone, which keeps their other tabs in step.
|
| The document stays small: a save over 64 KB is refused. A view a
| preference names may disappear, such as a team being deleted; reading
| leaves those out, and the next save drops them for good.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxPreferencesSize is the largest preferences document saved, in bytes.
*/
const MaxPreferencesSize = 64 * 1024

/*
ViewPreference is how a member likes one view: its sort, the status it
shows, list or table layout and the table's columns. Empty means the
default.
*/
type ViewPreference struct {
	Sort    string   `json:"sort,omitempty"`
	Status  string   `json:"status,omitempty"`
	Layout  string   `json:"layout,omitempty"`
	Columns []string `json:"columns,omitempty"`
}

/*
Preferences are a member's inbox preferences: the sidebar document, the
preferences of each view by its key (mine, team_..., cvw_...), the
pinned customer attributes, and when they were last saved, nil before
the first save.
*/
type Preferences struct {
	Sidebar          json.RawMessage
	Views            map[string]ViewPreference
	PinnedAttributes []string
	UpdatedAt        *time.Time
}

/*
PreferencesInput is who asks.
*/
type PreferencesInput struct {
	Membership authz.Membership
}

/*
ShowPreferences returns the caller's inbox preferences, leaving out
views whose team, member or inbox is gone.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ShowPreferences struct {
	DB  *database.DB
	IDs *ids.Registry
}

/*
Handle returns the preferences.
*/
func (a *ShowPreferences) Handle(ctx context.Context, in PreferencesInput) (Preferences, error) {
	if err := CanRead(in.Membership); err != nil {
		return Preferences{}, err
	}

	var out Preferences

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = preferenceStore{q: models.New(tx), ids: a.IDs}.load(ctx, in.Membership)

		return err
	})

	return out, err
}

/*
preferenceStore reads preferences inside a transaction.
*/
type preferenceStore struct {
	q   *models.Queries
	ids *ids.Registry
}

/*
load reads the member's preferences, the defaults before the first
save, without the views whose target is gone.
*/
func (s preferenceStore) load(ctx context.Context, m authz.Membership) (Preferences, error) {
	q := s.q

	out := Preferences{Sidebar: json.RawMessage(`{}`), Views: map[string]ViewPreference{}, PinnedAttributes: []string{}}

	row, err := q.MemberInboxPreferences(ctx, models.MemberInboxPreferencesParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, nil
	}

	if err != nil {
		return out, fmt.Errorf("load inbox preferences: %w", err)
	}

	out.Sidebar, out.UpdatedAt = row.Sidebar, &row.UpdatedAt

	if err := json.Unmarshal(row.Views, &out.Views); err != nil {
		return out, fmt.Errorf("read view preferences: %w", err)
	}

	if err := json.Unmarshal(row.PinnedAttributes, &out.PinnedAttributes); err != nil {
		return out, fmt.Errorf("read pinned attributes: %w", err)
	}

	return out, s.prune(ctx, m.Workspace.ID, out.Views)
}

/*
prune removes the views naming a team, member or inbox that is
gone. Custom views are kept: they answer for themselves when listed.
*/
func (s preferenceStore) prune(ctx context.Context, workspace ids.UUID, views map[string]ViewPreference) error {
	targets := map[ids.UUID]string{}

	for key := range views {
		if id, ok := viewTarget(s.ids, key); ok {
			targets[id] = key
		}
	}

	if len(targets) == 0 {
		return nil
	}

	wanted := make([]ids.UUID, 0, len(targets))
	for id := range targets {
		wanted = append(wanted, id)
	}

	live, err := s.q.LiveViewTargets(ctx, models.LiveViewTargetsParams{WorkspaceID: workspace, Ids: wanted})
	if err != nil {
		return fmt.Errorf("find live views: %w", err)
	}

	for id, key := range targets {
		if !slices.Contains(live, id) {
			delete(views, key)
		}
	}

	return nil
}

/*
viewTarget returns the team, member or inbox a view key names.
*/
func viewTarget(registry *ids.Registry, key string) (ids.UUID, bool) {
	for _, prefix := range []models.Prefix{models.PrefixTeam, models.PrefixMember, models.PrefixInbox} {
		if !strings.HasPrefix(key, string(prefix)+"_") {
			continue
		}

		id, err := registry.Decode(string(prefix), key)

		return id, err == nil
	}

	return ids.Nil(), false
}
