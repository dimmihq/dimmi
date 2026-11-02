/*
|--------------------------------------------------------------------------
| Local Listing
|--------------------------------------------------------------------------
|
| A listing walks only the folders that can hold matching paths and
| yields files in path order, the order S3 uses. Sorting each folder
| with folder names followed by a slash makes a depth-first walk come
| out in that order without holding the whole listing in memory.
|
*/

package local

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"syscall"

	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
A listing is one walk of a store for the files under a prefix, yielding
to the caller of List.
*/
type listing struct {
	store  *store
	prefix string
	yield  func(filesystem.Entry, error) bool
}

/*
walk yields the files under the folder whose paths start with the
prefix, and reports whether the caller wants more.
*/
func (l *listing) walk(ctx context.Context, dir string) bool {
	entries, err := fs.ReadDir(l.store.root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return true
	}

	if err != nil {
		l.yield(filesystem.Entry{}, err)

		return false
	}

	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(sortKey(a), sortKey(b)) })

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			l.yield(filesystem.Entry{}, err)

			return false
		}

		name := entry.Name()
		if dir != "." {
			name = dir + "/" + name
		}

		if !l.visit(ctx, entry, name) {
			return false
		}
	}

	return true
}

/*
visit walks into a folder that can hold matching paths and yields a
matching regular file, skipping links, sidecars and temporary files.
*/
func (l *listing) visit(ctx context.Context, entry fs.DirEntry, name string) bool {
	if entry.IsDir() {
		if !strings.HasPrefix(name+"/", l.prefix) && !strings.HasPrefix(l.prefix, name+"/") {
			return true
		}

		return l.walk(ctx, name)
	}

	if !entry.Type().IsRegular() || strings.HasSuffix(name, attrsSuffix) || !strings.HasPrefix(name, l.prefix) {
		return true
	}

	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}

	if err != nil {
		l.yield(filesystem.Entry{}, err)

		return false
	}

	return l.yield(filesystem.Entry{Path: name, Size: info.Size(), LastModified: info.ModTime()}, nil)
}

func sortKey(entry fs.DirEntry) string {
	if entry.IsDir() {
		return entry.Name() + "/"
	}

	return entry.Name()
}
