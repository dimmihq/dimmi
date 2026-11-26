package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileKeepsSecretsPrivate(t *testing.T) {
	f := &File{Path: filepath.Join(t.TempDir(), "secrets.json")}

	if _, err := f.Get("dimmi"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}

	if err := SetJSON(f, "dimmi", map[string]string{"access_token": "x"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(f.Path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v %v", info.Mode(), err)
	}

	var got map[string]string
	if err := GetJSON(f, "dimmi", &got); err != nil || got["access_token"] != "x" {
		t.Fatalf("%v %v", got, err)
	}

	if err := os.Chmod(f.Path, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Get("dimmi"); err != nil {
		t.Fatal(err)
	}

	if info, _ := os.Stat(f.Path); info.Mode().Perm() != 0o600 {
		t.Fatal("a file others could read was not made private again")
	}

	if err := f.Delete("dimmi"); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Get("dimmi"); !errors.Is(err, ErrNotFound) {
		t.Fatal("delete kept the secret")
	}
}
