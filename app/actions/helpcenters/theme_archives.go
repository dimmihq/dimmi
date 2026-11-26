/*
|--------------------------------------------------------------------------
| Theme Archives
|--------------------------------------------------------------------------
|
| A theme travels as a zip of its folder, up and down. Reading one never
| trusts its sizes: every file is read through a limit, so a zip that
| claims to be small and unpacks to gigabytes stops at the theme's own
| limits.
|
*/

package helpcenters

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
Unzip reads a theme's zip into its files by path. A zip of the theme's
folder itself, with theme.json one folder down, is read from inside that
folder; dotfiles and folders such as __MACOSX are skipped.
*/
func Unzip(archive []byte) (map[string][]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, ErrThemeArchive
	}
	if len(r.File) > themes.MaxFiles*2 {
		return nil, ErrThemeArchive.With("reason", fmt.Sprintf("a theme holds at most %d files", themes.MaxFiles))
	}
	files := map[string][]byte{}
	total := 0
	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, "./")
		if f.FileInfo().IsDir() || skipped(name) {
			continue
		}
		body, err := unzipped(f, name)
		if err != nil {
			return nil, err
		}
		if total += len(body); total > themes.MaxBytes {
			return nil, ErrThemeArchive.With("reason", "a theme holds at most 25 MB")
		}
		files[name] = body
	}
	return inside(files), nil
}

/*
unzipped reads one file of a theme's zip, refusing one larger than a
theme's file may be.
*/
func unzipped(f *zip.File, name string) ([]byte, error) {
	if f.UncompressedSize64 > themes.MaxFileBytes {
		return nil, ErrThemeArchive.With("reason", name+" is larger than 5 MB")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, ErrThemeArchive
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, themes.MaxFileBytes+1))
	if err != nil || len(body) > themes.MaxFileBytes {
		return nil, ErrThemeArchive.With("reason", name+" could not be read")
	}
	return body, nil
}

/*
inside reads a zip of the theme's folder from inside the folder, when
theme.json is not at the zip's root but one folder down.
*/
func inside(files map[string][]byte) map[string][]byte {
	if _, ok := files[themes.ManifestFile]; ok {
		return files
	}
	root := commonFolder(files)
	if root == "" {
		return files
	}
	out := make(map[string][]byte, len(files))
	for name, body := range files {
		out[strings.TrimPrefix(name, root)] = body
	}
	return out
}

func skipped(name string) bool {
	for part := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(part, ".") || part == "__MACOSX" {
			return true
		}
	}
	return false
}

/*
commonFolder is the one folder every file sits in, with its slash, when
theme.json is right inside it.
*/
func commonFolder(files map[string][]byte) string {
	root := ""
	for name := range files {
		first, _, found := strings.Cut(name, "/")
		if !found || (root != "" && root != first) {
			return ""
		}
		root = first
	}
	if _, ok := files[path.Join(root, themes.ManifestFile)]; !ok || root == "" {
		return ""
	}
	return root + "/"
}

/*
Zip writes a theme's files into a zip, in path order.
*/
func Zip(files map[string][]byte) ([]byte, error) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range sortedNames(files) {
		f, err := w.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func sortedNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
