/*
|--------------------------------------------------------------------------
| Source Trees
|--------------------------------------------------------------------------
|
| A source with structure shows it as a tree to choose from: while it is
| being added, from the connected account alone, and afterwards from
| the saved source. A change of what the source reads, whether its
| ticks, its patterns or its connection, starts a sync at once, so newly
| chosen documents arrive and documents left out are removed without
| anyone pressing Sync now.
|
*/

package knowledgesources

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrNoTree is a source whose driver shows no tree.
*/
var ErrNoTree = fault.New(fault.Invalid, "knowledge_source_tree_unsupported",
	"This source has no items to choose from.").OnField("type")

/*
Tree returns the items of a source being added, read with its settings.
*/
func (e *Engine) Tree(ctx context.Context, in SourceInput) ([]sources.Node, error) {
	if !in.Membership.Can("knowledge:manage") {
		return nil, exceptions.ErrForbidden
	}

	d, err := e.driverOf(in.Type)
	if err != nil {
		return nil, err
	}

	browser, ok := d.(sources.Browser)
	if !ok {
		return nil, ErrNoTree
	}

	cfg, err := e.connected(ctx, in.Membership.Workspace.ID, d, configOf(splitSecrets(d.Describe(), in.Settings)))
	if err != nil {
		return nil, checkError(err)
	}

	nodes, err := browser.Tree(ctx, cfg)

	return nodes, checkError(err)
}

/*
SourceTree returns the items of a saved source.
*/
func (e *Engine) SourceTree(ctx context.Context, m authz.Membership, id ids.UUID) ([]sources.Node, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	var s models.KnowledgeSource

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error

		s, err = findSource(ctx, q, m.Workspace.ID, id)

		return err
	})
	if err != nil {
		return nil, err
	}

	d, err := e.driverOf(s.Type)
	if err != nil {
		return nil, err
	}

	browser, ok := d.(sources.Browser)
	if !ok {
		return nil, ErrNoTree
	}

	cfg, err := e.settingsOf(ctx, s)
	if err != nil {
		return nil, checkError(err)
	}

	nodes, err := browser.Tree(ctx, cfg)

	return nodes, checkError(err)
}

/*
readsDifferently reports whether a change makes the source read other
documents than before: other settings, such as another repository or
other ticks, or other include or exclude patterns.
*/
func readsDifferently(current models.KnowledgeSource, config json.RawMessage, include, exclude []string) bool {
	return !sameJSON(current.Config, config) || !slices.Equal(current.Include, include) ||
		!slices.Equal(current.Exclude, exclude)
}

/*
sameJSON compares two JSON documents by what they say, not by how the
database ordered their keys.
*/
func sameJSON(a, b json.RawMessage) bool {
	var left, right any

	_ = json.Unmarshal(a, &left)
	_ = json.Unmarshal(b, &right)

	l, _ := json.Marshal(left)
	r, _ := json.Marshal(right)

	return bytes.Equal(l, r)
}
