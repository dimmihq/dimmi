/*
|--------------------------------------------------------------------------
| Widget Changelog Controller
|--------------------------------------------------------------------------
|
| The messenger's "What's new" tab: the newest published entries of the
| changelog of the help center the messenger links to, each body as plain
| blocks, newest first. A changelog is public, so anyone gets it, before
| the visitor says anything. Without a help center, or with the tab off,
| the tab stays empty.
|
*/

package widget

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
maxChangelogEntries caps the entries the tab carries at once.
*/
const maxChangelogEntries = 20

/*
ChangelogController serves the messenger's "What's new" tab.
*/
type ChangelogController struct {
	Public    *helpcenters.Public
	Changelog *changelog.Public
	Addresses helpcenters.Addresses
}

/*
ChangelogEntry is a published entry as the messenger shows it: the name
of its first label, when it has one, and when it was published, which
the messenger compares with what the visitor last saw.
*/
type ChangelogEntry struct {
	Slug        string              `json:"slug"`
	Title       string              `json:"title"`
	Summary     string              `json:"summary"`
	Label       string              `json:"label"`
	PublishedAt time.Time           `json:"published_at"`
	Body        []helpcenters.Block `json:"body"`
}

/*
ChangelogIndex is everything the tab draws, and the address of the full
changelog, empty without one.
*/
type ChangelogIndex struct {
	URL     string           `json:"url"`
	Entries []ChangelogEntry `json:"entries"`
}

/*
ChangelogOutput answers the tab.
*/
type ChangelogOutput struct {
	Body struct {
		Data ChangelogIndex `json:"data"`
	}
}

/*
Show answers the "What's new" tab of the messenger the request names.
*/
func (c *ChangelogController) Show(ctx context.Context, _ *requests.NoInput) (*ChangelogOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)
	out := &ChangelogOutput{}
	out.Body.Data = ChangelogIndex{Entries: []ChangelogEntry{}}
	w := resolved.Widget
	if w.HelpCenterID == nil || !w.ChangelogTabEnabled || c.Public == nil || c.Changelog == nil {
		return out, nil
	}

	snap, err := c.Public.LoadByID(ctx, resolved.Workspace.ID, *w.HelpCenterID)
	if errors.Is(err, helpcenters.ErrNoHelpCenter) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}

	latest, err := c.Changelog.Latest(ctx, snap.HelpCenter, maxChangelogEntries)
	if err != nil {
		return nil, err
	}

	for _, p := range latest {
		out.Body.Data.Entries = append(out.Body.Data.Entries, entryOf(p))
	}
	out.Body.Data.URL = c.Addresses.Root(snap.HelpCenter) + "/changelog"

	return out, nil
}

/*
entryOf shapes a published entry for the tab, as it was last published.
*/
func entryOf(p changelog.Published) ChangelogEntry {
	e := p.ChangelogEntry
	out := ChangelogEntry{
		Slug: e.Slug, Title: deref(e.PublishedTitle), Summary: deref(e.PublishedSummary),
		Body: helpcenters.Blocks(deref(e.PublishedBodyMarkdown)),
	}
	if e.PublishedAt != nil {
		out.PublishedAt = *e.PublishedAt
	}
	if len(p.Labels) > 0 {
		out.Label = p.Labels[0].Name
	}

	return out
}
