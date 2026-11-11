/*
|--------------------------------------------------------------------------
| Choosing Content
|--------------------------------------------------------------------------
|
| A service with structure, such as a help center's collections or a
| workspace's pages, shows it as a tree, and the member ticks what to
| read. The ticked ids live in the source's "selected" setting: a
| document is read when it, or anything above it, is ticked, and every
| document is read when nothing is. Changing the ticks and syncing again
| pulls in what was added and removes what was left out.
|
*/

package sources

import (
	"context"
	"slices"
)

/*
Node is one item of a source's tree: a container such as a collection
or a page with pages under it, or a document. Children are the items
under it.
*/
type Node struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Children []Node `json:"children"`
}

/*
Browser is a driver that shows its source as a tree to choose from.
*/
type Browser interface {
	/*
		Tree returns the source's items as a tree, outermost first.
	*/
	Tree(ctx context.Context, cfg Config) ([]Node, error)
}

/*
Chosen reports whether a document is read: nothing is ticked, or the
document or one of the items above it is.
*/
func Chosen(selected []string, ids ...string) bool {
	if len(selected) == 0 {
		return true
	}

	for _, id := range ids {
		if slices.Contains(selected, id) {
			return true
		}
	}

	return false
}
