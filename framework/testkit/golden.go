/*
|--------------------------------------------------------------------------
| Golden Files
|--------------------------------------------------------------------------
|
| Some outputs are contracts with programs outside this repository, such
| as an API document clients are built from. A test keeps the expected
| output in a file next to it and fails on any difference, so a change to
| the contract is always a deliberate change to that file. Running the
| tests with UPDATE_GOLDEN=1 writes the new output instead.
|
*/

package testkit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

/*
updateGolden is the environment variable that rewrites golden files.
*/
const updateGolden = "UPDATE_GOLDEN"

/*
Golden compares the output with the file at the path, relative to the
test's package, failing the test on any difference. With
UPDATE_GOLDEN=1 it writes the output to the file instead.
*/
func Golden(tb testing.TB, path string, got []byte) {
	tb.Helper()

	if os.Getenv(updateGolden) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			tb.Fatalf("create %s: %v", filepath.Dir(path), err)
		}

		if err := os.WriteFile(path, got, 0o600); err != nil {
			tb.Fatalf("write %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		tb.Fatalf("read golden file %s: %v (run with %s=1 to create it)", path, err, updateGolden)
	}

	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		tb.Errorf("%s differs from the output; if the change is meant, run with %s=1 and commit the file", path,
			updateGolden)
	}
}
