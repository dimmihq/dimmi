/*
Package secrets keeps the dimmi CLI's keys: the dimmi sign-in and the
source tool's token. They go in the operating system's keychain (macOS
Keychain, Windows Credential Manager, the Secret Service on Linux), and
in a file only the user can read when there is no keychain, such as on
a server without a desktop session. They are never printed or logged.
*/
package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

/*
ErrNotFound is returned for a key that was never saved.
*/
var ErrNotFound = errors.New("not connected")

/*
Service is the keychain service name.
*/
const Service = "dimmi-cli"

/*
Store keeps named secrets.
*/
type Store interface {
	/* Get reads a secret, and returns ErrNotFound when it was never saved. */
	Get(name string) (string, error)

	/* Set saves a secret. */
	Set(name, value string) error

	/* Delete forgets a secret; a missing one is not an error. */
	Delete(name string) error

	/* Where names the place secrets are kept, for messages. */
	Where() string
}

/*
Open returns the keychain when it works, and the private file in dir
otherwise. DIMMI_SECRETS=file forces the file.
*/
func Open(dir string) Store {
	file := &File{Path: filepath.Join(dir, "secrets.json")}

	if os.Getenv("DIMMI_SECRETS") == "file" {
		return file
	}

	if err := keyring.Set(Service, "probe", "ok"); err != nil {
		return file
	}

	_ = keyring.Delete(Service, "probe")

	return &Keychain{Profile: filepath.Base(dir), fallback: file}
}

/*
Keychain is the operating system's keychain. Secrets saved to the file
before a keychain was there are still read.
*/
type Keychain struct {
	Profile  string
	fallback *File
}

/*
account is the keychain entry of a secret: the profile's own, so each
profile keeps its own sign-ins.
*/
func (k *Keychain) account(name string) string { return k.Profile + "/" + name }

/*
Get reads a secret from the keychain, then from the file.
*/
func (k *Keychain) Get(name string) (string, error) {
	v, err := keyring.Get(Service, k.account(name))
	if errors.Is(err, keyring.ErrNotFound) {
		return k.fallback.Get(name)
	}

	return v, err
}

/*
Set saves a secret in the keychain.
*/
func (k *Keychain) Set(name, value string) error { return keyring.Set(Service, k.account(name), value) }

/*
Delete forgets a secret in the keychain and in the file.
*/
func (k *Keychain) Delete(name string) error {
	err := keyring.Delete(Service, k.account(name))
	if errors.Is(err, keyring.ErrNotFound) {
		err = nil
	}

	if ferr := k.fallback.Delete(name); err == nil {
		err = ferr
	}

	return err
}

/*
Where names the keychain.
*/
func (k *Keychain) Where() string { return "the system keychain" }

/*
File is a JSON file of secrets, readable only by its owner.
*/
type File struct {
	Path string
	mu   sync.Mutex
}

func (f *File) read() (map[string]string, error) {
	raw, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}

	if err != nil {
		return nil, err
	}

	info, err := os.Stat(f.Path)
	if err == nil && info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(f.Path, 0o600); err != nil {
			return nil, fmt.Errorf("%s is readable by others and can't be fixed: %w", f.Path, err)
		}
	}

	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s is damaged: %w", f.Path, err)
	}

	return out, nil
}

func (f *File) write(m map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}

	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}

	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, f.Path)
}

/*
Get reads a secret from the file.
*/
func (f *File) Get(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.read()
	if err != nil {
		return "", err
	}

	v, ok := m[name]
	if !ok {
		return "", ErrNotFound
	}

	return v, nil
}

/*
Set saves a secret in the file.
*/
func (f *File) Set(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.read()
	if err != nil {
		return err
	}

	m[name] = value

	return f.write(m)
}

/*
Delete forgets a secret in the file.
*/
func (f *File) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.read()
	if err != nil {
		return err
	}

	if _, ok := m[name]; !ok {
		return nil
	}

	delete(m, name)

	return f.write(m)
}

/*
Where returns the file's path.
*/
func (f *File) Where() string { return f.Path }

/*
GetJSON reads a secret saved as JSON.
*/
func GetJSON(s Store, name string, out any) error {
	v, err := s.Get(name)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v), out)
}

/*
SetJSON saves a value as JSON.
*/
func SetJSON(s Store, name string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return s.Set(name, string(raw))
}
