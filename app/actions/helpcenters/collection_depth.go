/*
|--------------------------------------------------------------------------
| Collection Depth
|--------------------------------------------------------------------------
|
| A help center's collections form a tree at most three levels deep, so
| readers find any article within a few clicks. A collection can never
| sit under itself or under one of its own sub-collections, and moving a
| collection carries everything inside it, so the deepest of those must
| still fit.
|
*/

package helpcenters

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxCollectionDepth is how many levels of collections a help center holds.
*/
const MaxCollectionDepth = 3

/*
checkParent refuses a parent that would nest the collection, with what
it holds, deeper than MaxCollectionDepth, or put it under itself or one
of its own sub-collections. A new collection has no id yet.
*/
func checkParent(ctx context.Context, q *models.Queries, c models.Collection, parent *ids.UUID) error {
	if parent == nil {
		return nil
	}
	if *parent == c.ID {
		return ErrCollectionDepth
	}
	found, err := findCollection(ctx, q, c.WorkspaceID, *parent)
	if err != nil {
		return err
	}
	all, err := q.ListCollections(ctx, models.ListCollectionsParams{
		WorkspaceID: c.WorkspaceID, HelpCenterID: found.HelpCenterID,
	})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	tree := treeOf(all)
	if tree.within(*parent, c.ID) || tree.depth(*parent)+tree.height(c.ID) > MaxCollectionDepth {
		return ErrCollectionDepth
	}
	return nil
}

/*
collectionTree knows each collection's parent and children.
*/
type collectionTree struct {
	parents  map[ids.UUID]ids.UUID
	children map[ids.UUID][]ids.UUID
}

func treeOf(all []models.Collection) collectionTree {
	t := collectionTree{parents: map[ids.UUID]ids.UUID{}, children: map[ids.UUID][]ids.UUID{}}
	for _, c := range all {
		if c.ParentID != nil {
			t.parents[c.ID] = *c.ParentID
			t.children[*c.ParentID] = append(t.children[*c.ParentID], c.ID)
		}
	}
	return t
}

/*
within says whether node is ancestor or sits somewhere under it.
*/
func (t collectionTree) within(node, ancestor ids.UUID) bool {
	for seen := 0; seen <= len(t.parents); seen++ {
		if node == ancestor {
			return true
		}
		up, ok := t.parents[node]
		if !ok {
			return false
		}
		node = up
	}
	return false
}

/*
depth is the level a collection sits at, one for the top.
*/
func (t collectionTree) depth(id ids.UUID) int {
	level := 1
	for up, ok := t.parents[id]; ok && level <= len(t.parents); up, ok = t.parents[up] {
		level++
	}
	return level
}

/*
height is how many levels a collection spans with everything inside it,
one when it holds no sub-collections.
*/
func (t collectionTree) height(id ids.UUID) int {
	tallest := 0
	for _, child := range t.children[id] {
		tallest = max(tallest, t.height(child))
	}
	return tallest + 1
}
