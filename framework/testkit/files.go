/*
|--------------------------------------------------------------------------
| Test Files
|--------------------------------------------------------------------------
|
| Some services read a file another process writes, such as the
| address a dev server leaves behind. Tests write that file in a
| directory of their own, removed when the test ends, so no test sees
| another's file.
|
*/

package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

/*
TempFile writes the content to a file with the name in a fresh
directory removed after the test, and returns the file's path.
*/
func TempFile(tb testing.TB, name, content string) string {
	tb.Helper()

	path := filepath.Join(tb.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		tb.Fatalf("write %s: %v", path, err)
	}

	return path
}
