/*
|--------------------------------------------------------------------------
| Prompt Loader
|--------------------------------------------------------------------------
|
| The prompt tree is compiled in: resources/prompts/agent/<version>/,
| a common folder and one folder per model family. A file under the
| model's family wins over the same file in common, so every family
| gets its own tuning where the shared text does not fit, and the
| fallback keeps a missing family file from breaking anything.
|
*/

package prompts

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/enums"
	resprompts "github.com/dimmihq/dimmi/resources/prompts"
)

/*
File is one resolved prompt file.
*/
type File struct {
	Name    string
	Path    string
	Content string
	Hash    string
}

/*
Fingerprint names everything that shaped a run's prompts: the version
and a hash per file used.
*/
type Fingerprint struct {
	Version string            `json:"version"`
	Hashes  map[string]string `json:"hashes"`
}

/*
Loader resolves prompt files for a version and family.
*/
type Loader struct {
	mu       sync.Mutex
	version  string
	override fs.FS
	cache    map[string]File
}

/*
NewLoader builds the loader for a version, with an optional override
directory that shadows the embedded tree on self-host.
*/
func NewLoader(version string, override fs.FS) *Loader {
	if version == "" {
		version = "v1"
	}
	return &Loader{version: version, override: override, cache: map[string]File{}}
}

/*
Get resolves one file for the family: the family's folder first, the
common folder second.
*/
func (l *Loader) Get(family enums.AIModelFamily, name string) (*File, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cacheKey := string(family) + "/" + name
	if cached, ok := l.cache[cacheKey]; ok {
		return &cached, nil
	}

	familyPath := fmt.Sprintf("agent/%s/%s/%s", l.version, family, name)
	commonPath := fmt.Sprintf("agent/%s/common/%s", l.version, name)

	path, content, err := l.read(familyPath)
	if err != nil {
		path, content, err = l.read(commonPath)
		if err != nil {
			return nil, fmt.Errorf("prompt %s not found: %w", name, err)
		}
	}

	file := File{Name: name, Path: path, Content: content, Hash: hash(content)}
	l.cache[cacheKey] = file
	return &file, nil
}

/*
read loads a file, the override directory first.
*/
func (l *Loader) read(path string) (string, string, error) {
	if l.override != nil {
		if data, err := fs.ReadFile(l.override, path); err == nil {
			return path, string(data), nil
		}
	}
	data, err := fs.ReadFile(resprompts.Files, path)
	if err != nil {
		return path, "", err
	}
	return path, string(data), nil
}

/*
Fingerprint computes the hashes of the named files.
*/
func (l *Loader) Fingerprint(family enums.AIModelFamily, names []string) (*Fingerprint, error) {
	fp := &Fingerprint{Version: l.version, Hashes: map[string]string{}}
	for _, name := range names {
		file, err := l.Get(family, name)
		if err != nil {
			return nil, err
		}
		fp.Hashes[file.Path] = file.Hash
	}
	return fp, nil
}

/*
hash is the short content fingerprint recorded on runs.
*/
func hash(content string) string {
	sum := hashing.SHA256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

/*
Versions lists the prompt versions compiled in.
*/
func Versions() ([]string, error) {
	entries, err := fs.ReadDir(resprompts.Files, "agent")
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions, nil
}

/*
_ keeps strings imported for the substitution helpers that follow.
*/
var _ = strings.NewReplacer
