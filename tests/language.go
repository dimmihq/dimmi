/*
|--------------------------------------------------------------------------
| Test Languages
|--------------------------------------------------------------------------
|
| Some tests need a second language to prove text follows the reader.
| They bring message files of their own, laid over the product's, so no
| real translation is ever needed by a test.
|
*/

package tests

import (
	"fmt"
	"io/fs"
	"testing/fstest"
)

/*
mergeLanguage copies the message files of both directories into one,
the extra files winning over the base ones at the same path.
*/
func mergeLanguage(base, extra fs.FS) (fs.FS, error) {
	merged := fstest.MapFS{}

	for _, files := range []fs.FS{base, extra} {
		err := fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}

			data, err := fs.ReadFile(files, path)
			merged[path] = &fstest.MapFile{Data: data}

			return err
		})
		if err != nil {
			return nil, fmt.Errorf("merge language files: %w", err)
		}
	}

	return merged, nil
}
