package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
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

	if err := os.Chmod(f.Path, 0o644); err != nil { //nolint:gosec // readable by others on purpose
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

func TestProfilesKeepTheirOwnKeychainEntries(t *testing.T) {
	keyring.MockInit()

	a, b := &Keychain{Profile: "default", fallback: &File{Path: t.TempDir() + "/a"}},
		&Keychain{Profile: "e2e", fallback: &File{Path: t.TempDir() + "/b"}}

	if err := a.Set("dimmi", "token-a"); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Get("dimmi"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("profile e2e read profile default's sign-in: %v", err)
	}

	if err := b.Delete("dimmi"); err != nil {
		t.Fatal(err)
	}

	if v, err := a.Get("dimmi"); err != nil || v != "token-a" {
		t.Fatalf("deleting in one profile touched another: %q %v", v, err)
	}
}
