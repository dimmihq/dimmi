/*
|--------------------------------------------------------------------------
| Theme Folder
|--------------------------------------------------------------------------
|
| The theme being written, read from its folder: every file but hidden
| ones, checked and parsed the way dimmi will check it when it is
| published, over the default theme. The folder is watched, and a change
| to any file loads it again; a theme with problems keeps them for the
| preview to show, with the last theme that loaded.
|
*/

package themepreview

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
ReadFolder reads a theme folder's files by their path in the folder,
leaving out hidden files and folders.
*/
func ReadFolder(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	root := os.DirFS(dir)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(root, filepath.ToSlash(rel))
		files[filepath.ToSlash(rel)] = body
		return err
	})
	return files, err
}

/*
Zip packs a theme's files for uploading, in path order.
*/
func Zip(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(files[n]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

/*
Folder is a watched theme folder: the theme as it last loaded, the
problems of its latest state, and a version that moves on each change.
*/
type Folder struct {
	Dir      string
	Fallback *themes.Theme

	mu       sync.Mutex
	theme    *themes.Theme
	problems []themes.Problem
	version  int
	stamp    string
}

/*
Load reads the folder now, and reports whether anything changed.
*/
func (f *Folder) Load() (bool, error) {
	stamp, err := stampOf(f.Dir)
	if err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if stamp == f.stamp {
		return false, nil
	}
	files, err := ReadFolder(f.Dir)
	if err != nil {
		return false, err
	}
	theme, problems := themes.Load(files, f.Fallback)
	f.stamp, f.problems = stamp, problems
	if theme != nil {
		f.theme = theme
	}
	f.version++
	return true, nil
}

/*
Watch loads the folder again every interval until the context ends,
calling changed after each change.
*/
func (f *Folder) Watch(ctx context.Context, every time.Duration, changed func()) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if ok, err := f.Load(); err == nil && ok {
				changed()
			}
		}
	}
}

/*
State returns the theme as it last loaded, nil when it never did, the
problems of the folder as it is now, and the folder's version.
*/
func (f *Folder) State() (*themes.Theme, []themes.Problem, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.theme, f.problems, f.version
}

/*
stampOf sums the folder's file names, sizes and times, which change
whenever a file does.
*/
func stampOf(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s %d %d\n", p, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return b.String(), err
}
